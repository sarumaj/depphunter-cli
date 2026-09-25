// Graph document -> navigable tree with aggregates and edge indexes.

// Roughly what a line of source weighs, for the files that have no lines to count.
// Anything binary, and anything over --max-file-size, is listed without being read,
// so it arrives with a size in bytes and nothing else; sized by lines alone every one
// of them came out at the floor height, which drew a 4 MB model as the same flat slab
// as an empty file. This is the conversion that gives them a storey instead. It is an
// average of source, not a measurement of anything - which is why it is only ever
// used for how big a thing is drawn, and never for a number the map puts in words.
const BYTES_PER_LINE = 40;

/**
 * How big a file is for the purpose of drawing it: its lines where it has any, and
 * what its bytes come to in lines where it has none.
 *
 * Kept apart from `loc` deliberately. Every count the map states out loud - the lines
 * in the status bar, a file's Lines in the panel, what a filter totals - stays lines
 * that were actually counted, because a stand-in in one of those is a lie. This is
 * the other thing: the one the geometry and the size palette ask for, where the
 * question is only ever "how much is there".
 * Implements: REQ-MAP-058
 */
export const bulk = n =>
  n.loc || Math.round((n.bytes || 0) / BYTES_PER_LINE);

/**
 * Whether a file's size is only known in bytes: nothing read it, so there are no
 * lines to report and "0 lines" would be a statement about the file rather than
 * about the reading of it. What the UI says instead is how big it is.
 * Implements: REQ-MAP-060
 */
export const unread = n => n.kind === 'file' && !n.loc && !!n.bytes;

/** Bytes, for somewhere there is room to say it: 4.1 MB, 812 kB, 96 bytes. */
// Implements: REQ-MAP-060
export function fileSize(bytes) {
  if (!bytes) return '0 bytes';
  if (bytes < 1000) return `${bytes} bytes`;
  const units = ['kB', 'MB', 'GB', 'TB'];
  let n = bytes / 1000, i = 0;
  while (n >= 1000 && i < units.length - 1) { n /= 1000; i++; }
  return `${n < 10 ? n.toFixed(1) : Math.round(n)} ${units[i]}`;
}

// Implements: REQ-MAP-059, REQ-MAP-008, REQ-MAP-010
export function buildModel(graph) {
  const byId = new Map();
  for (const n of graph.nodes) {
    byId.set(n.id, { ...n, children: [], parentNode: null, depth: 0, fileCount: 0, totalLoc: 0, totalBulk: 0, langLoc: new Map(), importers: 0 });
  }
  for (const n of byId.values()) {
    const p = n.parent && byId.get(n.parent);
    if (p) { n.parentNode = p; p.children.push(n); }
  }

  const root = byId.get('d:.');
  const ecosystems = [...byId.values()].filter(n => n.kind === 'ecosystem');

  const walk = (n, depth) => {
    n.depth = depth;
    for (const c of n.children) walk(c, depth + 1);
    if (n.kind === 'file') {
      n.fileCount = 1;
      n.totalLoc = n.loc || 0;
      n.totalBulk = bulk(n);
      n.langLoc.set(n.lang || '', n.totalLoc);
    } else if (n.kind === 'dir') {
      for (const c of n.children) {
        if (c.kind !== 'dir' && c.kind !== 'file') continue;
        n.fileCount += c.fileCount;
        n.totalLoc += c.totalLoc;
        n.totalBulk += c.totalBulk;
        for (const [l, v] of c.langLoc) n.langLoc.set(l, (n.langLoc.get(l) || 0) + v);
      }
    }
  };
  walk(root, 0);
  for (const e of ecosystems) walk(e, 0);

  // Stable, learnable order: directories first, then files, each alphabetical.
  const order = { dir: 0, file: 1, symbol: 2, package: 3 };
  for (const n of byId.values()) {
    n.children.sort((a, b) => (order[a.kind] - order[b.kind]) || a.name.localeCompare(b.name));
  }

  const edgesFrom = new Map(), edgesTo = new Map();
  for (const e of graph.edges) {
    if (!byId.has(e.from) || !byId.has(e.to)) continue;
    push(edgesFrom, e.from, e);
    push(edgesTo, e.to, e);
    const t = byId.get(e.to);
    // "depends" edges come from other packages (--resolve-depth), not from files.
    if (t.kind === 'package' && e.kind !== 'depends') t.importers++;
  }

  return { graph, byId, root, ecosystems, edgesFrom, edgesTo, refsFrom: new Map(), refsTo: new Map(), languages: rankLanguages(root) };
}

