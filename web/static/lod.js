// What of the props is drawn, and how finely. The trees, bushes and lamps are
// thousands of instances, most of them behind the walker, beyond the horizon, or a
// pixel wide on a map zoomed out.
//
// So they are scattered by cell (CELL units a side), and before each frame every cell
// is judged against the camera: out of view, or beyond the walker's horizon, it is not
// drawn; on screen but small it is drawn with a coarse copy of each model (simplified),
// and coarser still where a prop is a pixel or two; otherwise in full. Each kind takes
// one draw a level of detail: the instances of the cells a frame wants are copied, cell
// by cell, into the front of that level's mesh, and only when what it wants has
// changed.
//
// Implements: REQ-PERF-010

import * as THREE from './vendor/three.module.min.js';

/** A cell's side, in world units. */
export const CELL = 4;
/**
 * The coarse levels of detail: a prop whose radius spans fewer than `px` pixels on
 * screen is drawn with its vertices clustered on a `grid`^3 lattice (simplified).
 */
export const LEVELS = [{ px: 6, grid: 4 }, { px: 1.5, grid: 2 }];

const coarse = new WeakMap();

/**
 * A coarse copy of a non-indexed or indexed geometry with a position and optionally a
 * color attribute (vertex clustering): the vertices in each cell of a grid^3 lattice
 * over its bounds merge into one at their mean, and the triangles that collapse go.
 * The same geometry and grid give the same copy, once.
 */
export function simplified(geometry, grid) {
  let copies = coarse.get(geometry);
  if (!copies) coarse.set(geometry, copies = new Map());
  let out = copies.get(grid);
  if (out) return out;
  const source = geometry.index ? geometry.toNonIndexed() : geometry;
  const position = source.getAttribute('position'), color = source.getAttribute('color');
  source.computeBoundingBox();
  const { min, max } = source.boundingBox;
  const size = new THREE.Vector3().subVectors(max, min).addScalar(1e-6);
  const cluster = new Map(); // lattice cell -> [sum x, y, z, r, g, b, n]
  const cellOf = new Int32Array(position.count);
  for (let i = 0; i < position.count; i++) {
    const cx = Math.min(grid - 1, Math.floor((position.getX(i) - min.x) / size.x * grid));
    const cy = Math.min(grid - 1, Math.floor((position.getY(i) - min.y) / size.y * grid));
    const cz = Math.min(grid - 1, Math.floor((position.getZ(i) - min.z) / size.z * grid));
    const key = (cx * grid + cy) * grid + cz;
    cellOf[i] = key;
    let sum = cluster.get(key);
    if (!sum) cluster.set(key, sum = [0, 0, 0, 0, 0, 0, 0]);
    sum[0] += position.getX(i); sum[1] += position.getY(i); sum[2] += position.getZ(i);
    if (color) { sum[3] += color.getX(i); sum[4] += color.getY(i); sum[5] += color.getZ(i); }
    sum[6]++;
  }
  const positions = [], colors = [];
  for (let i = 0; i + 2 < position.count; i += 3) {
    const a = cellOf[i], b = cellOf[i + 1], c = cellOf[i + 2];
    if (a === b || b === c || a === c) continue;
    for (const key of [a, b, c]) {
      const sum = cluster.get(key), n = sum[6];
      positions.push(sum[0] / n, sum[1] / n, sum[2] / n);
      if (color) colors.push(sum[3] / n, sum[4] / n, sum[5] / n);
    }
  }
  out = new THREE.BufferGeometry();
  out.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  if (color) out.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  out.computeVertexNormals();
  copies.set(grid, out);
  return out;
}

const NONE = 255;
// The level of detail for a prop whose radius spans `pixels`: 0 is the full model.
const levelOf = pixels => {
  let level = 0;
  while (level < LEVELS.length && pixels < LEVELS[level].px) level++;
  return level;
};

const cellKey = (x, z) => `${Math.floor(x / CELL)},${Math.floor(z / CELL)}`;

/**
 * One kind of prop - a species' crowns, the lamp posts - as an InstancedMesh a level of
 * detail on one material, in `meshes`: the full model, then LEVELS'. items: [{x, z, y}], placed by
 * place(item, matrix) and colored by tint(item, color) when given. `cells` is the
 * table the kinds of one map share (a Map), filled with the cells they stand in.
 */
