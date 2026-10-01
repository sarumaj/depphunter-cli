// The plants the city stands among, when the drawn models are not there.
//
// props.glb is fetched after the map is up, and a page without it - a test, a slow
// network, a file that failed to load - dresses the map from the tables in city.js
// instead. Those tables are the fallback nobody sees while the models work, which is
// how a tree in them could go on blocking the walker with nothing drawn to show for it.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box } from './stub.mjs';

const { makeProps, setNight, rampsFor, rampHeight } = await import('../static/map/city.js');
const { raised, buildParameters } = await import('../static/map/buildings.js');

describe('vegetation without the plant models', () => {
  // Verifies: REQ-CITY-022
  it('draws every tree it makes an obstacle of', () => {
    const land = box('land', 0, 0, 12, 12, { y: -0.45, h: 0.45 });
    const group = makeProps([land], m => m, 'city');
    assert.ok(group.userData.obstacles.length, 'a shore with no trees on it');
    // Two kinds a species, trunk then crown, before the bushes and the lamps; each
    // kind drawn in full and coarse (lod.js).
    const species = group.children.filter(mesh => !mesh.userData.coarse).slice(0, 6);
    for (const mesh of species) {
      const position = mesh.geometry.getAttribute('position');
      assert.ok(position && position.count > 0, 'a tree species drawn with no geometry');
      assert.ok(mesh.count > 0, 'a tree species with no instances drawn');
    }
  });
});

describe('street lamps at night', () => {
  // Verifies: REQ-CITY-036
  it('lights the city\'s lamps after dark only, with a glow and a pool on the pavement', () => {
    const land = box('land', 0, 0, 12, 12, { y: -0.45, h: 0.45 });
    const block = box('terrace', 0, 0, 8, 8, { y: 0, h: 0.2 });
    const group = makeProps([land, block], m => m, 'city');
    const night = group.children.filter(mesh => mesh.userData.afterDark);
    assert.ok(night.length, 'the city\'s lamps have nothing to light');
    assert.ok(night.some(mesh => mesh.count > 0), 'no lamp is lit');
    assert.ok(night.every(mesh => !mesh.visible), 'a lamp is lit before dark');
    setNight(group, true);
    assert.ok(night.every(mesh => mesh.visible), 'a lamp stayed dark at night');
    setNight(group, false);
    assert.ok(night.every(mesh => !mesh.visible), 'a lamp stayed lit by day');
  });

  // Verifies: REQ-CITY-036
  it('leaves an LED lit whether or not it is dark', () => {
    const block = box('terrace', 0, 0, 8, 8, { y: 0, h: 0.2 });
    const group = makeProps([block], m => m, 'circuit');
    const glow = group.children.filter(mesh => mesh.userData.glow);
    assert.ok(glow.length && glow.every(mesh => mesh.visible && !mesh.userData.afterDark));
  });
});

describe('the way up from one level to the next', () => {
  // A terrace on the street's level, one on it, and one on that: a terrace's node is
  // its kids' parentNode, which is how blocks() finds what stands on what.
  const street = box('terrace', 0, 0, 12, 12, { y: 0, h: 0.28 });
  const middle = box('terrace', 0, 0, 7, 7, { y: 0.28, h: 0.28 });
  const top = box('terrace', 0, 0, 3, 3, { y: 0.56, h: 0.28 });
  middle.node.parentNode = street.node;
  top.node.parentNode = middle.node;

  // Verifies: REQ-CITY-037
  it('paves a raised level as a plaza and leaves the street\'s own level a road', () => {
    assert.equal(raised(street), false);
    assert.equal(raised(middle), true);
    assert.equal(buildParameters(street)[2], 0);
    assert.equal(buildParameters(middle)[2], 1, 'the shader is not told the level is a plaza');
  });

  // Verifies: REQ-CITY-037
  it('drives up from a street, and climbs stairs from a plaza', () => {
    const ramps = rampsFor([street, middle, top]);
    const up = ramps.find(r => r.y1 === middle.y + middle.h), stairs = ramps.find(r => r.y1 === top.y + top.h);
    assert.ok(up && !up.stairs && up.drive, 'no ramp with a driveway up from the street');
    assert.ok(stairs?.stairs && !stairs.drive, 'no stairs up from the plaza');
    assert.ok(stairs.len < up.len, 'the stairs are no shorter than a ramp');
    // The walker goes up a flight as up a ramp: from the lower level to the upper.
    const foot = [stairs.origin[0] + stairs.n[0] * stairs.width / 2, stairs.origin[1] + stairs.n[1] * stairs.width / 2];
    const head = [foot[0] + stairs.u[0] * (stairs.len - 0.01), foot[1] + stairs.u[1] * (stairs.len - 0.01)];
    assert.ok(Math.abs(rampHeight(stairs, ...foot) - stairs.y0) < 1e-9);
    assert.ok(Math.abs(rampHeight(stairs, ...head) - stairs.y1) < 1e-9);
  });
});
