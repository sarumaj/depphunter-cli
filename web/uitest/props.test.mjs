// The plants the city stands among, when the drawn models are not there.
//
// props.glb is fetched after the map is up, and a page without it - a test, a slow
// network, a file that failed to load - dresses the map from the tables in city.js
// instead. Those tables are the fallback nobody sees while the models work, which is
// how a tree in them could go on blocking the walker with nothing drawn to show for it.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box } from './stub.mjs';

const { makeProps } = await import('../static/city.js');

describe('vegetation without the plant models', () => {
  // Verifies: REQ-CITY-022
  it('draws every tree it makes an obstacle of', () => {
    const land = box('land', 0, 0, 12, 12, { y: -0.45, h: 0.45 });
    const group = makeProps([land], m => m, 'city');
    assert.ok(group.userData.obstacles.length, 'a shore with no trees on it');
    // Two meshes a species, trunk then crown, before the bushes and the lamps.
    const species = group.children.slice(0, 6);
    for (const mesh of species) {
      const pos = mesh.geometry.getAttribute('position');
      assert.ok(pos && pos.count > 0, 'a tree species drawn with no geometry');
    }
  });
});
