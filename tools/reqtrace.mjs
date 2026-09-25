#!/usr/bin/env node
// Requirement traceability for docs/requirements.
//
// Reads every requirement file (docs/requirements/<scope>/REQ-*.md), validates
// its front matter against docs/requirements/README.md, collects the
// `Implements:` and `Verifies:` annotations from the tracked sources, and writes
// docs/requirements/TRACEABILITY.md.
//
//	node tools/reqtrace.mjs          rewrite TRACEABILITY.md
//	node tools/reqtrace.mjs --check  fail if a requirement or an annotation is
//	                                 invalid, or TRACEABILITY.md is stale
//
// Annotations are located by declaration name rather than by line number, so
// editing unrelated code does not make the matrix stale.

import { execFileSync } from 'node:child_process';
import { readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = join(fileURLToPath(import.meta.url), '..', '..');
const REQ_DIR = join(ROOT, 'docs', 'requirements');
const MATRIX = join(REQ_DIR, 'TRACEABILITY.md');

const ENUMS = {
  type: ['functional', 'non-functional', 'interface', 'constraint', 'limitation'],
  priority: ['must', 'should', 'may'],
  status: ['implemented', 'partial', 'not-implemented', 'superseded', 'withdrawn'],
  verification: ['unit', 'integration', 'ui', 'extension', 'e2e', 'manual', 'inspection'],
};
// Verification kinds that a test in the repository can carry out.
const AUTOMATED = new Set(['unit', 'integration', 'ui', 'extension']);
const REQUIRED = ['id', 'uuid', 'title', 'scope', 'type', 'priority', 'status', 'verification'];
const ID = /^REQ-[A-Z0-9]+-\d{3}$/;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const ANNOTATION = /\b(Implements|Verifies):\s*(REQ-[A-Z0-9]+-\d{3}(?:\s*,\s*REQ-[A-Z0-9]+-\d{3})*)/g;
// Paths never annotated: third-party code, fixtures and the specification itself.
const SKIP = [/(^|\/)vendor\//, /(^|\/)node_modules\//, /(^|\/)testdata\//, /^docs\//, /^tools\/reqtrace\.mjs$/];
const TEXT = /(\.(go|m?js|ts|py|sh|ya?ml|css|html)|(^|\/)go\.mod)$/;

const errors = [];
const fail = (msg) => errors.push(msg);

/** Parses the small YAML subset the requirement front matter uses. */
function frontMatter(text, file) {
  const m = /^---\n([\s\S]*?)\n---\n/.exec(text);
  if (!m) {
    fail(`${file}: no front matter`);
    return null;
  }
  const out = {};
  let list = null;
  for (const line of m[1].split('\n')) {
    if (!line.trim()) continue;
    const item = /^\s+-\s+(.*)$/.exec(line);
    if (item) {
      if (!list) fail(`${file}: list item outside a list: ${line}`);
      else out[list].push(unquote(item[1]));
      continue;
    }
    const kv = /^([a-z_]+):\s*(.*)$/.exec(line);
    if (!kv) {
      fail(`${file}: cannot read front matter line: ${line}`);
      continue;
    }
    const [, key, value] = kv;
    if (value === '') {
      out[key] = [];
      list = key;
    } else if (value.startsWith('[')) {
      out[key] = value.replace(/^\[|\]$/g, '').split(',').map((s) => unquote(s.trim())).filter(Boolean);
      list = null;
    } else {
      out[key] = unquote(value);
      list = null;
    }
  }
  return out;
}

const unquote = (s) => s.replace(/^(['"])(.*)\1$/, '$2');

function loadRequirements() {
  const requirements = [];
  for (const scope of readdirSync(REQ_DIR).sort()) {
    const dir = join(REQ_DIR, scope);
    if (!statSync(dir).isDirectory()) continue;
    for (const name of readdirSync(dir).sort()) {
      if (!name.endsWith('.md')) continue;
      const file = relative(ROOT, join(dir, name)).split(sep).join('/');
      const fm = frontMatter(readFileSync(join(dir, name), 'utf8'), file);
      if (!fm) continue;
      for (const key of REQUIRED) if (fm[key] === undefined) fail(`${file}: missing ${key}`);
      if (!ID.test(fm.id ?? '')) fail(`${file}: malformed id ${fm.id}`);
      else if (!name.startsWith(`${fm.id}-`)) fail(`${file}: file name does not start with ${fm.id}-`);
      if (!UUID.test(fm.uuid ?? '')) fail(`${file}: malformed uuid ${fm.uuid}`);
      if (fm.scope !== scope) fail(`${file}: scope ${fm.scope} is not its directory ${scope}`);
      if (fm.id && fm.scope && !fm.id.startsWith(`REQ-${fm.scope.toUpperCase()}-`)) {
        fail(`${file}: id ${fm.id} does not carry the scope prefix REQ-${fm.scope.toUpperCase()}-`);
      }
      for (const key of ['type', 'priority', 'status']) {
        if (fm[key] !== undefined && !ENUMS[key].includes(fm[key])) fail(`${file}: ${key} ${fm[key]} is not one of ${ENUMS[key].join(', ')}`);
      }
      for (const v of [].concat(fm.verification ?? [])) {
        if (!ENUMS.verification.includes(v)) fail(`${file}: verification ${v} is not one of ${ENUMS.verification.join(', ')}`);
      }
      if (fm.status === 'superseded' && !(fm.superseded_by ?? []).length) fail(`${file}: superseded without superseded_by`);
      requirements.push({ ...fm, source: [].concat(fm.source ?? []), verification: [].concat(fm.verification ?? []), superseded_by: [].concat(fm.superseded_by ?? []), file, impl: [], tests: [] });
    }
  }
  const seen = new Map();
  for (const r of requirements) {
    for (const key of ['id', 'uuid']) {
      const k = `${key}:${r[key]}`;
      if (seen.has(k)) fail(`${r.file}: ${key} ${r[key]} also used by ${seen.get(k)}`);
      else seen.set(k, r.file);
    }
  }
  for (const r of requirements) {
    for (const s of r.superseded_by) if (!seen.has(`id:${s}`)) fail(`${r.file}: superseded_by names unknown ${s}`);
  }
  return requirements;
}

/** The name declared at or after line i, for a stable location label. */
function declaration(lines, i) {
  const patterns = [
    /^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)/,
    /^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s*([A-Za-z_$][\w$]*)/,
    /^\s*(?:export\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)/,
    /^\s*(?:export\s+)?(?:const|let|var|type|interface|enum)\s+\(?\s*([A-Za-z_$][\w$]*)/,
    /^\s*def\s+([A-Za-z_]\w*)/,
    /^\s*(?:describe|it|test)\s*\(\s*(['"`])(.+?)\1/,
    /^\s*(?:(?:static|async|get|set|private|public|protected|readonly)\s+)*([A-Za-z_$][\w$]*)\s*\([^;]*\)\s*(?::[^{]*)?\{\s*$/,
    /^\s*([A-Za-z_$][\w$]*)\s*[:=]\s*(?:async\s+)?(?:function|\(|[A-Za-z_$][\w$]*\s*=>)/,
    /^\s*-?\s*name:\s*(.+)$/,
    /^\s*([A-Za-z_][\w-]*):\s*$/,
  ];
  for (let j = i; j < Math.min(lines.length, i + 8); j++) {
    const line = lines[j];
    if (/^\s*(\/\/|#|\/\*|\*|<!--)/.test(line) && j > i) continue;
    for (const p of patterns) {
      const m = p.exec(line);
      if (m) return (m[2] ?? m[1]).trim();
    }
    if (j > i && line.trim()) break;
  }
  return null;
}

function scanAnnotations(requirements) {
  const byId = new Map(requirements.map((r) => [r.id, r]));
  const files = execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard'], { cwd: ROOT, encoding: 'utf8' })
    .split('\n')
    .filter((f) => f && TEXT.test(f) && !SKIP.some((p) => p.test(f)));
  for (const file of files.sort()) {
    let text;
    try {
      text = readFileSync(join(ROOT, file), 'utf8');
    } catch {
      continue; // deleted in the working tree
    }
    if (!text.includes('REQ-')) continue;
    const lines = text.split('\n');
    // CI workflows count as tests: a job that builds and runs every release target
    // verifies what no unit test can.
    const test = /(_test\.go|\.test\.m?js|\/uitest\/|\/test\/|^\.github\/workflows\/)/.test(file);
    lines.forEach((line, i) => {
      for (const m of line.matchAll(ANNOTATION)) {
        const [, kind, list] = m;
        const where = { file, name: declaration(lines, i + 1) ?? declaration(lines, i) };
        for (const id of list.split(',').map((s) => s.trim())) {
          const r = byId.get(id);
          if (!r) {
            fail(`${file}:${i + 1}: ${kind} names unknown requirement ${id}`);
            continue;
          }
          if (kind === 'Verifies' && !test) fail(`${file}:${i + 1}: Verifies outside a test file`);
          (kind === 'Implements' ? r.impl : r.tests).push(where);
        }
      }
    });
  }
  for (const r of requirements) {
    if ((r.status === 'implemented' || r.status === 'partial') && !r.impl.length) {
      fail(`${r.file}: status ${r.status} but no Implements: annotation`);
    }
    if ((r.status === 'not-implemented' || r.status === 'withdrawn') && r.impl.length) {
      fail(`${r.file}: status ${r.status} but annotated as implemented in ${r.impl[0].file}`);
    }
  }
}

const link = (from, file) => relative(from, join(ROOT, file)).split(sep).join('/');
const loc = (w) => `[${w.file}](${link(REQ_DIR, w.file)})${w.name ? ` \`${w.name.replace(/[`|]/g, '')}\`` : ''}`;
const cell = (s) => String(s).replace(/\|/g, '\\|');

/**
 * A Markdown table with its pipes aligned: every cell padded to its column's width
 * and the delimiter row spelled out to match, so the source reads as a table and
 * markdownlint's table-column-style rule (MD060) accepts it. `right` names the
 * columns aligned right, which is where counts belong.
 */
function table(header, rows, right = []) {
  const all = [header, ...rows].map((r) => r.map(String));
  const width = header.map((_, i) => Math.max(3, ...all.map((r) => r[i].length)));
  const line = (r) => `| ${r.map((c, i) => (right.includes(i) ? c.padStart(width[i]) : c.padEnd(width[i]))).join(' | ')} |`;
  const rule = `| ${width.map((w, i) => (right.includes(i) ? `${'-'.repeat(w - 1)}:` : '-'.repeat(w))).join(' | ')} |`;
  return [line(all[0]), rule, ...all.slice(1).map(line)];
}

function render(requirements) {
  const scopes = [...new Set(requirements.map((r) => r.scope))];
  const count = (list, pred) => list.filter(pred).length;
  const live = (r) => r.status !== 'superseded' && r.status !== 'withdrawn';
  const unverified = (r) => live(r) && r.status !== 'not-implemented' && !r.tests.length;
  const untested = (r) => live(r) && r.status !== 'not-implemented' && r.verification.some((v) => AUTOMATED.has(v)) && !r.tests.length;
  const out = [];
  out.push('# Requirements traceability matrix', '');
  out.push('<!-- Generated by tools/reqtrace.mjs; do not edit by hand. -->');
  out.push('<!-- markdownlint-disable MD033 -->', '');
  out.push('Generated from the requirement files in this directory and the', '`Implements:` and `Verifies:` annotations in the source tree. Regenerate', 'it with `node tools/reqtrace.mjs`.', '');
  out.push('## Summary', '');
  const row = (name, list) => [name, list.length, count(list, (r) => r.status === 'implemented'), count(list, (r) => r.status === 'partial'), count(list, (r) => r.status === 'not-implemented'), count(list, (r) => !live(r)), count(list, untested), count(list, unverified)];
  out.push(
    ...table(
      ['Scope', 'Requirements', 'Implemented', 'Partial', 'Not implemented', 'Superseded / withdrawn', 'Expected automated test missing', 'No test at all'],
      [...scopes.map((s) => row(`[${s}](#${s})`, requirements.filter((r) => r.scope === s))), row('**Total**', requirements)],
      [1, 2, 3, 4, 5, 6, 7],
    ),
    '',
  );

  const gap = (title, list, extra) => {
    out.push(`### ${title}`, '');
    if (!list.length) out.push('None.', '');
    else {
      const header = extra ? ['ID', 'Title', 'Expected tests'] : ['ID', 'Title'];
      const rows = list.map((r) => [`[${r.id}](${link(REQ_DIR, r.file)})`, cell(r.title), ...(extra ? [extra(r)] : [])]);
      out.push(...table(header, rows), '');
    }
  };
  out.push('## Coverage gaps', '');
  gap('Not implemented', requirements.filter((r) => r.status === 'not-implemented'));
  gap('Partially implemented', requirements.filter((r) => r.status === 'partial'));
  gap('No automated test', requirements.filter(untested), (r) => r.verification.filter((v) => AUTOMATED.has(v)).join(', '));

  for (const s of scopes) {
    out.push(`## ${s}`, '');
    const rows = [];
    for (const r of requirements.filter((x) => x.scope === s)) {
      const status = r.superseded_by.length ? `${r.status} by ${r.superseded_by.join(', ')}` : r.status;
      rows.push([`[${r.id}](${link(REQ_DIR, r.file)})`, `\`${r.uuid}\``, cell(r.title), r.type, status, r.verification.join(', '), r.impl.map(loc).join('<br>') || '—', r.tests.map(loc).join('<br>') || '—']);
    }
    out.push(...table(['ID', 'UUID', 'Title', 'Type', 'Status', 'Verification', 'Implemented in', 'Verified by'], rows), '');
  }
  return out.join('\n');
}

const requirements = loadRequirements();
scanAnnotations(requirements);
const text = render(requirements);
if (process.argv.includes('--check')) {
  let current = '';
  try {
    current = readFileSync(MATRIX, 'utf8');
  } catch {}
  if (current !== text) fail(`${relative(ROOT, MATRIX)} is stale: run node tools/reqtrace.mjs`);
} else {
  writeFileSync(MATRIX, text);
}
for (const e of errors) console.error(e);
console.error(`${requirements.length} requirements, ${errors.length} problem(s)`);
process.exit(errors.length ? 1 : 0);
