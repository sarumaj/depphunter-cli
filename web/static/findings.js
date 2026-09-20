// What the scanners said, placed on the map: a vulnerability belongs to the package it
// affects, a linter's complaint to the file it is about, and a directory carries what
// lies below it. The same index answers three questions - what the panel lists, what
// colour the streets' bugs are, and how many there are still to catch.

export const SEVERITIES = ['critical', 'high', 'medium', 'low', 'info', 'unknown'];
const RANK = { critical: 5, high: 4, medium: 3, low: 2, info: 1, unknown: 0 };

export const rankOf = sev => RANK[sev] ?? 0;

/** The worst of two severities, either of which may be missing. */
export const worse = (a, b) => (!a ? b : !b ? a : rankOf(a) >= rankOf(b) ? a : b);

/** Severity colours, read from the stylesheet so both themes choose their own. */
export function severityColors() {
  const cs = getComputedStyle(document.documentElement);
  const out = {};
  for (const s of SEVERITIES) out[s] = cs.getPropertyValue(`--sev-${s}`).trim() || cs.getPropertyValue('--muted').trim();
  return out;
}

/**
 * indexFindings places a findings document (internal/findings.Set) on the model.
 *
 * Returns { all, own, rollup, place, sources, partial }:
 *   own     nodeId -> the findings listed on that node itself
 *   rollup  nodeId -> { count, worst } including everything below it
 *   place   finding -> the node its bug stands at (a package before a file)
 */
export function indexFindings(set, model) {
  const all = [], own = new Map(), home = new Map();
  for (const f of set?.findings || []) {
    const pkg = f.package ? model.byId.get(`p:${f.ecosystem}:${f.package}`) : null;
    const file = f.path ? model.byId.get(`f:${f.path}`) : null;
    // A finding with a path this map does not show (a lock file, a vendored copy) is
    // still about somewhere: the nearest directory that is on the map takes it.
    const near = !file && f.path ? nearestDir(model, f.path) : null;
    const on = [pkg, file, near].filter(Boolean);
    if (!on.length) on.push(model.root); // a whole-repository finding
    all.push(f);
    home.set(f, on[0]);
    for (const n of on) push(own, n.id, f);
  }

  // Everything below a directory, an ecosystem or a file with symbols counts towards it.
  const rollup = new Map();
  const visit = n => {
    let count = 0, worst = undefined;
    for (const c of n.children) {
      const r = visit(c);
      count += r.count;
      worst = worse(worst, r.worst);
    }
    for (const f of own.get(n.id) || []) {
      count++;
      worst = worse(worst, f.severity);
    }
    const r = { count, worst };
    rollup.set(n.id, r);
    return r;
  };
  visit(model.root);
  for (const e of model.ecosystems) visit(e);

  return {
    all,
    own: id => own.get(id) || [],
    rollup: id => rollup.get(id) || { count: 0, worst: undefined },
    place: f => home.get(f) || model.root,
    sources: set?.sources || [],
    partial: !!set?.partial,
  };
}

// nearestDir walks a path upwards until it finds a directory the map draws.
function nearestDir(model, path) {
  let dir = path;
  for (let i = 0; i < 64; i++) {
    const cut = dir.lastIndexOf('/');
    dir = cut < 0 ? '.' : dir.slice(0, cut);
    const n = model.byId.get(`d:${dir}`);
    if (n) return n;
    if (dir === '.') return null;
  }
  return null;
}

/** A one-line summary of where a finding is, for a list row or a bug's label. */
export function whereOf(f) {
  if (f.package) return f.version ? `${f.package}@${f.version}` : f.package;
  if (f.path) return f.line ? `${f.path}:${f.line}` : f.path;
  return 'this repository';
}

function push(map, k, v) {
  const a = map.get(k);
  if (a) a.push(v); else map.set(k, [v]);
}
