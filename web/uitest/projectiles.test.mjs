// What a shot leaves behind once it is over.
//
// Every shot is built afresh - its mesh, its geometries and its materials (tools.js
// projectile), and the line it trails (walk.js shotFrom) - and on a GPU those are
// buffers and programs the renderer keeps until it is told to let them go. Taking a
// shot off the map is not telling it: a walker who hoses a wall with the nail gun for
// a minute would leave thousands of them behind. So these fire each tool that throws
// something many times over, let the shots end every way a shot ends, and count what
// is still held: a counting wrapper round the scene notes every geometry and material
// put on it, and crosses each off when it is disposed.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const { TOOLS, TOOL_IDS } = await import('../static/tools.js');
const WALK = await import('../static/walk.js');

// The tools that throw something, and the two of them that pay out a line to it.
const THROWN = TOOL_IDS.filter(id => TOOLS[id].projectile);
const LINES = THROWN.filter(id => TOOLS[id].reel);

/**
 * A scene that counts. Whatever is added to it is searched for geometries and
 * materials, and each is held in `live` until its dispose event.
 */
function countingScene() {
  const scene = new THREE.Scene();
  const live = new Set(), seen = new WeakSet();
  const note = resource => {
    if (!resource || seen.has(resource)) return;
    seen.add(resource);
    live.add(resource);
    resource.addEventListener('dispose', () => live.delete(resource));
  };
  const add = scene.add.bind(scene);
  scene.add = (...objects) => {
    for (const object of objects) {
      object.traverse(part => {
        note(part.geometry);
        for (const material of [part.material].flat()) note(material);
      });
    }
    return add(...objects);
  };
  return { scene, live };
}

/**
 * A walker as far as throwing concerns it: standing at the origin on flat ground with
 * nothing around, with the shot methods off Walker. There are no bugs and no
 * buildings, so a shot ends where it lands, at the end of its reach or its line, or
 * when the line it bit with is cut.
 */
function thrower() {
  const { scene, live } = countingScene();
  const W = WALK.Walker.prototype;
  const walker = {
    p: { x: 0, z: 0, feet: 0, yaw: 0, pitch: 0, vy: 0 }, keys: new Set(), darts: [], puffs: [], pull: null,
    boxes: [], bugs: null, fell: null, viewmodel: null, offhand: null,
    scene: { scene, bendable: material => material, unbend() {} },
    muzzle: () => null, boxAt: () => null, height: () => 0, flying: () => false,
    flash() {}, tag() {}, drawHud() {},
    shotFrom: W.shotFrom, loose: W.loose, updateDarts: W.updateDarts, dropDarts: W.dropDarts, dropPuff: W.dropPuff,
    flyCast: W.flyCast, flyFree: W.flyFree, dressDart: W.dressDart,
    cutLine: W.cutLine, hook: W.hook, glance: W.glance, rebound: W.rebound, puff: W.puff,
    steer: W.steer, wallAhead: W.wallAhead,
  };
  return { walker, live };
}

/** Lets every shot in the air fly until it is over, at 20 frames a second. */
function flyOut(walker) {
  for (let i = 0; i < 400 && walker.darts.length; i++) walker.updateDarts(0.05);
  assert.equal(walker.darts.length, 0, 'a shot was still in the air after 20 seconds');
}

/** Where a line is aimed when it is meant to bite: the ground five units ahead. */
const GROUND = { kind: 'land', x: 0, y: -1, z: -5, w: 20, h: 1, d: 20 };

/** A shot of `id` aimed at the ground ahead, which a line bites. */
function aimed(walker, id) {
  const shot = walker.shotFrom(TOOLS[id], null);
  Object.assign(shot, { to: new THREE.Vector3(0, 0, -5), target: GROUND, bug: null, T: 0.3, arc: 0.1 });
  return shot;
}

describe('what a shot leaves behind', () => {
  it('frees every shot that flies out, however many are thrown', () => {
    for (const id of THROWN) {
      const { walker, live } = thrower();
      for (let round = 0; round < 20; round++) {
        walker.loose(walker.shotFrom(TOOLS[id], null));
        flyOut(walker);
      }
      assert.equal(live.size, 0, `${live.size} geometries and materials of ${id} shots are still held`);
    }
  });

  it('frees a line and what it bit with when the line is cut', () => {
    for (const id of LINES) {
      const { walker, live } = thrower();
      for (let round = 0; round < 20; round++) {
        aimed(walker, id);
        flyOut(walker);
        assert.ok(walker.pull, `a ${id} line aimed at the ground did not bite`);
        walker.cutLine();
      }
      assert.equal(live.size, 0, `${live.size} geometries and materials of ${id} lines are still held`);
    }
  });

  it('frees the one line a second one replaces', () => {
    const { walker, live } = thrower();
    for (let round = 0; round < 20; round++) {
      aimed(walker, 'grapple');
      flyOut(walker);
    }
    walker.cutLine();
    assert.equal(live.size, 0, `${live.size} geometries and materials of replaced lines are still held`);
  });

  it('frees what is still in the air when the shots are dropped', () => {
    const { walker, live } = thrower();
    for (let round = 0; round < 20; round++) {
      for (const id of THROWN) walker.loose(walker.shotFrom(TOOLS[id], null));
      walker.updateDarts(0.05);
      walker.dropDarts();
    }
    assert.equal(live.size, 0, `${live.size} geometries and materials of dropped shots are still held`);
  });
});
