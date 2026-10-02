// The guide that shows where a shot would go before it is taken.
//
// It is only worth having if it is right: a ring that says a dart will land somewhere
// it then does not is worse than no ring. So these ask it where a shot would land and
// then fly the shot, and compare.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const { TOOLS } = await import('../static/walk/tools.js');
const WALK = await import('../static/walk/walk.js');

/** The ground: one box with its top at 0, under everything. */
const GROUND = { kind: 'land', x: 0, y: -1, z: 0, w: 400, h: 1, d: 400 };
const WALL = { kind: 'building', x: 0, y: 0, z: -20, w: 6, h: 8, d: 2 };
const NEAR_WALL = { ...WALL, z: -6 }; // inside the nail gun's reach

/**
 * A walker as far as the guide concerns it: standing at the origin, the wind as
 * given, `boxes` the only things in the world, the gun's muzzle at the eye.
 */
function walker(tool, { boxes = [GROUND], wind = [0, 0, 0], pitch = 0, aimAt = null } = {}) {
  const scene = new THREE.Scene();
  const w = Object.assign(Object.create(WALK.Walker.prototype), {
    active: true, primary: TOOLS[tool], frozen: false, showing: null, arrival: null, dying: null, pull: null,
    wheel: { open: false }, p: { x: 0, z: 0, feet: 0, yaw: 0, pitch }, airNow: new THREE.Vector3(...wind),
    scene: { scene, bendable: m => m, walkCamera: { position: new THREE.Vector3(0, 0.45, 0) } }, viewmodel: null, darts: [],
    aim: { i: aimAt ? 0 : -1, point: aimAt?.point || null, bug: null },
    boxes: aimAt ? [aimAt.box] : [],
  });
  w.muzzle = () => null;
  w.boxAt = v => boxes.find(b => Math.abs(v.x - b.x) <= b.w / 2 && Math.abs(v.z - b.z) <= b.d / 2 && v.y >= b.y && v.y <= b.y + b.h) || null;
  return w;
}
const ring = w => w.guide.group.children.at(-1); // the landing marker

describe('the trajectory guide', () => {
  // Verifies: REQ-TOOL-082
  it('rings where a shot let go along the view lands, and the shot lands there', () => {
    for (const wind of [[0, 0, 0], [2, 0, 0]]) {
      const w = walker('dart', { wind, pitch: 0.3 });
      w.drawPath();
      assert.ok(w.guide.group.visible && ring(w).visible, 'no ring where a lobbed dart lands');
      const at = ring(w).position;
      // ... and the dart itself, flown under the same physics.
      const shot = w.shotFrom(TOOLS.dart, null);
      w.loose(shot);
      w.darts = [shot];
      let landed = null;
      for (let i = 0; i < 600 && !landed; i++) {
        w.flyFree(shot, 1 / 120);
        if (shot.mesh.position.y <= 0) landed = shot.mesh.position;
      }
      assert.ok(landed && Math.hypot(landed.x - at.x, landed.z - at.z) < 0.6,
        `the ring is at ${at.x.toFixed(2)}, ${at.z.toFixed(2)}; the dart came down at ${landed?.x.toFixed(2)}, ${landed?.z.toFixed(2)}`);
      if (wind[0]) assert.ok(at.x > 0.2, 'the guide leaves the wind out');
    }
  });

  // Verifies: REQ-TOOL-082
  it('ends an aimed shot on its mark, and a shot into a wall on the wall', () => {
    const point = new THREE.Vector3(0, 4, -19);
    const aimed = walker('dart', { aimAt: { box: WALL, point } });
    aimed.drawPath();
    assert.ok(ring(aimed).position.distanceTo(point) < 0.05, 'the ring is not on the aimed point');
    const free = walker('nailer', { boxes: [GROUND, NEAR_WALL], pitch: 0.05 });
    free.drawPath();
    assert.ok(ring(free).visible && Math.abs(ring(free).position.z - (NEAR_WALL.z + NEAR_WALL.d / 2)) < 0.05, 'a nail at a wall is not ringed on the wall');
  });

  // Verifies: REQ-TOOL-082
  it('shows nothing for a tool that throws nothing, or while the walker is held', () => {
    const net = walker('net');
    net.drawPath();
    assert.equal(net.guide.group.visible, false, 'a net has a trajectory');
    const held = walker('dart');
    held.frozen = true;
    held.drawPath();
    assert.equal(held.guide.group.visible, false, 'a held walker is shown a trajectory');
  });
});
