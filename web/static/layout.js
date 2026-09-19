import potpack from './vendor/potpack.js';

// Archipelago layout: the repository is a mainland of nested terraces, each external
// ecosystem an island north of it. Positions derive only from the hierarchy and the
// expansion state, so the same repository always produces the same map.

const FILE = 1.0;        // building footprint
const GAP = 0.35;        // between siblings
const PAD = 0.55;        // inside a terrace
const TERRACE = 0.28;    // terrace thickness
const MAX_H = 10;        // tallest building
const SYM = 0.42, SYM_GAP = 0.12;
const LAND_MARGIN = 1.2, LAND_H = 0.45, ISLAND_GAP = 4;

export const SCALES = {
  linear: t => t,
  sqrt: t => Math.sqrt(t),
  log: t => Math.log1p(t * 1000) / Math.log1p(1000),
};

/**
 * @returns {{boxes: Box[], byNode: Map<string, Box>, bounds}}
 * Heights use the unfiltered maximum so filtering never rescales what stays visible.
 * Box: {node, kind: 'land'|'terrace'|'district'|'building'|'symbol'|'package', x, z (centre), y, w, d, h}
 */
export function layout(model, state) {
  const { visible, counts } = state.vis;
  const scale = SCALES[state.heightScale] || SCALES.sqrt;
  const maxLoc = fileLocs(model.root).reduce((a, b) => Math.max(a, b), 1);
  const height = loc => 0.2 + scale(Math.min(1, (loc || 0) / maxLoc)) * MAX_H;
  const maxImporters = model.ecosystems.flatMap(e => e.children).reduce((a, p) => Math.max(a, p.importers), 1);
  const pkgHeight = n => 0.3 + scale(n.importers / maxImporters) * MAX_H * 0.5;

  const boxes = [];
  const sizes = new Map();

  const expanded = n => state.expanded.has(n.id);
  const symbols = n => n.children.filter(c => c.kind === 'symbol');

  function measure(n) {
    let s;
    if (n.kind === 'package') {
      s = { w: FILE, d: FILE };
    } else if (n.kind === 'file') {
      const syms = symbols(n);
      if (expanded(n) && syms.length) {
        const cols = Math.ceil(Math.sqrt(syms.length));
        const rows = Math.ceil(syms.length / cols);
        s = { w: cols * (SYM + SYM_GAP) - SYM_GAP + 2 * SYM_GAP, d: rows * (SYM + SYM_GAP) + SYM_GAP, cols };
      } else {
        s = { w: FILE, d: FILE };
      }
    } else if (n.kind === 'dir' && !expanded(n)) {
      const side = Math.max(1.4, Math.sqrt(counts.get(n.id).fileCount) * (FILE + GAP) * 0.75);
      s = { w: side, d: side };
    } else { // expanded dir or ecosystem
      const items = n.children.filter(c => c.kind !== 'symbol' && visible(c)).map(c => ({ n: c, ...measure(c) }));
      s = { ...shelf(items), items };
    }
    sizes.set(n.id, s);
    return s;
  }

  function place(n, x0, z0, y) {
    const s = sizes.get(n.id);
    const cx = x0 + s.w / 2, cz = z0 + s.d / 2;
    if (n.kind === 'file') {
      if (s.cols) {
        boxes.push({ node: n, kind: 'terrace', x: cx, z: cz, y, w: s.w, d: s.d, h: TERRACE * 0.6 });
        const top = y + TERRACE * 0.6;
        symbols(n).forEach((sym, i) => {
          const c = i % s.cols, r = Math.floor(i / s.cols);
          boxes.push({
            node: sym, kind: 'symbol',
            x: x0 + SYM_GAP + c * (SYM + SYM_GAP) + SYM / 2, z: z0 + SYM_GAP + r * (SYM + SYM_GAP) + SYM / 2,
            y: top, w: SYM, d: SYM, h: symbolHeight(sym),
          });
        });
      } else {
        boxes.push({ node: n, kind: 'building', x: cx, z: cz, y, w: s.w, d: s.d, h: height(n.loc) });
      }
      return;
    }
    if (n.kind === 'package') {
      boxes.push({ node: n, kind: 'package', x: cx, z: cz, y, w: s.w, d: s.d, h: pkgHeight(n) });
      return;
    }
    if (n.kind === 'dir' && !expanded(n)) {
      const c = counts.get(n.id);
      boxes.push({ node: n, kind: 'district', x: cx, z: cz, y, w: s.w, d: s.d, h: height(c.totalLoc / Math.max(1, c.fileCount)) });
      return;
    }
    boxes.push({ node: n, kind: 'terrace', x: cx, z: cz, y, w: s.w, d: s.d, h: TERRACE });
    for (const it of s.items) place(it.n, x0 + it.x, z0 + it.z, y + TERRACE);
  }

  // Mainland.
  const main = measure(model.root);
  const land = (x0, z0, w, d, node) => boxes.push({
    node, kind: 'land', x: x0 + w / 2, z: z0 + d / 2, y: -LAND_H, w: w + 2 * LAND_MARGIN, d: d + 2 * LAND_MARGIN, h: LAND_H,
  });
  land(0, 0, main.w, main.d, model.root);
  place(model.root, 0, 0, 0);

  // Islands, left to right along the north shore, largest first.
  const islands = model.ecosystems
    .filter(e => visible(e) && e.children.length)
    .map(e => ({ e, s: measure(e) }))
    .sort((a, b) => b.s.w * b.s.d - a.s.w * a.s.d);
  let x = 0;
  for (const { e, s } of islands) {
    const z0 = -ISLAND_GAP - 2 * LAND_MARGIN - s.d;
    land(x, z0, s.w, s.d, e);
    place(e, x, z0, 0);
    x += s.w + 2 * LAND_MARGIN + ISLAND_GAP;
  }

  // Land sits under a terrace of the same node; the terrace is the node's representative.
  const byNode = new Map();
  boxes.forEach((b, i) => {
    b.i = i;
    if (b.kind !== 'land') byNode.set(b.node.id, b);
  });
  return { boxes, byNode, bounds: bounds(boxes) };
}

