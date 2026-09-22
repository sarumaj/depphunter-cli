// The dependency graph as roads on the ground.
//
// Selecting a module has always drawn its dependencies as arcs - tubes lifted over
// the map with an arrow at the far end. An arc says which two buildings are joined
// and nothing else; it is a line on a chart laid over a city. This lays the same
// edges down as streets: from the selected building, out through the gaps between
// the blocks, to every building it depends on and every building that depends on it,
// with chevrons along the way saying which direction the dependency runs in. The
// arcs stay, as the indicator that reads from across the map.
//
// The route is found rather than drawn. The map is cut into a grid, every standing
// box is an obstacle and everything a box stands on is ground; one breadth-first
// sweep out from the selected building reaches the whole map, and every other
// building then walks back down it. One sweep serves the lot because every edge
// drawn has the selection at one end of it - which is the only reason this is cheap
// enough to do on a click.
//
// Roads keep to the axes and prefer to carry straight on, so what comes out is a
// street with corners rather than a staircase of pixels. They lie on whatever they
// cross, so a road that climbs onto a terrace steps up with it - and that includes
// the bridges, which are the way across the water and are laid into the grid as the
// ground they are. Every island is on the end of one (city.js builds a spanning tree
// of them), so a road to an external package goes over a bridge like everything
// else. Open water is left passable at a price, for the shores no bridge was built
// between - dear enough that a road walks to the crossing rather than swimming the
// bay, which is what would make the network look like it was guessing.

import * as THREE from './vendor/three.module.min.js';
import { bridgesFor, bridgeBounds, bridgeHeight } from './city.js';

const CELL = 0.12;        // the grid routes are found on: a street is three cells wide
const MAX_CELLS = 700000; // a very large map gets a coarser grid rather than a slow one
const CLEAR = 0.04;       // how far a road keeps off a wall
const WIDTH = 0.2;        // how wide a road is
const CASE = 0.05;        // the edging on either side of it, in the opposite tone
const LIFT = 0.015;       // how far above the ground it lies, so it does not fight it
const CHEVRON = 0.8;      // one direction mark every this far along
const CHEV_LEN = 0.3;     // ... this long, and as wide as the road
// How long a piece of road may be before it is cut in two. Walk mode bends the world
// in the vertex shader, which bends the corners of a quad and nothing in between: a
// long straight run is a chord across the curve and sinks under the ground halfway
// along it. Short pieces follow the bend, and on the flat map they cost nothing but
// vertices.
const SEG = 0.5;
const MAX_ROUTES = 60;    // beyond this the map is a plate of spaghetti anyway
// What a cell of open water costs against a cell of street. The bridges carry the
// crossings, so swimming is the last resort rather than a shortcut: at this price a
// road will walk five units out of its way to reach a bridge before it will leave
// the shore for one cell. The sweep holds one list per outstanding cost, so this is
// also how many of those there are - cheap, but not free.
const SEA = 60;

// The kinds that stand on the ground, and so cannot be driven through. A terrace is
// not one of them: it is the ground its children stand on, and a road crosses it.
const SOLID = new Set(['building', 'symbol', 'package', 'district']);

export class Routes {
  constructor(scene) {
    this.scene = scene;
    this.group = new THREE.Group();
    this.materials = new Map();
    this.grid = null;
    scene.scene.add(this.group);
  }

  /**
   * Reads the layout: what is solid, and how high the ground is under every cell.
   * Called with the boxes, once per layout, before any route is asked for.
   */
  setLayout(boxes) {
    this.clear();
    this.grid = boxes?.length ? gridOf(boxes) : null;
  }

