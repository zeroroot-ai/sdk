#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

/**
 * lint-pagination.mjs — SDK proto pagination lint
 *
 * Spec: cross-repo-cohesion-fixes Requirement 4.3, 4.4, D3.
 *
 * Walks every *.proto under api/proto/, finds every service method whose name
 * starts with "List", inspects the request message for fields named "limit"
 * or "offset", and fails (exit 1) when any such method is NOT in the
 * grandfathered allow-list below.
 *
 * The grandfather list is a literal const — adding to it requires editing this
 * file, i.e. it requires PR review of the addition.
 *
 * An entry that matches no such RPC also fails (ADR-0094). A stale entry is a
 * ready exemption for a new RPC that gets the same path and method name: three
 * entries for gibson/tenant/v1 outlived their protos that way (sdk#183).
 *
 * Usage:
 *   node scripts/lint-pagination.mjs
 *
 * LINT_PAGINATION_PROTO_ROOT points the walk at another proto tree. Only the
 * fixture in scripts/__tests__/lint-pagination.test.sh sets it.
 */

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, '..');
const PROTO_ROOT = process.env.LINT_PAGINATION_PROTO_ROOT || join(REPO_ROOT, 'api/proto');

// ---------------------------------------------------------------------------
// Grandfathered (proto-root-relative-path, method-name) pairs.
// These RPCs pre-date the AIP-158 pagination convention and keep offset/limit
// for wire compatibility. DO NOT add new entries — new List* RPCs MUST use
// page_size + page_token. Adding here requires a PR justification.
//
// Spec: cross-repo-cohesion-fixes Requirement 4.2; design.md "Out of scope".
// ---------------------------------------------------------------------------
const GRANDFATHER_LIST = [
  // PluginAdminService was decomposed out of the former gibson.admin.v1 surface
  // (#267). Under ADR-0058 (E6, narrow-the-SDK) it was re-homed
  // into its own wire package gibson.pluginadmin.v1 so it can stay in the OSS
  // SDK while the nine tenant-admin services move to gibson. The wire-compat
  // rationale for the limit/offset shape is unchanged — path corrected.
  // (secrets.proto/ListSecrets and grants.proto/ListActiveGrants left the SDK
  // in E6 — those entries are dropped with their protos.)
  { file: 'gibson/pluginadmin/v1/plugin_admin.proto', method: 'ListPluginInstalls' },
  { file: 'gibson/daemon/v1/daemon.proto',       method: 'ListMissions' },
  { file: 'gibson/daemon/v1/daemon.proto',       method: 'ListMissionDefinitions' },
  // ListSecrets RETURNS. The note above records that this entry was dropped
  // when secrets.proto left the SDK in E6; ADR-0096 brought the proto back as
  // its own wire package gibson.secrets.v1, so the entry comes back with it and
  // its path is corrected the same way PluginAdminService's was. This is not a
  // new List* RPC and the guard's rule is unchanged.
  //
  // Wire-locked, but not by `make proto-breaking`: the proto is new in this
  // module, so there is no previous version here for that gate to compare
  // against. It is locked by live callers — `ListSecretsRequest` is
  // {category_filter, limit, offset, name_prefix} and the dashboard's secrets
  // UI and gibson's handler both speak it today. Renaming `limit` to
  // `page_size` and replacing `offset` with `page_token` would turn the
  // dashboard's package switch from a call-site rename into a rewrite of its
  // pagination, which ADR-0096 explicitly scoped out.
  //
  // Converting it to AIP-158 is a worthwhile separate change: one deliberate
  // migration across sdk, gibson and dashboard together. Justification: ADR-0096.
  { file: 'gibson/secrets/v1/secrets.proto', method: 'ListSecrets' },
];

// used[i] is set when entry i exempted an RPC in this run.
const used = GRANDFATHER_LIST.map(() => false);

function isGrandfathered(relPath, methodName) {
  const i = GRANDFATHER_LIST.findIndex(
    (e) => relPath === e.file && methodName === e.method,
  );
  if (i < 0) return false;
  used[i] = true;
  return true;
}

// ---------------------------------------------------------------------------
// Proto text parsing helpers
// ---------------------------------------------------------------------------

/**
 * Walk a directory tree and collect all *.proto file paths.
 */
