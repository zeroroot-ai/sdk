#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

/**
 * lint-pagination.mjs: each list request uses one page shape, page_size and
 * page_token (ADR-0028, rule 3; sdk#186).
 *
 * The lint reads each *.proto file under api/proto. It fails when:
 *
 *   - any message declares a field named `offset`, or
 *   - the request of an RPC whose name starts with List declares a field
 *     named `limit` or `offset`, or
 *   - the walk finds no proto file, because that run checked nothing.
 *
 * No allowlist exists. The last limit and offset fields left in sdk#186, and
 * their numbers are reserved.
 *
 * LINT_PAGINATION_PROTO_ROOT points the walk at a different proto tree. Only
 * the fixture in scripts/__tests__/lint-pagination.test.sh sets it.
 */

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, '..');
const PROTO_ROOT = process.env.LINT_PAGINATION_PROTO_ROOT || join(REPO_ROOT, 'api/proto');

function findProtos(dir) {
  const results = [];
  let entries;
  try {
    entries = readdirSync(dir);
  } catch {
    return results;
  }
  for (const entry of entries) {
    // google/ holds vendored well-known types, not sdk messages.
    if (entry === 'google') continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) results.push(...findProtos(full));
    else if (entry.endsWith('.proto')) results.push(full);
  }
  return results;
}

function stripComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\n]*/g, '');
}

function messageBlock(text, name) {
  const m = new RegExp(`\\bmessage\\s+${name}\\s*\\{`).exec(text);
  if (!m) return null;
  let depth = 1;
  let i = m.index + m[0].length;
  const start = i;
  while (i < text.length && depth > 0) {
    if (text[i] === '{') depth++;
    else if (text[i] === '}') depth--;
    i++;
  }
  return text.slice(start, i - 1);
}

const FIELD = (name) => new RegExp(`^\\s*(?:optional\\s+)?[\\w.]+\\s+${name}\\s*=\\s*\\d+`, 'm');

const protoFiles = findProtos(PROTO_ROOT);
if (protoFiles.length === 0) {
  console.error(`[lint-pagination] FAIL: no *.proto file under ${PROTO_ROOT}, so this run checked nothing.`);
  process.exit(1);
}

let violations = 0;
for (const absPath of protoFiles) {
  const relPath = relative(PROTO_ROOT, absPath);
  const text = stripComments(readFileSync(absPath, 'utf8'));

  for (const m of text.matchAll(/\bmessage\s+(\w+)\s*\{/g)) {
    const block = messageBlock(text, m[1]);
    if (block !== null && FIELD('offset').test(block)) {
      console.error(
        `[lint-pagination] ${relPath}: message ${m[1]} declares an offset field. ` +
        `A list request uses page_size and page_token (ADR-0028, rule 3).`,
      );
      violations++;
    }
  }

  for (const rpc of text.matchAll(/\brpc\s+(List\w*)\s*\(\s*(?:stream\s+)?(\w+)\s*\)/g)) {
    const block = messageBlock(text, rpc[2]);
    if (block !== null && FIELD('limit').test(block)) {
      console.error(
        `[lint-pagination] ${relPath}: ${rpc[1]} uses limit pagination. ` +
        `A list request uses page_size and page_token (ADR-0028, rule 3).`,
      );
      violations++;
    }
  }
}

if (violations > 0) {
  console.error(`[lint-pagination] FAIL: ${violations} page field(s) of the wrong shape.`);
  process.exit(1);
}
console.log('[lint-pagination] SDK protos: each list request uses page_size and page_token.');