  /**
   * Lays down a road for every edge. arcs are what scene.setArcs draws in the air -
   * { from, to, color } - and `hub` is the box they all have an end at, which is what
   * the single sweep starts from.
   */
  set(arcs, hub) {
    this.clear();
    const g = this.grid;
    if (!g || !hub || !arcs?.length) return this.scene.requestRender();
    const dist = sweep(g, hub);
    const byColor = new Map();
    for (const a of arcs.slice(0, MAX_ROUTES)) {
      const far = a.from === hub ? a.to : a.from;
      if (far === hub) continue;
      const path = walkBack(g, dist, far);
      if (!path) continue;
      // Downhill from the far building is the way back to the selection, so a path
      // out of the sweep always runs the wrong way for an edge that leads away.
      if (a.from === hub) path.reverse();
      const dense = densify(path);
      const bucket = byColor.get(a.color) || { edging: [], road: [], chev: [] };
      ribbon(dense, bucket.edging, WIDTH / 2 + CASE, 0);
      ribbon(dense, bucket.road, WIDTH / 2, 0.004);
      chevrons(dense, bucket.chev);
      byColor.set(a.color, bucket);
    }
    // The edge colors are a near-black and a grey, which is what the arcs want to be
    // against the sky and exactly what a road does not want to be against asphalt.
    // So the road is edged and marked in the opposite tone: what carries across the
    // map is the pale casing, and the color inside it still says which way the
    // dependency runs.
    for (const [color, { edging, road, chev }] of byColor) {
      this.add(edging, contrast(color), 0.85, 0);
      this.add(road, color, 0.9, 1);
      this.add(chev, contrast(color), 0.95, 2);
    }
    this.scene.requestRender();
  }

  add(positions, color, opacity, layer) {
    if (!positions.length) return;
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    const mesh = new THREE.Mesh(geo, this.material(color, opacity));
    mesh.frustumCulled = false; // walk mode bends it away from its own bounds
    // Edging, then the road over it, then the marks on top. These lie within a
    // couple of millimeters of each other, and nothing here writes depth, so the
    // order they are drawn in is the only thing that decides what is seen. Left to
    // the renderer's own sort they are three surfaces at the same distance and the
    // order is whatever it happens to be.
    mesh.renderOrder = 3 + layer;
    this.group.add(mesh);
  }

  // Two materials per color - the road and its chevrons - however many roads there
  // are. They are bendable, so a road follows the planet in walk mode.
  material(color, opacity) {
    const key = `${color}:${opacity}`;
    let m = this.materials.get(key);
    if (!m) {
      // A road is a decal: it lies on a surface rather than above it, and lifting it
      // far enough to win the depth test outright would have it floating over a
      // terrace. The offset settles that at the depth test instead, which is what it
      // is for - in walk mode the ground is bent in the vertex shader and a
      // millimeter of clearance on the flat map is not a millimeter out there.
      m = this.scene.bendable(new THREE.MeshBasicMaterial({
        color, transparent: true, opacity, side: THREE.DoubleSide, depthWrite: false,
        polygonOffset: true, polygonOffsetFactor: -8, polygonOffsetUnits: -12,
      }));
      this.materials.set(key, m);
    }
    return m;
  }

  clear() {
    for (const mesh of this.group.children) mesh.geometry.dispose();
    this.group.clear();
    for (const m of this.materials.values()) m.dispose();
    this.materials.clear();
  }

  dispose() {
    this.clear();
    this.scene.scene.remove(this.group);
  }
}

// White over a dark road, near-black over a light one.
function contrast(color) {
  const c = new THREE.Color(color);
  return c.r * 0.3 + c.g * 0.59 + c.b * 0.11 > 0.42 ? '#141414' : '#f2f2f2';
}

/**
 * The grid: which cells are solid, how high the ground is in each, and where every
 * box sits on it. Cells are tested by their centres rather than by whether a
 * footprint touches them at all, because a street is only three cells wide and
 * rounding both its sides outwards closes it.
 */
