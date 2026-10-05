#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Zero Root AI

/**
 * lint-idempotency-key.mjs: each request that creates something or starts
 * work has an idempotency_key (ADR-0028, rule 2; sdk#207).
 *
 * The lint reads each *.proto file under api/proto/gibson. It fails when:
 *
 *   - an RPC whose name starts with Create, Run, Start or Submit has a request
 *     message with no field `string idempotency_key` that has a
 *     buf.validate max_len rule, or
 *   - a message with the name (Create|Run|Start|Submit)...Request has no such
 *     field (the verb is a full word: RunnerStatusRequest is not a match), or
 *   - the walk finds no proto file or no such RPC, because that run checked
 *     nothing.
 *
 * No allowlist exists. A request that must not have the field needs a
 * different verb.
 *
 * LINT_IDEMPOTENCY_PROTO_ROOT points the walk at a different proto tree. Only
 * the fixture in scripts/__tests__/lint-idempotency-key.test.sh sets it.
 */

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(__dirname, '..');
const PROTO_ROOT = process.env.LINT_IDEMPOTENCY_PROTO_ROOT || join(REPO_ROOT, 'api/proto/gibson');

const VERB = '(?:Create|Run|Start|Submit)';
const RPC_RE = new RegExp(`\\brpc\\s+(${VERB}[A-Z0-9]\\w*|${VERB})\\s*\\(\\s*(?:stream\\s+)?([\\w.]+)\\s*\\)`, 'g');
const MESSAGE_RE = new RegExp(`\\bmessage\\s+(${VERB}(?:[A-Z0-9]\\w*)?Request)\\s*\\{`, 'g');
const FIELD_RE = /\bstring\s+idempotency_key\s*=\s*\d+\s*\[[^\]]*\bmax_len\b[^\]]*\]\s*;/;

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

// stripComments removes block and line comments, so a field that a comment
// names is not a field.
function stripComments(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\n]*/g, '');
}

// messageBlock returns the body of the named message, or null.
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

const protoFiles = findProtos(PROTO_ROOT);
if (protoFiles.length === 0) {
  console.error(`[lint-idempotency-key] FAIL: no *.proto file under ${PROTO_ROOT}, so this run checked nothing.`);
  process.exit(1);
}

let violations = 0;
let checked = 0;

for (const absPath of protoFiles) {
  const relPath = relative(PROTO_ROOT, absPath);
  const text = stripComments(readFileSync(absPath, 'utf8'));

  // name of the request message -> the reason it must have the field.
  const required = new Map();
  for (const m of text.matchAll(RPC_RE)) {
    required.set(m[2], `the request of rpc ${m[1]}`);
  }
  for (const m of text.matchAll(MESSAGE_RE)) {
    if (!required.has(m[1])) required.set(m[1], `message ${m[1]}`);
  }

  for (const [name, reason] of required) {
    checked++;
    const block = messageBlock(text, name.split('.').pop());
    if (block === null) {
      console.error(
        `[lint-idempotency-key] ${relPath}: ${reason} is ${name}, and this file does not define it. ` +
        `Define the request message in the file of its service.`,
      );
      violations++;
      continue;
    }
    if (!FIELD_RE.test(block)) {
      console.error(
        `[lint-idempotency-key] ${relPath}: ${reason} has no idempotency_key. ` +
        `Add \`string idempotency_key = N [(buf.validate.field).string = { max_len: 128 }];\` to ${name} (ADR-0028).`,
      );
      violations++;
    }
  }
}

if (checked === 0) {
  console.error(`[lint-idempotency-key] FAIL: no create, run, start or submit request under ${PROTO_ROOT}, so this run checked nothing.`);
  process.exit(1);
}

if (violations > 0) {
  console.error(`[lint-idempotency-key] FAIL: ${violations} request(s) with no idempotency_key.`);
  process.exit(1);
}

console.log(`[lint-idempotency-key] SDK protos: ${checked} create, run, start and submit requests have an idempotency_key.`);
