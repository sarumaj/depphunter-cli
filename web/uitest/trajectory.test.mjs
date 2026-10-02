// The guide that shows where a shot would go before it is taken.
//
// It is only worth having if it is right: a marker that says a dart will land somewhere
// it then does not is worse than no marker, and it is what the hunting hand is aimed
// with now that the crosshair is gone. So these ask it where a shot would land, then
// throw the shot itself, and compare - standing and running, in still air and in the
// wind, and with a dart that steers.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const { TOOLS } = await import('../static/walk/tools.js');
const WALK = await import('../static/walk/walk.js');

/** The ground: one box with its top at 0, under everything near. */
const GROUND = { kind: 'land', x: 0, y: -1, z: 0, w: 400, h: 1, d: 400, i: 0 };
const WALL = { kind: 'building', x: 0, y: 0, z: -6, w: 6, h: 8, d: 2, i: 1 };

/**
 * A walker as far as throwing concerns it: standing at the origin, the wind and their
 * own motion as given, `boxes` the only things in the world, the hand at the eye.
 */
function walker(tool, { boxes = [GROUND], wind = [0, 0, 0], moving = [0, 0, 0], pitch = 0, yaw = 0 } = {}) {
  const scene = new THREE.Scene();
  const w = Object.assign(Object.create(WALK.Walker.prototype), {
    active: true, primary: TOOLS[tool], frozen: false, showing: null, arrival: null, dying: null, pull: null,
    handsOff: false, wheel: { open: false }, p: { x: 0, z: 0, feet: 0, yaw, pitch },
    airNow: new THREE.Vector3(...wind), moving: new THREE.Vector3(...moving),
    scene: { scene, bendable: m => m, walkCamera: { position: new THREE.Vector3(0, 0.45, 0) } },
    viewmodel: null, darts: [], bugs: null, boxes, tagged: new Set(),
  });
  w.muzzle = () => null;
  w.tag = () => {};
  w.flash = () => {};
  w.boxAt = v => boxes.find(b => Math.abs(v.x - b.x) <= b.w / 2 && Math.abs(v.z - b.z) <= b.d / 2 && v.y >= b.y && v.y <= b.y + b.h) || null;
  return w;
}

/** Throws the walker's tool for real and flies it, at 60 frames a second, until it is over; where it ended. */
function thrown(w) {
  const shot = w.shotFrom(w.primary, null);
  w.loose(shot);
  for (let i = 0; i < 600; i++) if (w.flyFree(shot, 1 / 60)) return shot.mesh.position;
  return null;
}

describe('the trajectory guide', () => {
  // Verifies: REQ-TOOL-082
  it('puts its marker where the shot then lands, standing or running, in still air or wind', () => {
    for (const moving of [[0, 0, 0], [0, 0, -8.5], [6, 0, 0]]) {
      for (const wind of [[0, 0, 0], [3, 0, 0]]) {
        const w = walker('dart', { wind, moving, pitch: 0.15 });
        const at = w.predict();
        assert.ok(at, `no landing predicted running ${moving} in ${wind}`);
        const landed = thrown(w);
        assert.ok(landed && Math.hypot(landed.x - at.point.x, landed.z - at.point.z) < 0.6,
          `the marker is at ${at.point.x.toFixed(2)}, ${at.point.z.toFixed(2)}; the dart came down at ${landed?.x.toFixed(2)}, ${landed?.z.toFixed(2)}`);
      }
    }
    // Running sideways carries the shot sideways, and the guide says so.
    const still = walker('dart', { pitch: 0.25 }).predict(), running = walker('dart', { pitch: 0.25, moving: [6, 0, 0] }).predict();
    assert.ok(running.point.x - still.point.x > 3, 'the guide leaves out how the walker is moving');
  });

  // Verifies: REQ-TOOL-082
  it('follows a tracking dart that steers onto a wall, and rings a shot that strikes one on its face', () => {
    // A wall ahead and off to one side: the dart turns onto it, and the guide with it.
    const side = { ...WALL, x: 4, z: -18, w: 4, d: 2 };
    const w = walker('dart', { boxes: [GROUND, side], pitch: 0.1 });
    const at = w.predict();
    assert.equal(at?.box, side, 'the guide does not steer the dart onto the wall');
    const landed = thrown(w);
    assert.ok(landed.distanceTo(at.point) < 0.6, 'the dart did not land where the guide said');
    // A nail into a wall ahead is ringed on the wall's face, not inside it.
    const nail = walker('nailer', { boxes: [GROUND, WALL], pitch: 0.05 }).predict();
    assert.ok(Math.abs(nail.point.z - (WALL.z + WALL.d / 2)) < 0.05, 'a nail at a wall is not ringed on its face');
    assert.equal(nail.normal.z, 1);
  });

  // Verifies: REQ-TOOL-082
  it('draws nothing where the shot would land on nothing, for a tool that throws nothing, or while held', () => {
    // Out over the water: no ground under the shot.
    const sea = walker('nailer', { boxes: [] });
    sea.prediction = sea.predict();
    sea.drawPath();
    assert.equal(sea.prediction, null);
    assert.equal(sea.guide.group.visible, false, 'a guide is drawn to nowhere');
    const net = walker('net');
    net.prediction = { path: [new THREE.Vector3(), new THREE.Vector3(0, 0, -1)], point: new THREE.Vector3(0, 0, -1), normal: null };
    net.drawPath();
    assert.equal(net.guide.group.visible, false, 'a net has a trajectory');
    const held = walker('dart', { pitch: 0.25 });
    held.prediction = held.predict();
    held.frozen = true;
    held.drawPath();
    assert.equal(held.guide.group.visible, false, 'a held walker is shown a trajectory');
    held.frozen = false;
    held.drawPath();
    assert.equal(held.guide.group.visible, true);
  });
});