function findProtos(dir) {
  const results = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    const st = statSync(full);
    if (st.isDirectory()) {
      results.push(...findProtos(full));
    } else if (entry.endsWith('.proto')) {
      results.push(full);
    }
  }
  return results;
}

/**
 * Parse all service definitions from a proto file's text.
 * Returns an array of { serviceName, methods: [{ name, requestMessage }] }.
 */
function parseServices(text) {
  const services = [];
  // Strip block comments to avoid false positives from commented-out code.
  const stripped = text.replace(/\/\*[\s\S]*?\*\//g, '');

  const serviceRe = /service\s+(\w+)\s*\{/g;
  let svcMatch;

  while ((svcMatch = serviceRe.exec(stripped)) !== null) {
    const svcName = svcMatch[1];
    // Find the matching closing brace for this service block.
    const blockStart = svcMatch.index + svcMatch[0].length;
    let depth = 1;
    let i = blockStart;
    while (i < stripped.length && depth > 0) {
      if (stripped[i] === '{') depth++;
      else if (stripped[i] === '}') depth--;
      i++;
    }
    const block = stripped.slice(blockStart, i - 1);

    // Find rpc declarations within the service block.
    const rpcRe = /rpc\s+(\w+)\s*\(\s*(\w+)\s*\)/g;
    let rpcMatch;
    const methods = [];
    while ((rpcMatch = rpcRe.exec(block)) !== null) {
      methods.push({ name: rpcMatch[1], requestMessage: rpcMatch[2] });
    }
    services.push({ serviceName: svcName, methods });
  }
  return services;
}

/**
 * Check whether a named message in the proto text contains a field
 * named "limit" or "offset".
 */
function messageHasLimitOrOffset(text, messageName) {
  // Find the message block.
  const msgRe = new RegExp(`\\bmessage\\s+${messageName}\\s*\\{`);
  const m = msgRe.exec(text);
  if (!m) return false;

  const blockStart = m.index + m[0].length;
  let depth = 1;
  let i = blockStart;
  while (i < text.length && depth > 0) {
    if (text[i] === '{') depth++;
    else if (text[i] === '}') depth--;
    i++;
  }
  const block = text.slice(blockStart, i - 1);
  // Match field declarations: "int32 limit = N;" or "int64 offset = N;"
  return /\b(limit|offset)\s*=\s*\d+\s*;/.test(block);
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

const protoFiles = findProtos(PROTO_ROOT);
let violations = 0;

// A floor. A walk that finds no proto file checked nothing and must not pass.
if (protoFiles.length === 0) {
  console.error(`[lint-pagination] FAIL: no *.proto file under ${PROTO_ROOT}, so this run checked nothing.`);
  process.exit(1);
}

for (const absPath of protoFiles) {
  const relPath = relative(PROTO_ROOT, absPath);
  const text = readFileSync(absPath, 'utf8');
  const services = parseServices(text);

  for (const svc of services) {
    for (const method of svc.methods) {
      if (!method.name.startsWith('List')) continue;
      if (!messageHasLimitOrOffset(text, method.requestMessage)) continue;
      if (isGrandfathered(relPath, method.name)) continue;

      console.error(
        `[lint-pagination] ${relPath}: ${svc.serviceName}/${method.name} ` +
        `uses limit/offset pagination. New List* RPCs must use page_size + page_token (AIP-158). ` +
        `If this is intentional, add it to GRANDFATHER_LIST in scripts/lint-pagination.mjs ` +
        `with a PR justification.`
      );
      violations++;
    }
  }
}

// An entry that exempted nothing: the RPC is gone, or it no longer uses
// limit/offset. Either way the entry records a decision about nothing.
GRANDFATHER_LIST.forEach((e, i) => {
  if (used[i]) return;
  console.error(
    `[lint-pagination] ${e.file}: GRANDFATHER_LIST entry ${e.method} matches no List* RPC ` +
    `with limit/offset pagination. Delete the entry from scripts/lint-pagination.mjs.`
  );
  violations++;
});

if (violations > 0) {
  console.error(`[lint-pagination] FAIL: ${violations} violation(s). See above.`);
  process.exit(1);
}

console.log('[lint-pagination] SDK protos: all List* methods use AIP-158 pagination or are grandfathered.');
process.exit(0);