function gridOf(boxes) {
  let minX = Infinity, maxX = -Infinity, minZ = Infinity, maxZ = -Infinity;
  for (const b of boxes) {
    minX = Math.min(minX, b.x - b.w / 2); maxX = Math.max(maxX, b.x + b.w / 2);
    minZ = Math.min(minZ, b.z - b.d / 2); maxZ = Math.max(maxZ, b.z + b.d / 2);
  }
  const area = Math.max(1, (maxX - minX) * (maxZ - minZ));
  const per = Math.min(1 / CELL, Math.sqrt(MAX_CELLS / area));
  const W = Math.ceil((maxX - minX) * per) + 2, H = Math.ceil((maxZ - minZ) * per) + 2;
  const g = {
    minX, minZ, per, W, H,
    solid: new Uint8Array(W * H),
    dry: new Uint8Array(W * H), // 1 where there is something to stand on
    top: new Float32Array(W * H).fill(deck(boxes)),
    dist: new Int32Array(W * H), // what a sweep writes, reused by every sweep
  };

  // The ground first: a cell takes the highest thing it can be stood on. What no box
  // covers is water, and stays at the height the causeways run at.
  for (const b of boxes) {
    if (b.kind !== 'land' && b.kind !== 'terrace') continue;
    const y = b.y + b.h;
    each(g, b, 0, (i, j) => {
      const k = j * W + i;
      g.dry[k] = 1;
      if (y > g.top[k]) g.top[k] = y;
    });
  }
  // Then the bridges, which are ground over water and the only ground there is
  // between two shores. They go down after the land - where a deck runs over its own
  // shore the two agree on the height anyway - and before the solids, so a building
  // standing at a landing still closes the road off.
  //
  // Walked along rather than filled in by cell centres, the way a box is: a deck is
  // under a unit wide, and on a map coarse enough to need a bigger cell it would fall
  // through the gaps between the centres and leave the roads swimming beside a bridge
  // that was right there. The width is taken in by the road's own, so what is marked
  // is where a road may lie rather than where the deck ends.
  for (const r of bridgesFor(boxes)) {
    const b = bridgeBounds(r);
    const half = (r.axis === 'x' ? b.z1 - b.z0 : b.x1 - b.x0) / 2;
    const usable = Math.max(0, half - (WIDTH / 2 + CASE));
    const step = 0.5 / per; // half a cell, so nothing between two samples is missed
    const alongs = Math.max(1, Math.ceil((r.to - r.from) / step));
    const across = Math.max(1, Math.ceil((2 * usable) / step));
    for (let a = 0; a <= alongs; a++) {
      const at = r.from + ((r.to - r.from) * a) / alongs;
      for (let c = 0; c <= across; c++) {
        const off = usable === 0 ? 0 : -usable + (2 * usable * c) / across;
        const x = r.axis === 'x' ? at : r.across + off;
        const z = r.axis === 'x' ? r.across + off : at;
        const i = col(g, x), j = row(g, z);
        if (i < 0 || i >= W || j < 0 || j >= H) continue;
        const k = j * W + i;
        g.dry[k] = 1;
        const h = bridgeHeight(r, x, z);
        if (h > g.top[k]) g.top[k] = h;
        if (usable === 0) break;
      }
    }
  }
  for (const b of boxes) {
    if (!SOLID.has(b.kind)) continue;
    each(g, b, CLEAR, (i, j) => { g.solid[j * W + i] = 1; });
  }
  return g;
}

// The height a causeway runs at: the top of the land, which is the shore it leaves.
function deck(boxes) {
  let y = 0;
  for (const b of boxes) if (b.kind === 'land') y = Math.max(y, b.y + b.h);
  return y;
}

// Every cell whose centre lies within a box, grown by `pad`.
function each(g, b, pad, fn) {
  const i0 = Math.max(0, first(g, b.x - b.w / 2 - pad - g.minX));
  const i1 = Math.min(g.W - 1, last(g, b.x + b.w / 2 + pad - g.minX));
  const j0 = Math.max(0, first(g, b.z - b.d / 2 - pad - g.minZ));
  const j1 = Math.min(g.H - 1, last(g, b.z + b.d / 2 + pad - g.minZ));
  for (let j = j0; j <= j1; j++) for (let i = i0; i <= i1; i++) fn(i, j);
}

