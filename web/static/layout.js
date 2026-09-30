// Implements: REQ-DIST-017

import { bulk } from './model.js';
import { FACADE, STORY } from './buildings.js';

// Archipelago layout: the repository is a mainland of nested terraces, each external
// ecosystem an island in rings around it. Positions derive only from the hierarchy and the
// expansion state, so the same repository always produces the same map.

// A building's footprint grows with its facade (buildings.js FACADE), so a face
// holds as many windows as it was laid out with; its height is data and does not.
const FILE = FACADE;
const GAP = 0.35;        // between siblings
const PAD = 0.55;        // inside a terrace
const TERRACE = 0.28;    // terrace thickness
const MAX_H = 10;        // tallest building
const SYM = 0.42, SYM_GAP = 0.12;
const LAND_MARGIN = 1.2, LAND_H = 0.45, ISLAND_GAP = 4;

// Implements: REQ-MAP-038
export const SCALES = {
  linear: t => t,
  sqrt: t => Math.sqrt(t),
  log: t => Math.log1p(t * 1000) / Math.log1p(1000),
};

/**
 * @returns {{boxes: Box[], byNode: Map<string, Box>, bounds}}
 * Heights use the unfiltered maximum so filtering never rescales what stays visible.
 * Box: {node, kind: 'land'|'terrace'|'district'|'building'|'symbol'|'package', x, z (center), y, w, d, h}
 * Implements: REQ-MAP-010
 */
export function layout(model, state) {
  const { visible, counts } = state.vis;
  const scale = SCALES[state.heightScale] || SCALES.sqrt;
  const maxLoc = maxFileLoc(model.root);
  // At least a story: a door in a building shorter than itself is no door.
  const height = loc => STORY * FACADE + scale(Math.min(1, (loc || 0) / maxLoc)) * MAX_H;
  const maxImporters = model.ecosystems.flatMap(e => e.children).reduce((a, p) => Math.max(a, p.importers), 1);
  const packageHeight = n => 0.3 + scale(n.importers / maxImporters) * MAX_H * 0.5;

  const boxes = [];
  const sizes = new Map();

  const expanded = n => state.expanded.has(n.id);
  const symbols = n => n.children.filter(c => c.kind === 'symbol');

  // Implements: REQ-MAP-003
  function measure(n) {
    let s;
    if (n.kind === 'package') {
      s = { w: FILE, d: FILE };
    } else if (n.kind === 'file') {
      const symbols_ = symbols(n);
      if (expanded(n) && symbols_.length) {
        const columns = Math.ceil(Math.sqrt(symbols_.length));
        const rows = Math.ceil(symbols_.length / columns);
        s = { w: columns * (SYM + SYM_GAP) - SYM_GAP + 2 * SYM_GAP, d: rows * (SYM + SYM_GAP) + SYM_GAP, cols: columns };
      } else {
        s = { w: FILE, d: FILE };
      }
    } else if (n.kind === 'dir' && !expanded(n)) {
      const side = Math.max(1.4 * FILE, Math.sqrt(counts.get(n.id).fileCount) * (FILE + GAP) * 0.75);
      s = { w: side, d: side };
    } else { // expanded dir or ecosystem
      const items = n.children.filter(c => c.kind !== 'symbol' && visible(c)).map(c => ({ n: c, ...measure(c) }));
      s = { ...shelf(items), items };
    }
    sizes.set(n.id, s);
    return s;
  }

  // Implements: REQ-MAP-002, REQ-MAP-004, REQ-MAP-005, REQ-MAP-006, REQ-MAP-008, REQ-MAP-058
  function place(n, x0, z0, y) {
    const s = sizes.get(n.id);
    const cx = x0 + s.w / 2, cz = z0 + s.d / 2;
    if (n.kind === 'file') {
      if (s.cols) {
        boxes.push({ node: n, kind: 'terrace', x: cx, z: cz, y, w: s.w, d: s.d, h: TERRACE * 0.6 });
        const top = y + TERRACE * 0.6;
        symbols(n).forEach((symbol, i) => {
          const c = i % s.cols, r = Math.floor(i / s.cols);
          boxes.push({
            node: symbol, kind: 'symbol',
            x: x0 + SYM_GAP + c * (SYM + SYM_GAP) + SYM / 2, z: z0 + SYM_GAP + r * (SYM + SYM_GAP) + SYM / 2,
            y: top, w: SYM, d: SYM, h: symbolHeight(symbol),
          });
        });
      } else {
        boxes.push({ node: n, kind: 'building', x: cx, z: cz, y, w: s.w, d: s.d, h: height(bulk(n)) });
      }
      return;
    }
    if (n.kind === 'package') {
      boxes.push({ node: n, kind: 'package', x: cx, z: cz, y, w: s.w, d: s.d, h: packageHeight(n) });
      return;
    }
    if (n.kind === 'dir' && !expanded(n)) {
      const c = counts.get(n.id);
      boxes.push({ node: n, kind: 'district', x: cx, z: cz, y, w: s.w, d: s.d, h: height(c.totalBulk / Math.max(1, c.fileCount)) });
      return;
    }
    boxes.push({ node: n, kind: 'terrace', x: cx, z: cz, y, w: s.w, d: s.d, h: TERRACE });
    for (const it of s.items) place(it.n, x0 + it.x, z0 + it.z, y + TERRACE);
  }

  // Mainland.
  // Implements: REQ-MAP-001
  const main = measure(model.root);
  const land = (x0, z0, w, d, node) => boxes.push({
    node, kind: 'land', x: x0 + w / 2, z: z0 + d / 2, y: -LAND_H, w: w + 2 * LAND_MARGIN, d: d + 2 * LAND_MARGIN, h: LAND_H,
  });
  land(0, 0, main.w, main.d, model.root);
  place(model.root, 0, 0, 0);

  // Islands ring the mainland, the most imported nearest.
  // Implements: REQ-MAP-007, REQ-MAP-011
  const islands = model.ecosystems
    .filter(e => visible(e) && e.children.length)
    .map(e => ({ e, s: measure(e), uses: e.children.reduce((a, p) => a + (p.importers || 0), 0) }))
    .sort((a, b) => b.uses - a.uses || a.e.name.localeCompare(b.e.name));
  const shore = { x0: -LAND_MARGIN, z0: -LAND_MARGIN, x1: main.w + LAND_MARGIN, z1: main.d + LAND_MARGIN };
  ringIslands(islands, shore, (it, ox, oz) => {
    land(ox + LAND_MARGIN, oz + LAND_MARGIN, it.s.w, it.s.d, it.e);
    place(it.e, ox + LAND_MARGIN, oz + LAND_MARGIN, 0);
  });

  // Land sits under a terrace of the same node; the terrace is the node's representative.
  const byNode = new Map();
  boxes.forEach((b, i) => {
    b.i = i;
    if (b.kind !== 'land') byNode.set(b.node.id, b);
  });
  return { boxes, byNode, bounds: bounds(boxes) };
}

