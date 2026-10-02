// How thrown things fly: what the launcher works out before a shot leaves, and what
// the air does to it on the way.
//
// An aimed shot is solved rather than drawn, so the one thing it has to do is land
// where it was aimed - in still air and in a wind, near and at the tool's reach. A
// miss flies on under the same physics, which have to come out the same however
// fast the frames are, and have to give the wind its due: a bubble goes with it and
// a nail does not.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const { aim, along, fly, Breeze, BREEZE } = await import('../static/walk/ballistics.js');
const { TOOLS, TOOL_IDS } = await import('../static/walk/tools.js');

const THROWN = TOOL_IDS.filter(id => TOOLS[id].projectile);
const v = (x, y, z) => new THREE.Vector3(x, y, z);
const MUZZLE = v(0, 1.4, 0);

describe('an aimed shot', () => {
  // Verifies: REQ-TOOL-081
  it('lands on its mark, near and at its reach, in still air and a crosswind', () => {
    for (const id of ['rod', 'dart', 'nailer', 'grapple']) {
      const { flight, reach } = TOOLS[id];
      for (const wind of [v(0, 0, 0), v(BREEZE * 1.45, 0, 0), v(0, 0, BREEZE * 1.45)]) {
        for (const to of [v(0, 1.4, -reach * 0.95), v(reach * 0.3, 3, -reach * 0.3), v(0, 0.2, -reach * 0.6)]) {
          const shot = aim(MUZZLE, to, flight, wind);
          assert.ok(shot, `${id} found no way to ${to.toArray()} in a wind of ${wind.toArray()}`);
          assert.ok(shot.path.at(-1).distanceTo(to) < 1e-9, `${id} lands off its mark`);
          // And the path it lands by is the physics', not a curve drawn to the mark:
          // flown again from the same start at the same speed, step by step, it
          // passes where the path says.
          const at = along(shot.path, shot.T, shot.T / 2, v(0, 0, 0));
          const first = shot.path[1].clone().sub(shot.path[0]).multiplyScalar(120);
          const pos = MUZZLE.clone(), vel = first.clone();
          fly(pos, vel, flight, wind, shot.T / 2);
          assert.ok(pos.distanceTo(at) < 0.15 + 0.02 * reach, `${id}'s path is not its own flight`);
        }
      }
    }
  });

  // Verifies: REQ-TOOL-081
  it('leads into a crosswind rather than flying straight at the mark', () => {
    const to = v(0, 1.4, -TOOLS.dart.reach);
    const shot = aim(MUZZLE, to, TOOLS.dart.flight, v(BREEZE, 0, 0));
    const leave = shot.path[1].clone().sub(shot.path[0]);
    assert.ok(leave.x < 0, 'a dart thrown across the wind did not lead into it');
  });

  // Verifies: REQ-TOOL-081
  it('cannot send a bubble up the wind', () => {
    const { flight, reach } = TOOLS.bubbles;
    assert.ok(aim(MUZZLE, v(0, 1.4, -reach), flight, v(0, 0, 0)), 'a bubble does not reach in still air');
    assert.equal(aim(MUZZLE, v(0, 1.4, -reach), flight, v(0, 0, BREEZE * 1.45)), null, 'a bubble beat a gust');
  });
});

describe('a shot in the air', () => {
  // Verifies: REQ-TOOL-081
  it('flies the same at twenty frames a second as at a hundred and forty', () => {
    for (const id of THROWN) {
      const { flight } = TOOLS[id];
      const run = fps => {
        const pos = MUZZLE.clone(), vel = v(0, 0.2, -1).setLength(flight.speed);
        for (let i = 0; i < fps; i++) fly(pos, vel, flight, v(BREEZE, 0, 0), 1 / fps);
        return pos;
      };
      assert.ok(run(20).distanceTo(run(140)) < 0.05, `${id} lands somewhere else at a different frame rate`);
    }
  });

  // Verifies: REQ-TOOL-081
  it('goes with the wind as much as it is light', () => {
    // Sideways, by the time it has come five units on: as far as a bubble gets.
    const drift = id => {
      const { flight } = TOOLS[id];
      const pos = MUZZLE.clone(), vel = v(0, 0, -1).setLength(flight.speed);
      for (let t = 0; t < 4 && pos.z > -5; t += 1 / 120) fly(pos, vel, flight, v(BREEZE, 0, 0), 1 / 120);
      return pos.x;
    };
    assert.ok(drift('bubbles') > 0.25, `a bubble went ${drift('bubbles')} with the wind`);
    assert.ok(drift('nailer') < 0.005, `a nail was blown ${drift('nailer')} off course`);
    assert.ok(drift('nailer') < drift('dart') && drift('dart') < drift('bubbles'), 'what is lighter does not drift more');
  });
});

describe('the wind', () => {
  // Verifies: REQ-TOOL-081
  it('blows level, about a walker\'s pace, and changes over seconds rather than frames', () => {
    const breeze = new Breeze(7);
    let strongest = 0, weakest = Infinity;
    for (let t = 0; t < 900; t += 0.5) {
      const now = breeze.at(t), next = breeze.at(t + 1 / 60);
      assert.equal(now.y, 0);
      strongest = Math.max(strongest, now.length());
      weakest = Math.min(weakest, now.length());
      assert.ok(now.distanceTo(next) < 0.05, 'the wind jumped between two frames');
    }
    assert.ok(strongest < BREEZE * 1.5 && weakest > BREEZE * 0.25, `the wind ran from ${weakest} to ${strongest}`);
    assert.ok(strongest - weakest > BREEZE * 0.5, 'the wind never changed');
  });
});