// The first and last cells whose centres fall inside a span measured from the grid's
// own corner. A cell counts by its centre, not by being touched at all: a street is
// three cells wide, and rounding both its sides outwards closes it.
const first = (g, v) => Math.ceil(v * g.per - 0.5);
const last = (g, v) => Math.floor(v * g.per - 0.5);
const col = (g, x) => Math.floor((x - g.minX) * g.per - 0.5);
const row = (g, z) => Math.floor((z - g.minZ) * g.per - 0.5);
const cx = (g, i) => g.minX + (i + 0.5) / g.per;
const cz = (g, j) => g.minZ + (j + 0.5) / g.per;

/**
 * The cost of reaching every cell from the ground beside `hub`, over the cells that
 * are not solid. Four neighbors, not eight: a city's streets meet at right angles,
 * and a route allowed to cut corners comes out as a staircase.
 *
 * A step costs one on land and SEA over water, so this is a shortest-path sweep
 * rather than a plain breadth-first one - but with only two costs it needs no heap.
 * Cells are held in a ring of SEA + 1 lists, one per outstanding cost, and taken in
 * the order the costs come up; nothing is ever reached more cheaply than the list it
 * is taken from, which is the whole of the argument for doing it this way.
 */
function sweep(g, hub) {
  // The one buffer, kept with the grid: a map of any size is several megabytes of
  // it, and this runs on every click.
  const dist = g.dist;
  dist.fill(-1);
  const ring = [];
  for (let i = 0; i <= SEA; i++) ring.push([]);
  let waiting = 0;
  const relax = (k, d) => {
    if (g.solid[k] || (dist[k] >= 0 && dist[k] <= d)) return;
    dist[k] = d;
    ring[d % (SEA + 1)].push(k);
    waiting++;
  };
  for (const k of around(g, hub)) relax(k, 0);
  for (let d = 0; waiting > 0; d++) {
    const bucket = ring[d % (SEA + 1)];
    while (bucket.length) {
      const k = bucket.pop();
      waiting--;
      if (dist[k] !== d) continue; // reached more cheaply after it was put in
      const to = d + (g.dry[k] ? 1 : SEA);
      const i = k % g.W;
      if (i > 0) relax(k - 1, to);
      if (i < g.W - 1) relax(k + 1, to);
      if (k >= g.W) relax(k - g.W, to);
      if (k < g.W * (g.H - 1)) relax(k + g.W, to);
    }
  }
  return dist;
}

// The free cells lying against a box: where a road meets its door.
function around(g, b) {
  const out = [];
  const reach = Math.ceil(0.25 * g.per) + 1;
  const i0 = Math.max(0, col(g, b.x - b.w / 2) - reach), i1 = Math.min(g.W - 1, col(g, b.x + b.w / 2) + reach + 1);
  const j0 = Math.max(0, row(g, b.z - b.d / 2) - reach), j1 = Math.min(g.H - 1, row(g, b.z + b.d / 2) + reach + 1);
  for (let j = j0; j <= j1; j++) {
    for (let i = i0; i <= i1; i++) {
      const k = j * g.W + i;
      if (!g.solid[k]) out.push(k);
    }
  }
  return out;
}

/**
 * From the box back down the sweep to the selection: the nearest cell beside it that
 * the sweep reached, then downhill from there, carrying straight on wherever that is
 * still downhill. Returns the corners of the road, or null if nothing connects.
 */
function walkBack(g, dist, box) {
  let at = -1, best = Infinity;
  for (const k of around(g, box)) {
    if (dist[k] >= 0 && dist[k] < best) { best = dist[k]; at = k; }
  }
  if (at < 0) return null;
  const path = [at];
  let dir = 0;
  const steps = [1, -1, g.W, -g.W];
  for (let guard = 0; dist[at] > 0 && guard < dist.length; guard++) {
    let next = -1;
    // The direction it was already going, then the others: a road that can carry on
    // straight does, which is what turns a field of single steps into a street.
    for (const s of [dir, ...steps]) {
      if (!s) continue;
      const k = at + s;
      const wrapped = (s === 1 || s === -1) && Math.floor(k / g.W) !== Math.floor(at / g.W);
      if (wrapped || k < 0 || k >= dist.length || dist[k] < 0) continue;
      // What it cost to step from there to here is what that cell's own ground says.
      if (dist[k] + (g.dry[k] ? 1 : SEA) === dist[at]) { next = k; dir = s; break; }
    }
    if (next < 0) break;
    at = next;
    path.push(at);
  }
  return corners(g, path);
}

