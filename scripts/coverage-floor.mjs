#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

/**
 * coverage-floor.mjs — uniform per-package coverage floor gate.
 *
 * Quality bar: docs/architecture/open-core/RESTRUCTURE-QUALITY-BARS.md §4 —
 * "uniform 80% absolute floor every repo". This is the package-level half of
 * that gate (the diff-coverage half is scripts/diff-coverage.mjs).
 *
 * The floor is enforced the same way the lint gate (sdk#385 / PR #386) is:
 * BLOCKING, but the pre-existing under-floor backlog is BASELINED rather than
 * fixed in one PR. Concretely:
 *
 *   1. Every package at or above FLOOR (80%) must STAY at or above FLOOR.
 *   2. A package listed in the baseline (scripts/coverage-baseline.json) — i.e.
 *      one that was already below the floor when the gate landed — must not
 *      REGRESS below its recorded baseline coverage (minus a small epsilon for
 *      measurement jitter). It is held at its current level and only ratchets
 *      up; the backlog is burned down in sdk#388.
 *   3. A package NOT in the baseline that comes in below FLOOR fails. New code
 *      cannot introduce a sub-floor package, and a baselined package that
 *      climbs to/above the floor is removed from the baseline (drift-checked).
 *
 * So: no new sub-floor packages, no regression of baselined ones, and the
 * floor ratchets monotonically toward a clean 80%-everywhere state.
 *
 * Coverage is computed from a Go coverage profile (`go test -coverprofile`),
 * statement-weighted per package, identically to `go tool cover -func` totals.
 *
 * Usage:
 *   node scripts/coverage-floor.mjs <coverage.out>            # gate (CI + `make`)
 *   node scripts/coverage-floor.mjs <coverage.out> --baseline # rewrite baseline
 *
 * Exit codes: 0 pass, 1 a floor/regression violation, 2 usage/IO error.
 */

import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const FLOOR = 80.0;

// Packages excluded from the floor (and the backlog). Generated proto bindings
// (api/gen) are tracked source, not hand-written, and are exempt per
// QUALITY-BARS §3 ("Public API exempt"). cueschemas is an embed-only package
// (//go:embed, no logic). These never carry meaningful unit coverage and would
// otherwise pollute the backlog. Matched against the import path.
const EXCLUDE_PKG = [
  /\/api\/gen\//,
  /\/api\/gen$/,
  /\/cueschemas$/,
];

function isExcludedPkg(pkg) {
  return EXCLUDE_PKG.some((re) => re.test(pkg));
}
// Allowed downward jitter (percentage points) for a baselined package, to
// absorb non-determinism in coverage measurement (e.g. map iteration order in
// tests). A regression beyond this is a real drop and fails.
const EPSILON = 0.5;

const __dirname = dirname(fileURLToPath(import.meta.url));
const BASELINE_PATH = join(__dirname, "coverage-baseline.json");

function parseArgs(argv) {
  const args = argv.slice(2);
  const writeBaseline = args.includes("--baseline");
  const profile = args.find((a) => !a.startsWith("--"));
  if (!profile) {
    process.stderr.write(
      "usage: coverage-floor.mjs <coverage.out> [--baseline]\n",
    );
    process.exit(2);
  }
  return { profile, writeBaseline };
}

/**
 * Aggregate a Go coverage profile into per-package { covered, total } statement
 * counts. Profile lines are: `path/file.go:sL.sC,eL.eC numStmts hitCount`.
 * The package is the directory of the file path (the import path prefix).
 */
function coverageByPackage(profileText) {
  const pkgs = new Map(); // pkg -> { covered, total }
  for (const line of profileText.split("\n")) {
    if (!line || line.startsWith("mode:")) continue;
    // Split off the trailing "<numStmts> <count>".
    const m = line.match(/^(.+):\d+\.\d+,\d+\.\d+ (\d+) (\d+)$/);
    if (!m) continue;
    const [, filePath, numStmtsRaw, countRaw] = m;
    const numStmts = Number(numStmtsRaw);
    const count = Number(countRaw);
    const pkg = filePath.slice(0, filePath.lastIndexOf("/"));
    const acc = pkgs.get(pkg) || { covered: 0, total: 0 };
    acc.total += numStmts;
    if (count > 0) acc.covered += numStmts;
    pkgs.set(pkg, acc);
  }
  const out = new Map();
  for (const [pkg, { covered, total }] of pkgs) {
    if (isExcludedPkg(pkg)) continue;
    out.set(pkg, total === 0 ? 100 : (covered / total) * 100);
  }
  return out;
}