function symbolHeight(sym) {
  switch (sym.symbolKind) {
    case 'type': case 'class': case 'interface': return 1.1;
    case 'func': case 'method': case 'function': return 0.7;
    default: return 0.35;
  }
}

function fileLocs(n, out = []) {
  if (n.kind === 'file') out.push(n.loc || 0);
  for (const c of n.children) if (c.kind === 'dir' || c.kind === 'file') fileLocs(c, out);
  return out;
}

// Packs a terrace's children into a near-square area with potpack. Each box carries
// its gap; positions are offset by the terrace padding. potpack's sort is stable and
// children arrive in name order, so the same tree always packs the same way.
function shelf(items) {
  if (!items.length) return { w: FILE + 2 * PAD, d: FILE + 2 * PAD };
  const boxes = items.map(it => ({ w: it.w + GAP, h: it.d + GAP, it }));
  const { w, h } = potpack(boxes);
  for (const b of boxes) {
    b.it.x = PAD + b.x;
    b.it.z = PAD + b.y;
  }
  return { w: Math.max(w - GAP, FILE) + 2 * PAD, d: Math.max(h - GAP, FILE) + 2 * PAD };
}

function bounds(boxes) {
  const b = { minX: Infinity, maxX: -Infinity, minZ: Infinity, maxZ: -Infinity, maxY: 0 };
  for (const x of boxes) {
    b.minX = Math.min(b.minX, x.x - x.w / 2); b.maxX = Math.max(b.maxX, x.x + x.w / 2);
    b.minZ = Math.min(b.minZ, x.z - x.d / 2); b.maxZ = Math.max(b.maxZ, x.z + x.d / 2);
    b.maxY = Math.max(b.maxY, x.y + x.h);
  }
  return b;
}