// The path as the points it turns at, in world space, on the ground it crosses.
function corners(g, path) {
  const out = [];
  const point = k => new THREE.Vector3(cx(g, k % g.W), g.top[k] + LIFT, cz(g, Math.floor(k / g.W)));
  for (let n = 0; n < path.length; n++) {
    const prev = path[n - 1], k = path[n], next = path[n + 1];
    // A corner, either end, or a step up or down: everything else is in a straight
    // line between two of those and does not need a point of its own.
    const turn = prev === undefined || next === undefined || k - prev !== next - k;
    if (turn || Math.abs(g.top[k] - g.top[prev]) > 1e-4) out.push(point(k));
  }
  return out.length > 1 ? out : null;
}

/**
 * The corners with a point every SEG along the runs between them. Everything laid on
 * the road - the road, its edging, the chevrons - is built from this one polyline, so
 * they all bend the same way and none of them sinks through another.
 */
function densify(path) {
  const out = [path[0]];
  for (let n = 1; n < path.length; n++) {
    const a = path[n - 1], b = path[n];
    const len = Math.hypot(b.x - a.x, b.z - a.z);
    const pieces = Math.max(1, Math.ceil(len / SEG));
    for (let i = 1; i <= pieces; i++) out.push(a.clone().lerp(b, i / pieces));
  }
  return out;
}

/**
 * The road: a quad between each pair of points, the two far ends pushed out by half
 * its width so a corner fills in rather than leaving a notch. The quads meet exactly
 * and never overlap, because they are drawn with transparency and an overlap is a
 * darker patch.
 */
function ribbon(path, out, h, lift) {
  for (let n = 1; n < path.length; n++) {
    const a = path[n - 1], b = path[n];
    const dx = b.x - a.x, dz = b.z - a.z;
    const len = Math.hypot(dx, dz);
    if (len < 1e-5) continue;
    const ux = dx / len, uz = dz / len;      // along
    const nx = -uz * h, nz = ux * h;         // across
    const back = n === 1 ? h : 0, on = n === path.length - 1 ? h : 0;
    const x0 = a.x - ux * back, z0 = a.z - uz * back, y0 = a.y + lift;
    const x1 = b.x + ux * on, z1 = b.z + uz * on, y1 = b.y + lift;
    quad(out,
      x0 + nx, y0, z0 + nz, x1 + nx, y1, z1 + nz,
      x1 - nx, y1, z1 - nz, x0 - nx, y0, z0 - nz);
  }
}

// The chevrons: flat arrow heads laid along the road, pointing the way the dependency
// runs. They are what makes a road a direction rather than a connection.
function chevrons(path, out) {
  const h = WIDTH / 2;
  let carry = CHEVRON * 0.5;
  for (let n = 1; n < path.length; n++) {
    const a = path[n - 1], b = path[n];
    const dx = b.x - a.x, dz = b.z - a.z;
    const len = Math.hypot(dx, dz);
    if (len < 1e-5) continue;
    const ux = dx / len, uz = dz / len;
    for (let t = carry; t < len; t += CHEVRON) {
      const x = a.x + ux * t, z = a.z + uz * t;
      const y = a.y + ((b.y - a.y) * t) / len;
      // Tip forward, two barbs back and out: one triangle, drawn over the road.
      out.push(
        x + ux * CHEV_LEN, y + 0.008, z + uz * CHEV_LEN,
        x - uz * h, y + 0.008, z + ux * h,
        x + uz * h, y + 0.008, z - ux * h,
      );
    }
    carry = ((carry - len) % CHEVRON + CHEVRON) % CHEVRON;
  }
}

function quad(out, ax, ay, az, bx, by, bz, cx_, cy, cz_, dx, dy, dz) {
  out.push(ax, ay, az, bx, by, bz, cx_, cy, cz_, ax, ay, az, cx_, cy, cz_, dx, dy, dz);
}
