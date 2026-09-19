// Graph document -> navigable tree with aggregates and edge indexes.

export function buildModel(graph) {
  const byId = new Map();
  for (const n of graph.nodes) {
    byId.set(n.id, { ...n, children: [], parentNode: null, depth: 0, fileCount: 0, totalLoc: 0, langLoc: new Map(), importers: 0 });
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
      n.langLoc.set(n.lang || '', n.totalLoc);
    } else if (n.kind === 'dir') {
      for (const c of n.children) {
        if (c.kind !== 'dir' && c.kind !== 'file') continue;
        n.fileCount += c.fileCount;
        n.totalLoc += c.totalLoc;
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
    if (t.kind === 'package') t.importers++;
  }

  return { graph, byId, root, ecosystems, edgesFrom, edgesTo, languages: rankLanguages(root) };
}

function push(map, k, v) {
  const a = map.get(k);
  if (a) a.push(v); else map.set(k, [v]);
}

// Languages ordered by lines of code; the order decides categorical colour slots once
// per repository, so a language keeps its colour regardless of later filtering.
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

// Edges crossing the boundary of `sel`'s subtree, as {out, in} lists of raw edges.
export function boundaryEdges(model, sel) {
  const out = [], inc = [];
  const visit = n => {
    for (const e of model.edgesFrom.get(n.id) || []) if (!isWithin(model.byId.get(e.to), sel)) out.push(e);
    for (const e of model.edgesTo.get(n.id) || []) if (!isWithin(model.byId.get(e.from), sel)) inc.push(e);
    for (const c of n.children) visit(c);
  };
  visit(sel);
  return { out, in: inc };
}