/**
 * Places islands (in order) in rings around a rectangle: each goes to the side of the
 * current ring whose row would be least full, rows are centered on their side and
 * touch the ring's shore across the gap. When no side has room, the next ring starts
 * outside everything placed so far. Rows on the east and west stay within their
 * side's length, so they never reach the corners; only a northern or southern row
 * may outgrow its side, for an island wider than the whole side.
 * put(item, x0, z0) receives the island's outer corner, land margin included.
 * Implements: REQ-MAP-011
 */
function ringIslands(items, rect, put) {
  const G = ISLAND_GAP;
  let r = { ...rect };
  let pending = items;
  while (pending.length) {
    const sides = ['n', 'e', 's', 'w'].map(side => {
      const northSouth = side === 'n' || side === 's';
      return { side, ns: northSouth, cap: northSouth ? r.x1 - r.x0 : r.z1 - r.z0, used: 0, depth: 0, row: [] };
    });
    const rest = [];
    for (const it of pending) {
      const fw = it.s.w + 2 * LAND_MARGIN, fd = it.s.d + 2 * LAND_MARGIN;
      let best = null;
      for (const sideEntry of sides) {
        const along = sideEntry.ns ? fw : fd, away = sideEntry.ns ? fd : fw;
        const extent = sideEntry.used + (sideEntry.used ? G : 0) + along;
        if (extent > sideEntry.cap && (sideEntry.used || !sideEntry.ns)) continue;
        const fill = extent / sideEntry.cap;
        if (!best || fill < best.fill) best = { sd: sideEntry, along, away, len: extent, fill };
      }
      if (!best) { rest.push(it); continue; }
      best.sd.row.push({ it, along: best.along, away: best.away });
      best.sd.used = best.len;
      best.sd.depth = Math.max(best.sd.depth, best.away);
    }
    if (rest.length === pending.length) { // nothing fits even an empty ring: cannot happen, but never loop
      rest.forEach(it => put(it, r.x1 + G, r.z0));
      return;
    }
    const next = { ...r };
    for (const sideEntry of sides) {
      if (!sideEntry.row.length) continue;
      let t = (sideEntry.ns ? (r.x0 + r.x1) : (r.z0 + r.z1)) / 2 - sideEntry.used / 2;
      for (const { it, along, away } of sideEntry.row) {
        let x, z;
        if (sideEntry.side === 'n') { x = t; z = r.z0 - G - away; }
        else if (sideEntry.side === 's') { x = t; z = r.z1 + G; }
        else if (sideEntry.side === 'e') { x = r.x1 + G; z = t; }
        else { x = r.x0 - G - away; z = t; }
        put(it, x, z);
        next.x0 = Math.min(next.x0, x); next.z0 = Math.min(next.z0, z);
        next.x1 = Math.max(next.x1, x + (sideEntry.ns ? along : away)); next.z1 = Math.max(next.z1, z + (sideEntry.ns ? away : along));
        t += along + G;
      }
    }
    r = next;
    pending = rest;
  }
}

