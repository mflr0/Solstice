#!/usr/bin/env node
/**
 * Round-trip statistics of the rich editor over real texts.
 *
 *   node scripts/wiki-roundtrip-stats.mjs [--detail] < texts.jsonl
 *
 * Each input line is a JSON string (a description or a comment in wiki
 * markup), or a JSON object whose string values are such texts. Every text is
 * parsed into an editor document and written back as if it had been edited
 * entirely (untouched texts are always saved unchanged; this measures the
 * worst case). Only aggregate counts are printed, never any text.
 *
 * --detail adds one line per text that comes back different or changes on a
 * second pass: the constructs it contains and the kind of difference (block
 * structure, text or inline formatting, and the tags around it), still
 * without any of its content.
 */

import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';
import { runnerImport } from 'vite';

const detail = process.argv.includes('--detail');
const root = fileURLToPath(new URL('..', import.meta.url));
const { module: rt } = await runnerImport('/src/core/wiki-roundtrip.ts', {
  root,
  configFile: false,
  logLevel: 'silent',
});

const verdicts = { identical: 0, 'same-html': 0, 'same-meaning': 0, different: 0 };
/** construct → { texts, sameMeaning, different } */
const constructs = new Map();
let lines = 0;
let texts = 0;
let unreadable = 0;
let failures = 0;
let unstable = 0;
let differentWithAllRefs = 0;
let slowest = 0;
let totalMs = 0;
let chars = 0;
/** With --detail: "constructs | difference", for different and unstable texts. */
const details = [];

function check(src) {
  texts++;
  chars += src.length;
  const start = performance.now();
  let result;
  try {
    result = rt.roundTrip(src);
  } catch {
    failures++;
    return;
  }
  const ms = performance.now() - start;
  totalMs += ms;
  slowest = Math.max(slowest, ms);
  verdicts[result.verdict]++;
  // Different even when every attachment, user and issue exists?
  if (!result.sameWithAllRefs) differentWithAllRefs++;
  const names = result.constructs.join(' ') || '-';
  if (detail && result.verdict === 'different') {
    details.push(`different  ${names} | ${rt.differenceKind(src, result.canonical)}`);
  }
  try {
    const again = rt.canonicalWiki(result.canonical);
    if (again !== result.canonical) {
      unstable++;
      if (detail) {
        const kind = rt.differenceKind(result.canonical, again) || 'markup only, same rendering';
        details.push(`unstable   ${names} | ${kind}`);
      }
    }
  } catch {
    unstable++;
    if (detail) details.push(`unstable   ${names} | second pass failed`);
  }
  for (const c of result.constructs) {
    const entry = constructs.get(c) ?? { texts: 0, sameMeaning: 0, different: 0 };
    entry.texts++;
    if (result.verdict === 'same-meaning') entry.sameMeaning++;
    if (result.verdict === 'different') entry.different++;
    constructs.set(c, entry);
  }
}

const input = createInterface({ input: process.stdin, crlfDelay: Infinity });
for await (const line of input) {
  if (line.trim() === '') continue;
  lines++;
  let value;
  try {
    value = JSON.parse(line);
  } catch {
    unreadable++;
    continue;
  }
  if (typeof value === 'string') check(value);
  else if (value && typeof value === 'object') {
    for (const v of Object.values(value)) if (typeof v === 'string') check(v);
  } else unreadable++;
}

const pct = (n) => (texts ? `${((100 * n) / texts).toFixed(2)} %` : '-');
const out = [];
out.push(`lines read: ${lines} (unreadable: ${unreadable})`);
out.push(`texts: ${texts}, characters: ${chars}`);
out.push('');
out.push('canonical round trip (every block rewritten):');
out.push(
  `  identical source       ${String(verdicts.identical).padStart(8)}  ${pct(verdicts.identical)}`,
);
out.push(
  `  same HTML              ${String(verdicts['same-html']).padStart(8)}  ${pct(verdicts['same-html'])}`,
);
out.push(
  `  same text and marks    ${String(verdicts['same-meaning']).padStart(8)}  ${pct(verdicts['same-meaning'])}`,
);
out.push(
  `  different              ${String(verdicts.different).padStart(8)}  ${pct(verdicts.different)}`,
);
out.push(`    of which also when every reference exists: ${differentWithAllRefs}`);
out.push(`  errors                 ${String(failures).padStart(8)}`);
out.push(`  second pass changes    ${String(unstable).padStart(8)}`);
out.push(`time: ${totalMs.toFixed(0)} ms in all, slowest text ${slowest.toFixed(1)} ms`);
out.push('');
out.push('constructs (texts containing it / same text and marks / different):');
const rows = [...constructs.entries()].sort(
  (a, b) => b[1].different - a[1].different || b[1].texts - a[1].texts,
);
for (const [name, e] of rows) {
  out.push(
    `  ${name.padEnd(28)} ${String(e.texts).padStart(8)} ${String(e.sameMeaning).padStart(8)} ${String(e.different).padStart(8)}`,
  );
}
if (detail) {
  out.push('');
  out.push('different or unstable texts (constructs | difference), in no particular order:');
  for (const line of details.sort()) out.push(`  ${line}`);
}
process.stdout.write(`${out.join('\n')}\n`);
