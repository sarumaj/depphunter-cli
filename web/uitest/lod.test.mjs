// What of the props a frame draws, and how finely (lod.js).

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const { CELL, LEVELS, Scatter, simplified, cull } = await import('../static/lod.js');

const place = (it, m) => m.makeTranslation(it.x, it.y, it.z);
const crown = new THREE.IcosahedronGeometry(0.2, 3);

// Two kinds on one table of cells: `near` stands at the origin, `far` a long way off.
function scattered() {
  const cells = new Map();
  const items = [{ x: 0.5, z: 0.5, y: 0 }, { x: 1, z: 1, y: 0 }, { x: 80.5, z: 0.5, y: 0 }];
  const crowns = new Scatter(crown, new THREE.MeshBasicMaterial(), items, place, null, cells);
  const group = new THREE.Group();
  group.add(...crowns.meshes);
  group.userData.lod = { cells, scatters: [crowns] };
  return { group, crowns, cells };
}

const camera = (x, zoom) => {
  const c = new THREE.OrthographicCamera(-10, 10, 10, -10, -100, 100);
  c.position.set(x, 10, 0);
  c.lookAt(x, 0, 0);
  c.zoom = zoom;
  c.updateProjectionMatrix();
  return c;
};
const counts = scatter => scatter.meshes.map(m => m.count);

describe('props culled and drawn coarse', () => {
  // Verifies: REQ-PERF-010
  it('simplifies a model to fewer triangles, coarser at each level', () => {
    const full = crown.toNonIndexed().getAttribute('position').count;
    const sizes = LEVELS.map(level => simplified(crown, level.grid).getAttribute('position').count);
    assert.ok(sizes[0] > 0 && sizes[0] < full, `level 1: ${sizes[0]} of ${full} vertices`);
    assert.ok(sizes[1] > 0 && sizes[1] < sizes[0], `level 2: ${sizes[1]} of ${sizes[0]}`);
    assert.equal(simplified(crown, LEVELS[0].grid), simplified(crown, LEVELS[0].grid), 'built once');
  });

  // Verifies: REQ-PERF-010
  it('draws everything in full until a frame is judged', () => {
    const { crowns } = scattered();
    assert.deepEqual(counts(crowns), [3, 0, 0]);
  });

  // Verifies: REQ-PERF-010
  it('leaves out the cells a map camera does not see', () => {
    const { group, crowns, cells } = scattered();
    assert.ok(cells.size >= 2 && 80 / CELL > 2, 'the two groups share a cell');
    cull(group, camera(0, 100), 800);
    assert.deepEqual(counts(crowns), [2, 0, 0]);
    cull(group, camera(80, 100), 800);
    assert.deepEqual(counts(crowns), [1, 0, 0]);
  });

  // Verifies: REQ-PERF-010
  it('draws the props coarse as they shrink on screen', () => {
    const { group, crowns } = scattered();
    const pixels = zoom => 0.2 * zoom;
    const zoomFor = px => px / 0.2;
    cull(group, camera(0, zoomFor(LEVELS[0].px) * 1.5), 800);
    assert.deepEqual(counts(crowns), [2, 0, 0], 'full');
    cull(group, camera(0, zoomFor(LEVELS[0].px) * 0.9), 800);
    assert.deepEqual(counts(crowns), [0, 2, 0], `coarse at ${pixels(zoomFor(LEVELS[0].px) * 0.9)} px`);
    cull(group, camera(0, zoomFor(LEVELS[1].px) * 0.9), 800);
    assert.deepEqual(counts(crowns), [0, 0, 2], 'coarsest');
  });

  // Verifies: REQ-PERF-010
  it('leaves out the cells past the walker\'s horizon', () => {
    const { group, crowns } = scattered();
    const eye = new THREE.PerspectiveCamera(70, 1, 0.02, 3000);
    eye.position.set(0, 0.5, 0);
    eye.lookAt(40, 0.5, 0);
    eye.updateProjectionMatrix();
    const walk = { bend: v => v, center: new THREE.Vector3(0, -40, 0), radius: 40, at: new THREE.Vector3(), far: Infinity };
    cull(group, eye, 800, walk);
    assert.equal(crowns.meshes.reduce((n, m) => n + m.count, 0), 2, 'the near two drawn, the one 80 units off not');
  });
});