describe('aiming help', () => {
  // A bug standing still where it is put, and the hunt's own test for a shot that
  // passes it (bugs.js at: within CATCH).
  function withBug(w, at) {
    const bug = { position: new THREE.Vector3(...at), caught: false, f: { severity: 'high', title: 't' } };
    w.bugs = { bugs: [bug], at: (v, r = 0.42) => (!bug.caught && v.distanceTo(bug.position) < r ? bug : null), catch: b => { b.caught = true; } };
    return bug;
  }

  // Verifies: REQ-TOOL-083
  it('locks on to a bug the path passes near, and the shot then catches it', () => {
    const w = walker('nailer', { pitch: 0 });
    // Level with the muzzle, six ahead and a little more than a catch off the line.
    const bug = withBug(w, [0.6, 0.45 - 0.08 + 0.07, -6]);
    const at = w.predict();
    assert.equal(at?.lock, bug, 'no lock on a bug just off the line');
    assert.equal(at.bug, bug, 'the guide does not end on the bug it locked on to');
    w.prediction = at;
    w.aim = { i: -1, point: at.point, bug: bug, box: null };
    w.swing = 0;
    w.fire();
    const shot = w.darts.at(-1);
    for (let i = 0; i < 120 && !bug.caught; i++) w.flyFree(shot, 1 / 60);
    assert.ok(bug.caught, 'the shot did not home on the bug it was locked on to');
  });

  // Verifies: REQ-TOOL-083
  it('does not lock on to a bug well off the line', () => {
    const w = walker('nailer', { pitch: 0 });
    withBug(w, [2.5, 0.4, -6]);
    assert.equal(w.predict()?.lock ?? null, null);
  });

  // Verifies: REQ-TOOL-083
  it('draws a grapple up to the roof edge it would hold on, and says when it would not hold', () => {
    const tower = { kind: 'building', x: 0, y: 0, z: -12, w: 4, h: 3, d: 2, i: 1 };
    const w = walker('nailer', { boxes: [GROUND, tower] });
    w.secondary = TOOLS.grapple;
    // Looking at the face a little under the roof: drawn up to the edge, where it holds.
    w.lookingAt = () => ({ box: tower, point: new THREE.Vector3(0, 1.9, -11) });
    const up = w.planLine();
    assert.ok(up.snapped && up.holds, 'a look just under the roof is not taken up to its edge');
    assert.ok(Math.abs(up.point.y - (3 - 0.3)) < 1e-9);
    // Looking at its foot from close by: the edge is far off where the walker looks.
    const near = walker('nailer', { boxes: [GROUND, tower] });
    near.secondary = TOOLS.grapple;
    near.p.z = -9;
    near.lookingAt = () => ({ box: { ...tower, h: 12 }, point: new THREE.Vector3(0, 0.2, -11) });
    const low = near.planLine();
    assert.ok(!low.snapped && !low.holds, 'a hook at the foot of a tall wall is said to hold');
  });
});
