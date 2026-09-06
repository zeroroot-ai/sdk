#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

/**
 * diff-coverage.mjs — 85% diff-coverage gate on changed lines.
 *
 * Quality bar: docs/architecture/open-core/RESTRUCTURE-QUALITY-BARS.md §4 —
 * "85% diff-coverage on changed lines, blocking". This is the teeth of the
 * coverage gate: regardless of the absolute per-package floor (the baselined
 * backlog), any Go code a PR ADDS OR CHANGES must be ≥85% covered. New debt
 * cannot land. This mirrors the lint gate's `--new-from-merge-base` design
 * (PR #386): baseline the past, hold the line on new work.
 *
 * Mechanism:
 *   1. `git diff --unified=0 <base>...HEAD -- '*.go'` gives the added/changed
 *      lines per .go file (excluding deletions; we only score added lines).
 *   2. The Go coverage profile gives, per statement block, the line range and
 *      whether it was hit.
 *   3. We intersect: for every coverable statement line that the diff added,
 *      is it covered? covered/total over all changed coverable lines must be
 *      ≥ THRESHOLD.
 *
 * Generated, vendored, and non-test-relevant files are excluded (api/gen,
 * *.pb.go, *_test.go themselves, mocks) — same spirit as the lint excludes.
 *
 * Usage:
 *   node scripts/diff-coverage.mjs <coverage.out> [--base <ref>]
 *
 * --base defaults to origin/main; CI passes the PR target branch. The triple-dot
 * (<base>...HEAD) scopes the diff to changes introduced on this branch since the
 * merge-base, so unrelated churn on main is never counted.
 *
 * Exit codes: 0 pass (incl. "no changed coverable lines"), 1 below threshold,
 * 2 usage/IO error.
 */

import { readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";

const THRESHOLD = 85.0;

// Files whose changed lines are NOT subject to diff-coverage. Generated code,
// proto bindings, and test files are not meaningfully "covered by tests".
const EXCLUDE = [
  /(^|\/)api\/gen\//,
  /\.pb\.go$/,
  /_test\.go$/,
  /(^|\/)mock_[^/]*\.go$/,
  /(^|\/)zz_generated/,
];

function parseArgs(argv) {
  const args = argv.slice(2);
  let base = "origin/main";
  const baseIdx = args.indexOf("--base");
  if (baseIdx !== -1) base = args[baseIdx + 1];
  const profile = args.find(
    (a, i) => !a.startsWith("--") && args[i - 1] !== "--base",
  );
  if (!profile) {
    process.stderr.write(
      "usage: diff-coverage.mjs <coverage.out> [--base <ref>]\n",
    );
    process.exit(2);
  }
  return { profile, base };
}

function isExcluded(file) {
  return EXCLUDE.some((re) => re.test(file));
}

/**
 * Added line numbers per file from `git diff --unified=0 base...HEAD`.
 * Returns Map<file, Set<lineNo>> using the NEW-file line numbering (the @@ +A,B
 * hunk header), which is what the coverage profile reports against.
 */
function addedLines(base) {
  let diff;
  try {
    diff = execFileSync(
      "git",
      ["diff", "--unified=0", "--no-color", `${base}...HEAD`, "--", "*.go"],
      { encoding: "utf8", maxBuffer: 64 * 1024 * 1024 },
    );
  } catch (e) {
    process.stderr.write(`diff-coverage: git diff failed: ${e.message}\n`);
    process.exit(2);
  }

  const byFile = new Map();
  let curFile = null;
  let newLine = 0;
  for (const line of diff.split("\n")) {
    if (line.startsWith("+++ ")) {
      // "+++ b/path" or "+++ /dev/null" (deletion).
      const p = line.slice(4);
      curFile = p === "/dev/null" ? null : p.replace(/^b\//, "");
      if (curFile && !isExcluded(curFile) && !byFile.has(curFile)) {
        byFile.set(curFile, new Set());
      }
      continue;
    }
    const hunk = line.match(/^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@/);
    if (hunk) {
      newLine = Number(hunk[1]);
      continue;
    }
    if (!curFile || isExcluded(curFile)) continue;
    if (line.startsWith("+") && !line.startsWith("+++")) {
      byFile.get(curFile).add(newLine);
      newLine++;
    } else if (line.startsWith("-") || line.startsWith("---")) {
      // deletion: no advance in new-file numbering
    } else if (line.startsWith(" ")) {
      newLine++;
    }
  }
  return byFile;
}

/**
 * Coverage map keyed by file path (module-relative): Map<file, Map<lineNo, hit>>
 * where a line is "covered" if it belongs to at least one executed statement
 * block. The profile path is the import path; we strip the module prefix to a
 * repo-relative path so it lines up with git diff paths.
 */
function coverageByLine(profileText, moduleRoot) {
  const byFile = new Map();
  for (const line of profileText.split("\n")) {
    if (!line || line.startsWith("mode:")) continue;
    const m = line.match(/^(.+):(\d+)\.\d+,(\d+)\.\d+ (\d+) (\d+)$/);
    if (!m) continue;
    let [, filePath, sL, eL, , count] = m;
    const start = Number(sL);
    const end = Number(eL);
    const hit = Number(count) > 0;
    // Strip module prefix → repo-relative path.
    let rel = filePath;
    if (rel.startsWith(moduleRoot + "/")) rel = rel.slice(moduleRoot.length + 1);
    let lines = byFile.get(rel);
    if (!lines) {
      lines = new Map();
      byFile.set(rel, lines);
    }
    for (let ln = start; ln <= end; ln++) {
      // A line is covered if ANY block touching it was executed.
      lines.set(ln, (lines.get(ln) || false) || hit);
    }
  }
  return byFile;
}

function moduleRoot() {
  try {
    const mod = readFileSync("go.mod", "utf8");
    const m = mod.match(/^module\s+(\S+)/m);
    return m ? m[1] : "";
  } catch {
    return "";
  }
}

function main() {
  const { profile, base } = parseArgs(process.argv);

  let profileText;
  try {
    profileText = readFileSync(profile, "utf8");
  } catch (e) {
    process.stderr.write(`diff-coverage: cannot read ${profile}: ${e.message}\n`);
    process.exit(2);
  }

  const added = addedLines(base);
  const cover = coverageByLine(profileText, moduleRoot());

  let total = 0;
  let covered = 0;
  const uncoveredByFile = new Map();

  for (const [file, lines] of added) {
    const fileCov = cover.get(file);
    if (!fileCov) continue; // no coverable statements in this file (e.g. interface decls, consts)
    for (const ln of lines) {
      if (!fileCov.has(ln)) continue; // added line is not a coverable statement (comment, brace, decl)
      total++;
      if (fileCov.get(ln)) {
        covered++;
      } else {
        if (!uncoveredByFile.has(file)) uncoveredByFile.set(file, []);
        uncoveredByFile.get(file).push(ln);
      }
    }
  }

  if (total === 0) {
    process.stdout.write(
      `diff-coverage: PASS — no changed coverable Go statements since ${base}.\n`,
    );
    process.exit(0);
  }

  const pct = (covered / total) * 100;
  const pctR = Math.round(pct * 10) / 10;

  if (pct + 1e-9 < THRESHOLD) {
    process.stderr.write(
      `FAIL: diff coverage ${pctR}% (${covered}/${total} changed statements) ` +
        `is below the ${THRESHOLD}% bar.\n` +
        `Uncovered changed statements:\n`,
    );
    for (const [file, lns] of [...uncoveredByFile].sort()) {
      const ranges = compressRanges(lns.sort((a, b) => a - b));
      process.stderr.write(`  ${file}: ${ranges}\n`);
    }
    process.exit(1);
  }

  process.stdout.write(
    `diff-coverage: PASS — ${pctR}% (${covered}/${total} changed statements) ` +
      `≥ ${THRESHOLD}% since ${base}.\n`,
  );
}

function compressRanges(sorted) {
  const out = [];
  let s = null;
  let p = null;
  for (const n of sorted) {
    if (s === null) {
      s = p = n;
    } else if (n === p + 1) {
      p = n;
    } else {
      out.push(s === p ? `${s}` : `${s}-${p}`);
      s = p = n;
    }
  }
  if (s !== null) out.push(s === p ? `${s}` : `${s}-${p}`);
  return out.join(", ");
}

main();