// setReferences indexes symbol-level reference edges (from language servers) apart
// from imports, so the two kinds are shown and counted separately.
export function setReferences(model, edges) {
  model.refsFrom = new Map();
  model.refsTo = new Map();
  for (const e of edges || []) {
    if (!model.byId.has(e.from) || !model.byId.has(e.to)) continue;
    push(model.refsFrom, e.from, e);
    push(model.refsTo, e.to, e);
  }
}

function push(map, k, v) {
  const a = map.get(k);
  if (a) a.push(v); else map.set(k, [v]);
}

// Languages ordered by lines of code; the order decides categorical color slots once
// per repository, so a language keeps its color regardless of later filtering.
// Implements: REQ-MAP-036
function rankLanguages(root) {
  return [...root.langLoc.entries()]
    .filter(([l]) => l)
    .sort((a, b) => b[1] - a[1])
    .map(([l, loc]) => ({ lang: l, loc }));
}

export function isWithin(n, ancestor) {
  for (let p = n; p; p = p.parentNode) if (p === ancestor) return true;
  return false;
}

export function ancestors(n) {
  const out = [];
  for (let p = n.parentNode; p; p = p.parentNode) out.unshift(p);
  return out;
}

// Edges of a kind ('import' or 'reference') crossing the boundary of `sel`'s subtree,
// as {out, in} lists of raw edges.
// Implements: REQ-MOD-007, REQ-MAP-013
export function boundaryEdges(model, sel, kind = 'import') {
  const from = kind === 'reference' ? model.refsFrom : model.edgesFrom;
  const to = kind === 'reference' ? model.refsTo : model.edgesTo;
  const out = [], inc = [];
  const visit = n => {
    for (const e of from.get(n.id) || []) if (!isWithin(model.byId.get(e.to), sel)) out.push(e);
    for (const e of to.get(n.id) || []) if (!isWithin(model.byId.get(e.from), sel)) inc.push(e);
    for (const c of n.children) visit(c);
  };
  visit(sel);
  return { out, in: inc };
}

/**
 * The arcs drawn for a selection: the edges crossing its subtree's boundary, joined
 * between the representatives of their ends (`rep`, only for ends `visible` leaves
 * standing), one arc per pair and direction with the number of edges it stands for,
 * the `max` largest first. Null when nothing is selected or the selection is not drawn.
 * Returns {selBox, arcs: [{from, to, color, count}], lit: Set<box>}.
 * Implements: REQ-MAP-012, REQ-MAP-013
 */
export function focusArcs(model, sel, { kind = 'import', rep, visible, outColor, inColor, max = Infinity }) {
  const selBox = sel && rep(sel);
  if (!sel || !selBox) return null;
  const { out, in: inc } = boundaryEdges(model, sel, kind);
  const agg = new Map();
  const add = (from, to, color) => {
    if (!from || !to || from === to) return;
    const key = `${from.i}>${to.i}>${color}`;
    const a = agg.get(key) || { from, to, color, count: 0 };
    a.count++;
    agg.set(key, a);
  };
  const shown = id => {
    const n = model.byId.get(id);
    return visible(n) ? rep(n) : null;
  };
  for (const e of out) add(selBox, shown(e.to), outColor);
  for (const e of inc) add(shown(e.from), selBox, inColor);
  const arcs = [...agg.values()].sort((a, b) => b.count - a.count).slice(0, max);
  const lit = new Set([selBox]);
  for (const a of arcs) { lit.add(a.from); lit.add(a.to); }
  return { lit, arcs, selBox };
}

/**
 * The expansion for stepping to depth `level`, clamped to 1 and the deepest
 * directory level: every directory above it open, every one at or below it shut.
 * Returns {level, maxDepth, expanded: Set<id>}.
 * Implements: REQ-MAP-023
 */
export function expandToLevel(model, level) {
  let maxDepth = 0;
  for (const n of model.byId.values()) if (n.kind === 'dir') maxDepth = Math.max(maxDepth, n.depth + 1);
  level = Math.max(1, Math.min(maxDepth, level));
  const expanded = new Set([...model.byId.values()].filter(n => n.kind === 'dir' && n.depth < level).map(n => n.id));
  return { level, maxDepth, expanded };
}

/**
 * What toggling `n` would do given the expanded set: 'open', 'close', or '' when
 * there is nothing to open. A symbol toggles its file.
 * Implements: REQ-MAP-022
 */
export function toggles(n, expanded) {
  if (n.kind === 'symbol') n = n.parentNode;
  if (n.kind === 'file' && !n.children.length) return '';
  if (n.kind !== 'dir' && n.kind !== 'file') return '';
  return expanded.has(n.id) ? 'close' : 'open';
}