/**
 * The box that stands for `n` in a layout's byNode: its own, or its nearest drawn
 * ancestor's; null when nothing above it is drawn either.
 * Implements: REQ-MAP-009
 */
export function representative(byNode, n) {
  for (let p = n; p; p = p.parentNode) {
    const b = byNode.get(p.id);
    if (b) return b;
  }
  return null;
}

function symbolHeight(symbol) {
  switch (symbol.symbolKind) {
    case 'type': case 'class': case 'interface': case 'component': return 1.1;
    case 'func': case 'method': case 'function': return 0.7;
    default: return 0.35;
  }
}

/**
 * The largest file in the tree, for the height scale; every relayout asks again.
 *
 * Counted in lines that were really counted, so that the files nobody could count -
 * the binaries, and anything over --max-file-size - are measured against the source
 * around them rather than setting the scale for it. A 4 MB blob among 500-line files
 * would otherwise flatten the whole city to make room for itself; instead it runs up
 * against the Math.min(1, ...) in height() and tops out level with the longest file.
 * Implements: REQ-MAP-061
 */
function maxFileLoc(n, max = 1) {
  if (n.kind === 'file') return Math.max(max, n.loc || 0);
  for (const c of n.children) if (c.kind === 'dir' || c.kind === 'file') max = maxFileLoc(c, max);
  return max;
}

// Packs a terrace's children as densely as a near-square allows: bottom-left onto a
// skyline (packAt), in two orders and at a few strip widths around the square's,
// keeping whichever takes the least area. Each box carries its gap; positions are
// offset by the terrace padding. The sorts are stable and children arrive in name
// order, so the same tree always packs the same way.
// Implements: REQ-MAP-043, REQ-MAP-010
function shelf(items) {
  if (!items.length) return { w: FILE + 2 * PAD, d: FILE + 2 * PAD };
  const boxes = items.map(it => ({ w: it.w + GAP, h: it.d + GAP, it }));
  const area = boxes.reduce((a, b) => a + b.w * b.h, 0);
  const widest = Math.max(...boxes.map(b => b.w));
  let best = null;
  for (const order of ORDERS) {
    const sorted = [...boxes].sort(order);
    for (const k of WIDTHS) {
      const packed = packAt(sorted, Math.max(widest, Math.sqrt(area) * k));
      // Near-square: a long strip packs tighter, and makes a terrace nobody can take in.
      const long = Math.max(packed.w / packed.h, packed.h / packed.w);
      const score = packed.w * packed.h * (long > 2 ? long / 2 : 1);
      if (!best || score < best.score - 1e-9) best = { ...packed, sorted, score };
    }
  }
  best.at.forEach((p, i) => {
    best.sorted[i].it.x = PAD + p.x;
    best.sorted[i].it.z = PAD + p.z;
  });
  return { w: Math.max(best.w - GAP, FILE) + 2 * PAD, d: Math.max(best.h - GAP, FILE) + 2 * PAD };
}

// The strip widths shelf tries, as multiples of the side of a square of the boxes' area,
// and the orders it places them in: tallest first, and largest first.
const WIDTHS = [0.8, 0.9, 1, 1.1, 1.2, 1.35, 1.5];
const ORDERS = [
  (a, b) => b.h - a.h || b.w - a.w,
  (a, b) => b.w * b.h - a.w * a.h,
];

// Places boxes, in order, each where its top comes lowest on a skyline no wider than
// `width` (the leftmost such place); returns their corners and the space they take.
function packAt(boxes, width) {
  let sky = [{ x: 0, y: 0, w: width }];
  const at = [];
  let w = 0, h = 0;
  for (const box of boxes) {
    let place = null;
    for (let i = 0; i < sky.length && sky[i].x + box.w <= width + 1e-9; i++) {
      let y = 0;
      for (let j = i, end = sky[i].x + box.w; j < sky.length && sky[j].x < end - 1e-9; j++) y = Math.max(y, sky[j].y);
      if (!place || y < place.y - 1e-9) place = { x: sky[i].x, y };
    }
    at.push({ x: place.x, z: place.y });
    w = Math.max(w, place.x + box.w);
    h = Math.max(h, place.y + box.h);
    // The box's top replaces the skyline under it.
    const x0 = place.x, x1 = place.x + box.w, next = [];
    for (const s of sky) {
      if (s.x + s.w <= x0 + 1e-9 || s.x >= x1 - 1e-9) { next.push(s); continue; }
      if (s.x < x0) next.push({ x: s.x, y: s.y, w: x0 - s.x });
      if (s.x + s.w > x1) next.push({ x: x1, y: s.y, w: s.x + s.w - x1 });
    }
    next.push({ x: x0, y: place.y + box.h, w: box.w });
    next.sort((a, b) => a.x - b.x);
    sky = next.reduce((out, s) => {
      const last = out[out.length - 1];
      if (last && Math.abs(last.y - s.y) < 1e-9) last.w += s.w; else out.push({ ...s });
      return out;
    }, []);
  }
  return { at, w, h };
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
