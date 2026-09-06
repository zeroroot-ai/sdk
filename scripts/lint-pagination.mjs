#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

/**
 * lint-pagination.mjs — SDK proto pagination lint
 *
 * Spec: cross-repo-cohesion-fixes Requirement 4.3, 4.4, D3.
 *
 * Walks every *.proto under core/sdk/api/proto/, finds every service method
 * whose name starts with "List", inspects the request message for fields named
 * "limit" or "offset", and fails (exit 1) when any such method is NOT in the
 * grandfathered allow-list below.
 *
 * The grandfather list is a literal const — adding to it requires editing this
 * file, i.e. it requires PR review of the addition.
 *
 * NOTE: This script is intentionally duplicated in core/gibson/scripts/ with
 * a different proto root (D3 — do not abstract into a shared package).
 *
 * Usage:
 *   node core/sdk/scripts/lint-pagination.mjs
 *   node scripts/lint-pagination.mjs   (from core/sdk/ directory)
 */

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, '..');
const PROTO_ROOT = join(REPO_ROOT, 'api/proto');

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
  // by ADR-0039/0040 (#267). Under ADR-0058 (E6, narrow-the-SDK) it was re-homed
  // into its own wire package gibson.pluginadmin.v1 so it can stay in the OSS
  // SDK while the nine tenant-admin services move to gibson. The wire-compat
  // rationale for the limit/offset shape is unchanged — path corrected.
  // (secrets.proto/ListSecrets and grants.proto/ListActiveGrants left the SDK
  // in E6 — those entries are dropped with their protos.)
  { file: 'gibson/pluginadmin/v1/plugin_admin.proto', method: 'ListPluginInstalls' },
  { file: 'gibson/daemon/v1/daemon.proto',       method: 'ListMissions' },
  { file: 'gibson/daemon/v1/daemon.proto',       method: 'ListMissionDefinitions' },
  // ListAuditEvents is already cursor-paginated (limit + cursor → next_cursor),
  // not the offset anti-pattern this guard targets. Its page-size field is
  // named `limit` rather than the AIP-158 `page_size`; renaming it now is a
  // WIRE_JSON-breaking change (FIELD_SAME_NAME) that `make proto-breaking`
  // rejects, so it is wire-compat-locked to `limit` — the operative reason this
  // allow-list exists. Justification: sdk#250.
  { file: 'gibson/tenant/v1/tenant.proto',  method: 'ListAuditEvents' },
  // ListAlerts / ListConversations shipped in v0.137.0 as part of the ADR-0039
  // UserService decomposition (#267) and are wire-locked (dashboard chat/alerts
  // + gibson daemon handlers). Their page-size field is `int32 limit`; renaming
  // it to AIP-158 `page_size` is a WIRE_JSON-breaking FIELD rename that
  // `make proto-breaking` rejects, so they are wire-compat-locked to `limit`
  // — same rationale as ListAuditEvents above. Justification: sdk#250.
  { file: 'gibson/tenant/v1/user.proto',    method: 'ListAlerts' },
  { file: 'gibson/tenant/v1/user.proto',    method: 'ListConversations' },
];

function isGrandfathered(relPath, methodName) {
  return GRANDFATHER_LIST.some(
    (e) => relPath === e.file && methodName === e.method,
  );
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
        `If this is intentional, add it to GRANDFATHER_LIST in core/sdk/scripts/lint-pagination.mjs ` +
        `with a PR justification.`
      );
      violations++;
    }
  }
}

if (violations > 0) {
  console.error(`[lint-pagination] FAIL: ${violations} violation(s). See above.`);
  process.exit(1);
}

console.log('[lint-pagination] SDK protos: all List* methods use AIP-158 pagination or are grandfathered.');
process.exit(0);
