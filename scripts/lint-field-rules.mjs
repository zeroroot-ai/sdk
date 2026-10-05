#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

/**
 * lint-field-rules.mjs: each request of an sdk service states its field rules
 * (ADR-0028, rule 1; sdk#185).
 *
 * The daemon runs protovalidate on each request before the handler runs. It
 * can check only a rule that a proto states. A string field with no rule has
 * no bound, and an enum field with no rule accepts a number that the enum does
 * not define.
 *
 * The lint reads each *.proto file under api/proto/gibson. In the request
 * message of each RPC, each string field and each enum field that is not
 * repeated and not a map must carry a (buf.validate.field) option. It fails
 * on a field with no rule, and on a run that found no proto file or no RPC.
 *
 * LINT_FIELD_RULES_PROTO_ROOT points the walk at a different tree. Only the
 * fixture in scripts/__tests__/lint-field-rules.test.sh sets it.
 */

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, '..');
const PROTO_ROOT = process.env.LINT_FIELD_RULES_PROTO_ROOT || join(REPO_ROOT, 'api/proto/gibson');

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

function stripComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\n]*/g, '');
}

// block returns the body after the brace at index open, to its matching brace.
function block(text, open) {
  let depth = 1;
  let i = open + 1;
  while (i < text.length && depth > 0) {
    if (text[i] === '{') depth++;
    else if (text[i] === '}') depth--;
    i++;
  }
  return text.slice(open + 1, i - 1);
}

// topLevel removes each nested message and enum block, and keeps the body of
// a oneof, so only the fields of the message itself are left.
function topLevel(body) {
  let out = '';
  let i = 0;
  const nested = /\b(message|enum)\s+\w+\s*\{/g;
  let m;
  while ((m = nested.exec(body)) !== null) {
    if (m.index < i) continue;
    out += body.slice(i, m.index);
    const open = m.index + m[0].length - 1;
    i = open + block(body, open).length + 2;
    nested.lastIndex = i;
  }
  return out + body.slice(i);
}

const protoFiles = findProtos(PROTO_ROOT);
if (protoFiles.length === 0) {
  console.error(`[lint-field-rules] FAIL: no *.proto file under ${PROTO_ROOT}, so this run checked nothing.`);
  process.exit(1);
}

const texts = new Map(protoFiles.map((p) => [p, stripComments(readFileSync(p, 'utf8'))]));
const enums = new Set();
for (const text of texts.values()) {
  for (const m of text.matchAll(/\benum\s+(\w+)\s*\{/g)) enums.add(m[1]);
}

const FIELD = /^\s*(optional\s+)?([\w.]+)\s+(\w+)\s*=\s*\d+\s*(\[[\s\S]*?\])?\s*;/gm;
let violations = 0;
let requests = 0;

for (const [absPath, text] of texts) {
  const relPath = relative(PROTO_ROOT, absPath);
  const inputs = new Set();
  for (const m of text.matchAll(/\brpc\s+\w+\s*\(\s*(?:stream\s+)?([\w.]+)\s*\)/g)) {
    inputs.add(m[1].split('.').pop());
  }
  for (const name of inputs) {
    const m = new RegExp(`\\bmessage\\s+${name}\\s*\\{`).exec(text);
    if (!m) continue; // the request is defined in a different file
    requests++;
    const body = topLevel(block(text, m.index + m[0].length - 1)).replace(/\boneof\s+\w+\s*\{/g, '').replace(/\}/g, '');
    for (const f of body.matchAll(FIELD)) {
      const type = f[2];
      const isString = type === 'string';
      const isEnum = enums.has(type.split('.').pop());
      if (!isString && !isEnum) continue;
      if (f[4] && f[4].includes('buf.validate.field')) continue;
      console.error(
        `[lint-field-rules] ${relPath}: field ${f[3]} of ${name} has no buf.validate rule. ` +
        `Add [(buf.validate.field).string = { max_len: 1024 }] or ` +
        `[(buf.validate.field).enum = { defined_only: true }] (ADR-0028).`,
      );
      violations++;
    }
  }
}

if (requests === 0) {
  console.error(`[lint-field-rules] FAIL: no RPC request under ${PROTO_ROOT}, so this run checked nothing.`);
  process.exit(1);
}
if (violations > 0) {
  console.error(`[lint-field-rules] FAIL: ${violations} request field(s) with no rule.`);
  process.exit(1);
}
console.log(`[lint-field-rules] SDK protos: each string and enum field of ${requests} requests has a rule.`);
