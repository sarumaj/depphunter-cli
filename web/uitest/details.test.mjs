// The details that stand out of the city's buildings: where they go, that the height
// of a building is still the height of its file, and that they come and go with the
// camera and with dimming.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box } from './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const B = await import('../static/map/buildings.js');
const D = await import('../static/map/details.js');
const { Walker } = await import('../static/walk/walk.js');

// A district of files of every height, a unit and a bit apart as the layout packs
// them, and some packages; ids fixed so the types are too.
function district(count = 900) {
  const boxes = [];
  for (let i = 0; i < count; i++) {
    const h = 0.2 + (i % 97) / 96 * 10;
    boxes.push(box('building', (i % 30) * 1.35, Math.floor(i / 30) * 1.35, 1, 1, { y: 0.28, h, node: { id: `f:lib/file${i}.go`, kind: 'file' } }));
  }
  for (let i = 0; i < 60; i++) {
    boxes.push(box('package', 60 + (i % 6) * 1.35, Math.floor(i / 6) * 1.35, 1, 1, { h: 0.3 + (i % 9) * 0.6, node: { id: `p:package${i}`, kind: 'package' } }));
  }
  boxes.forEach((b, i) => { b.i = i; });
  return boxes;
}

// The world-space bounds of one detail: its kind's geometry through its matrix.
function bounds(detail) {
  const geometry = D.DETAIL_GEOMETRY[detail.kind];
  geometry.computeBoundingBox();
  return geometry.boundingBox.clone().applyMatrix4(new THREE.Matrix4().fromArray(detail.matrix));
}

const ROOF_GEAR = new Set(['tank', 'unit', 'antenna', 'rail']);