function round1(n) {
  return Math.round(n * 10) / 10;
}

function main() {
  const { profile, writeBaseline } = parseArgs(process.argv);

  let profileText;
  try {
    profileText = readFileSync(profile, "utf8");
  } catch (e) {
    process.stderr.write(`coverage-floor: cannot read ${profile}: ${e.message}\n`);
    process.exit(2);
  }

  const coverage = coverageByPackage(profileText);

  if (writeBaseline) {
    // Record every package CURRENTLY below the floor. These are the backlog
    // (sdk#388). Packages with no statements in the profile (e.g. pure proto
    // bindings, no test files) report 100% and never appear here.
    const baseline = {};
    for (const [pkg, cov] of [...coverage].sort()) {
      if (cov < FLOOR) baseline[pkg] = round1(cov);
    }
    const json =
      JSON.stringify(
        {
          _comment:
            "Packages below the 80% coverage floor when the gate landed (sdk#356). " +
            "Burndown: sdk#388. Each entry is held at its recorded level (no regression) " +
            "and removed once it reaches 80%. Regenerate with `make coverage-baseline`.",
          floor: FLOOR,
          packages: baseline,
        },
        null,
        2,
      ) + "\n";
    writeFileSync(BASELINE_PATH, json);
    process.stderr.write(
      `coverage-floor: wrote baseline with ${Object.keys(baseline).length} sub-floor packages to ${BASELINE_PATH}\n`,
    );
    process.exit(0);
  }

  let baseline;
  try {
    baseline = JSON.parse(readFileSync(BASELINE_PATH, "utf8")).packages || {};
  } catch (e) {
    process.stderr.write(
      `coverage-floor: cannot read baseline ${BASELINE_PATH}: ${e.message}\n`,
    );
    process.exit(2);
  }

  const violations = [];
  const ratcheted = []; // baselined packages that have reached the floor

  for (const [pkg, cov] of coverage) {
    const covR = round1(cov);
    if (cov >= FLOOR) {
      // At/above floor. If it was baselined, it has graduated — the baseline is
      // now stale and must be trimmed (keeps the backlog honest, like the lint
      // baseline). Drift-checked below.
      if (pkg in baseline) ratcheted.push({ pkg, cov: covR });
      continue;
    }
    // Below floor.
    if (pkg in baseline) {
      const floorForPkg = baseline[pkg] - EPSILON;
      if (cov < floorForPkg) {
        violations.push(
          `  ${pkg}: ${covR}% regressed below its baseline of ${baseline[pkg]}% ` +
            `(allowed ${round1(floorForPkg)}%)`,
        );
      }
    } else {
      violations.push(
        `  ${pkg}: ${covR}% is below the ${FLOOR}% floor and is not baselined ` +
          `(new sub-floor package — add tests or, if unavoidable, baseline it)`,
      );
    }
  }

  const shortPkg = (p) => p.replace(/^github\.com\/zeroroot-ai\/sdk\/?/, "./") || ".";

  if (ratcheted.length > 0) {
    process.stderr.write(
      "FAIL: these packages reached the 80% floor and must be removed from " +
        "scripts/coverage-baseline.json (run `make coverage-baseline` and commit):\n",
    );
    for (const { pkg, cov } of ratcheted) {
      process.stderr.write(`  ${shortPkg(pkg)}: now ${cov}%\n`);
    }
  }

  if (violations.length > 0) {
    process.stderr.write("FAIL: coverage floor violations:\n");
    for (const v of violations) {
      process.stderr.write(`${v.replace(/github\.com\/zeroroot-ai\/sdk\//g, "")}\n`);
    }
  }

  if (ratcheted.length > 0 || violations.length > 0) {
    process.exit(1);
  }

  const subFloor = Object.keys(baseline).length;
  process.stdout.write(
    `coverage-floor: PASS — ${coverage.size} packages, ` +
      `${coverage.size - subFloor} at/above ${FLOOR}% floor, ` +
      `${subFloor} baselined (sdk#388 burndown).\n`,
  );
}

main();