export class Scatter {
  constructor(geometry, material, items, place, tint, cells) {
    geometry.computeBoundingSphere();
    this.radius = geometry.boundingSphere.radius;
    const order = items.map(it => [cellKey(it.x, it.z), it]).sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0));
    this.matrices = new Float32Array(order.length * 16);
    this.colors = tint ? new Float32Array(order.length * 3) : null;
    this.runs = []; // [cell, start, count], in instance order
    const m = new THREE.Matrix4(), c = new THREE.Color();
    order.forEach(([key, it], i) => {
      place(it, m).toArray(this.matrices, i * 16);
      if (tint) tint(it, c).toArray(this.colors, i * 3);
      const last = this.runs[this.runs.length - 1];
      if (last?.[0].key === key) last[2]++;
      else {
        let cell = cells.get(key);
        if (!cell) cells.set(key, cell = { key, x: 0, z: 0, top: -Infinity, n: 0, hidden: false, pixels: Infinity });
        this.runs.push([cell, i, 1]);
      }
      const cell = this.runs[this.runs.length - 1][0];
      cell.x += it.x; cell.z += it.z; cell.n++;
      cell.top = Math.max(cell.top, it.y);
    });
    const mesh = g => {
      const out = new THREE.InstancedMesh(g, material, Math.max(1, order.length));
      if (tint) out.instanceColor = new THREE.InstancedBufferAttribute(new Float32Array(Math.max(1, order.length) * 3), 3);
      out.frustumCulled = false; // bent in walk mode; the cells are culled instead
      out.count = 0;
      return out;
    };
    this.meshes = [mesh(geometry), ...LEVELS.map(level => mesh(simplified(geometry, level.grid)))];
    for (const coarser of this.meshes.slice(1)) coarser.userData.coarse = true;
    this.wanted = new Uint8Array(this.runs.length).fill(NONE); // as the meshes start: empty
    this.update();
  }

  /** Copies the instances of the cells now wanted into their levels' meshes, if that changed. */
  update() {
    let changed = false;
    this.runs.forEach(([cell], i) => {
      const want = cell.hidden ? NONE : levelOf(this.radius * cell.pixels);
      if (want !== this.wanted[i]) { this.wanted[i] = want; changed = true; }
    });
    if (!changed) return;
    const at = this.meshes.map(() => 0);
    this.runs.forEach(([, start, count], i) => {
      const level = this.wanted[i];
      if (level === NONE) return;
      const mesh = this.meshes[level], k = at[level];
      mesh.instanceMatrix.array.set(this.matrices.subarray(start * 16, (start + count) * 16), k * 16);
      if (this.colors) mesh.instanceColor.array.set(this.colors.subarray(start * 3, (start + count) * 3), k * 3);
      at[level] += count;
    });
    this.meshes.forEach((mesh, level) => {
      mesh.count = at[level];
      mesh.instanceMatrix.needsUpdate = true;
      if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
    });
  }
}

const _frustum = new THREE.Frustum(), _matrix = new THREE.Matrix4(), _sphere = new THREE.Sphere(), _v = new THREE.Vector3();
// How much taller than the ground under it a prop can stand, and how far past its
// center a cell's props can reach: what a cell's bounds are drawn from.
const PROP_H = 1, CELL_R = CELL * Math.SQRT1_2 + 0.5;

/**
 * Judges every cell for a frame seen by `camera`: `hidden`, and `pixels`, how many
 * pixels a unit at the cell's nearest spans on a screen `height` pixels tall. On the
 * map (bend null) a cell is hidden out of view; walking, bend(v) is where the map is
 * drawn (MapScene.bend), `center` the planet's middle and `radius` its size, and a
 * cell is also hidden past the walker's horizon or the fog's end (`far`).
 */
function judge(cells, camera, height, walk = null) {
  camera.updateMatrixWorld();
  _frustum.setFromProjectionMatrix(_matrix.multiplyMatrices(camera.projectionMatrix, camera.matrixWorldInverse));
  const focal = camera.isPerspectiveCamera ? height / 2 / Math.tan(THREE.MathUtils.degToRad(camera.fov) / 2) : 0;
  const eyeHeight = walk ? Math.max(0, camera.position.distanceTo(walk.center) - walk.radius) : 0;
  const R = walk?.radius ?? 0;
  const horizon = h => Math.sqrt(Math.max(0, 2 * R * h + h * h));
  for (const cell of cells.values()) {
    const x = cell.x / cell.n, z = cell.z / cell.n;
    if (!walk) {
      _sphere.set(_v.set(x, cell.top, z), CELL_R + PROP_H);
      cell.hidden = !_frustum.intersectsSphere(_sphere);
      cell.pixels = camera.zoom;
      continue;
    }
    const along = Math.hypot(x - walk.at.x, z - walk.at.z) - CELL_R;
    const reach = Math.min(walk.far, horizon(eyeHeight) + horizon(Math.max(0, cell.top + PROP_H)));
    _sphere.set(walk.bend(_v.set(x, cell.top, z)), CELL_R + PROP_H);
    cell.hidden = along > reach || !_frustum.intersectsSphere(_sphere);
    cell.pixels = focal / Math.max(0.05, camera.position.distanceTo(_sphere.center) - _sphere.radius);
  }
}

/** Brings a props group (city.js makeProps) up to date with the frame `judge` is given. */
export function cull(group, camera, height, walk = null) {
  const lod = group.userData.lod;
  if (!lod) return;
  judge(lod.cells, camera, height, walk);
  for (const scatter of lod.scatters) scatter.update();
}
