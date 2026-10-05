#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

/**
 * lint-rpc-surface.mjs: the services and the RPCs of the sdk are one
 * checked-in list (ADR-0058, sdk#189).
 *
 * The sdk is the surface that outside developers pin, so each service and
 * each RPC is a long promise. This lint reads each *.proto file under
 * api/proto/gibson and compares the RPCs with api/proto/rpc-surface.txt. It
 * fails when:
 *
 *   - a proto declares an RPC that the list does not hold, or
 *   - the list holds an RPC that no proto declares, or
 *   - the list has the same line two times, or
 *   - the walk finds no proto file or no RPC, because that run checked nothing.
 *
 * A line of the list is `<package>.<Service>/<Method>`. A line that starts
 * with `#` is a comment.
 *
 * To add a service or an RPC, add its line to the list in the same pull
 * request. The reviewer then sees each change of the public surface in one
 * file.
 *
 * LINT_RPC_SURFACE_PROTO_ROOT and LINT_RPC_SURFACE_LIST point the lint at a
 * different tree and list. Only the fixture in
 * scripts/__tests__/lint-rpc-surface.test.sh sets them.
 */

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, '..');
const PROTO_ROOT = process.env.LINT_RPC_SURFACE_PROTO_ROOT || join(REPO_ROOT, 'api/proto/gibson');
const LIST_PATH = process.env.LINT_RPC_SURFACE_LIST || join(REPO_ROOT, 'api/proto/rpc-surface.txt');

function findProtos(dir) {
  const results = [];
  let entries;
  try {
    entries = readdirSync(dir);
  } catch {
    return results;
  }
  for (const entry of entries) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) results.push(...findProtos(full));
    else if (entry.endsWith('.proto')) results.push(full);
  }
  return results;
}

// stripComments removes block and line comments, so an RPC that a comment
// names is not an RPC.
function stripComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\n]*/g, '');
}

// declaredRPCs returns each `<package>.<Service>/<Method>` of one proto file.
function declaredRPCs(text) {
  const out = [];
  const pkg = /^\s*package\s+([\w.]+)\s*;/m.exec(text);
  const prefix = pkg ? `${pkg[1]}.` : '';
  const serviceRe = /\bservice\s+(\w+)\s*\{/g;
  let svc;
  while ((svc = serviceRe.exec(text)) !== null) {
    let depth = 1;
    let i = svc.index + svc[0].length;
    const start = i;
    while (i < text.length && depth > 0) {
      if (text[i] === '{') depth++;
      else if (text[i] === '}') depth--;
      i++;
    }
    const block = text.slice(start, i - 1);
    for (const rpc of block.matchAll(/\brpc\s+(\w+)\s*\(/g)) {
      out.push(`${prefix}${svc[1]}/${rpc[1]}`);
    }
  }
  return out;
}

const protoFiles = findProtos(PROTO_ROOT);
if (protoFiles.length === 0) {
  console.error(`[lint-rpc-surface] FAIL: no *.proto file under ${PROTO_ROOT}, so this run checked nothing.`);
  process.exit(1);
}

const declared = new Map(); // rpc -> proto path
for (const absPath of protoFiles) {
  const text = stripComments(readFileSync(absPath, 'utf8'));
  for (const rpc of declaredRPCs(text)) declared.set(rpc, relative(PROTO_ROOT, absPath));
}
if (declared.size === 0) {
  console.error(`[lint-rpc-surface] FAIL: no RPC under ${PROTO_ROOT}, so this run checked nothing.`);
  process.exit(1);
}

let listText;
try {
  listText = readFileSync(LIST_PATH, 'utf8');
} catch {
  console.error(`[lint-rpc-surface] FAIL: cannot read the list ${LIST_PATH}.`);
  process.exit(1);
}

let violations = 0;
const listed = new Set();
for (const raw of listText.split('\n')) {
  const line = raw.trim();
  if (line === '' || line.startsWith('#')) continue;
  if (listed.has(line)) {
    console.error(`[lint-rpc-surface] the list holds ${line} two times. Remove one line.`);
    violations++;
  }
  listed.add(line);
}

for (const [rpc, file] of [...declared].sort()) {
  if (!listed.has(rpc)) {
    console.error(
      `[lint-rpc-surface] ${file}: ${rpc} is not in api/proto/rpc-surface.txt. ` +
      `The sdk holds only what a component developer needs (ADR-0058). ` +
      `If the RPC belongs in the sdk, add its line to the list in this pull request.`,
    );
    violations++;
  }
}

for (const rpc of [...listed].sort()) {
  if (!declared.has(rpc)) {
    console.error(
      `[lint-rpc-surface] the list holds ${rpc}, and no proto declares it. Remove the line.`,
    );
    violations++;
  }
}

if (violations > 0) {
  console.error(`[lint-rpc-surface] FAIL: ${violations} difference(s) between the protos and the list.`);
  process.exit(1);
}

const services = new Set([...declared.keys()].map((rpc) => rpc.split('/')[0]));
console.log(`[lint-rpc-surface] SDK protos: ${services.size} services and ${declared.size} RPCs match the list.`);