describe('building details', () => {
  // Verifies: REQ-CITY-033
  it('stay within the footprint, standing out of it by no more than the overhang', () => {
    let seen = 0;
    for (const b of district()) {
      for (const d of D.detailsOf(b)) {
        const r = bounds(d);
        const out = ROOF_GEAR.has(d.kind) ? 1e-6 : D.OVERHANG + 1e-6;
        assert.ok(r.min.x >= b.x - b.w / 2 - out && r.max.x <= b.x + b.w / 2 + out, `${d.kind} of ${b.node.id} x ${r.min.x}..${r.max.x}`);
        assert.ok(r.min.z >= b.z - b.d / 2 - out && r.max.z <= b.z + b.d / 2 + out, `${d.kind} of ${b.node.id} z ${r.min.z}..${r.max.z}`);
        assert.ok(r.min.y >= b.y - 1e-6, `${d.kind} of ${b.node.id} under its ground`);
        const ceiling = ROOF_GEAR.has(d.kind) ? b.h + D.GEAR_MAX : b.h;
        assert.ok(r.max.y <= b.y + ceiling + 1e-6, `${d.kind} of ${b.node.id} tops out at ${r.max.y - b.y} of ${b.h}`);
        seen++;
      }
    }
    assert.ok(seen > 1000, `only ${seen} details`);
  });

  // Verifies: REQ-CITY-033
  it('put balconies on the stories and the French windows the facade paints', () => {
    let balconies = 0;
    for (const b of district()) {
      const [type, variant] = B.buildParameters(b);
      for (const d of D.detailsOf(b).filter(d => d.kind === 'balcony')) {
        balconies++;
        assert.ok(type === B.TYPE.residential || (type === B.TYPE.mixed && variant < 0.5), B.ARCHETYPES[type]);
        const story = (d.matrix[13] - b.y) / (B.STORY * B.FACADE);
        assert.ok(Math.abs(story - Math.round(story)) < 1e-6 && story >= 1, `a balcony at story ${story}`);
        // Centered on a bay of the residential layout.
        const along = Math.abs(d.matrix[12] - b.x) < 1e-6 ? d.matrix[14] - b.z : d.matrix[12] - b.x;
        const faceW = b.w, bay = faceW / Math.floor(faceW / B.FACADE / D.bayWidth(B.TYPE.residential));
        const at = (Math.abs(along) + faceW / 2) / bay - 0.5;
        assert.ok(Math.abs(at - Math.round(at)) < 1e-6 || Math.abs(Math.abs(along) - faceW / 2) < 1e-6, `off its bay: ${at}`);
      }
    }
    assert.ok(balconies > 500, `${balconies} balconies`);
  });

  // Verifies: REQ-CITY-034
  it('keep the top of a setback tower at the height of its file', () => {
    let towers = 0;
    for (const b of district()) {
      const tiers = D.tiersOf(b);
      if (!tiers) {
        assert.equal(D.massTop(b, b.x, b.z), b.h);
        continue;
      }
      towers++;
      assert.equal(B.archetype(b), B.TYPE.deco);
      assert.equal(Math.max(...tiers.map(t => t.y1)), b.h, 'the highest tier is the height');
      assert.equal(tiers[0].w, b.w, 'the shaft is the footprint');
      for (let k = 1; k < tiers.length; k++) {
        assert.equal(tiers[k].y0, tiers[k - 1].y1, 'each tier stands on the one below');
        assert.ok(tiers[k].w < tiers[k - 1].w && tiers[k].d < tiers[k - 1].d, 'and inside it');
      }
      assert.equal(D.massTop(b, b.x, b.z), b.h, 'the middle is the full height');
      assert.equal(D.massTop(b, b.x + b.w / 2 - 0.01, b.z), tiers[0].y1, 'the edge is the shaft');
    }
    assert.ok(towers > 3, `${towers} setback towers`);
  });

  // Verifies: REQ-CITY-034
  it('give the walker the tiers to stand on in the city, and the box elsewhere', () => {
    const b = district().find(x => D.tiersOf(x));
    const tiers = D.tiersOf(b);
    const walker = style => ({ scene: { style }, cellAt: () => [b], decks: new Map(), spans: new Map() });
    const height = (style, x, z) => Walker.prototype.height.call(walker(style), x, z);
    assert.equal(height('city', b.x, b.z), b.y + b.h);
    // A body whose edge is over the shaft's ledge and not over the tier: the ledge.
    const edge = b.x + b.w / 2 + 0.05;
    assert.equal(height('city', edge, b.z), b.y + tiers[0].y1);
    assert.equal(height('circuit', edge, b.z), b.y + b.h);
  });

  // Verifies: REQ-CITY-033, REQ-PERF-009
  it('are built near the walker and on a zoomed-in map, and go with the camera', () => {
    const boxes = district();
    const details = new D.Details(m => m);
    details.setBoxes(boxes, true);
    details.setColors(boxes.map(() => '#2a78d6'), []);
    const settle = view => { for (let n = 0; n < 100 && details.update(view); n++); };
    const total = () => Object.values(details.counts()).reduce((a, c) => a + c, 0);

    // Walking in the middle of it: a budget of buildings a frame, then all of them near.
    const middle = { walking: true, x: 20, z: 20 };
    assert.equal(details.update(middle), true, 'everything at once');
    assert.ok(details.active.size <= D.BUDGET);
    settle(middle);
    const near = details.active.size;
    assert.ok(near > D.BUDGET && total() > 0);
    for (const i of details.active) assert.ok(Math.hypot(boxes[i].x - 20, boxes[i].z - 20) < D.WALK_REACH);
    // One draw a kind, whatever the number of buildings.
    assert.equal(details.group.children.length, D.DETAIL_KINDS.length);
    // Walked off the map: none, and nothing drawn.
    settle({ walking: true, x: 500, z: 500 });
    assert.equal(details.active.size, 0);
    assert.equal(total(), 0);
    assert.ok(details.group.children.every(mesh => !mesh.visible));

    // The map: nothing while a unit is a few pixels, some once zoomed in, only on screen.
    const project = (cx, cz, span) => (x, y, z) => ({ x: (x - cx) / span, y: (z - cz) / span });
    settle({ walking: false, zoom: 10, key: 'far', project: project(20, 20, 40) });
    assert.equal(details.active.size, 0, 'sub-pixel details built');
    settle({ walking: false, zoom: 80, key: 'close', project: project(20, 20, 6) });
    assert.ok(details.active.size > 0 && details.active.size < near);
    for (const i of details.active) assert.ok(Math.abs(boxes[i].x - 20) <= 6 * 1.15 && Math.abs(boxes[i].z - 20) <= 6 * 1.15);
    assert.ok(details.active.size <= D.MAX_BOXES);
    // The flags the facade reads say which.
    for (let i = 0; i < boxes.length; i++) assert.equal(details.flags[i], details.active.has(i) ? 1 : 0);
  });

  // Verifies: REQ-CITY-033
  it('leave dimmed buildings bare, and take their building color', () => {
    const boxes = district();
    const details = new D.Details(m => m);
    details.setBoxes(boxes, true);
    const colors = boxes.map(() => '#2a78d6');
    details.setColors(colors, []);
    const view = { walking: true, x: 20, z: 20 };
    for (let n = 0; n < 100 && details.update(view); n++);
    const lit = [...details.active];
    const dim = lit.slice(0, 20);
    const faded = boxes.map((_, i) => dim.includes(i));
    details.setColors(colors, faded, dim);
    for (let n = 0; n < 100 && details.update(view); n++);
    for (const i of dim) {
      assert.ok(!details.active.has(i), `dimmed ${i} kept its details`);
      assert.equal(details.flags[i], 0);
    }
    // A recolored building (hover) repaints its tinted details in place.
    const [i] = [...details.active].filter(k => details.cache.get(k).some(d => d.tinted));
    const recolored = colors.slice();
    recolored[i] = '#ff0000';
    details.setColors(recolored, faded, [i]);
    const want = new THREE.Color('#ff0000');
    let checked = 0;
    for (const [kind, { ranges, items }] of details.ranges) {
      const r = ranges.get(i);
      if (!r) continue;
      const mesh = details.meshes.get(kind);
      for (let k = r[0]; k < r[0] + r[1]; k++) {
        const c = [mesh.instanceColor.getX(k), mesh.instanceColor.getY(k), mesh.instanceColor.getZ(k)];
        assert.deepEqual(c, items[k][1].tinted ? [want.r, want.g, want.b] : [1, 1, 1]);
        checked++;
      }
    }
    assert.ok(checked > 0);
  });

  // Verifies: REQ-CITY-033
  it('are none in the other styles', () => {
    const details = new D.Details(m => m);
    const boxes = district();
    details.setBoxes(boxes, false);
    details.setColors(boxes.map(() => '#2a78d6'), []);
    assert.equal(details.update({ walking: true, x: 20, z: 20 }), false);
    assert.equal(details.active.size, 0);
  });
});
