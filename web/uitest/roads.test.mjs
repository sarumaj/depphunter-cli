// The roads the selection's dependencies are drawn as, where the map is not flat.
//
// A road lies on whatever it crosses, and the city has levels: a nested terrace
// stands a step above the block it sits on, and the shore a step below the city that
// stands on it. Laid straight over those edges, a road puts a quad on end at the curb
// - a wall of asphalt where the street should climb. Two things keep that from
// happening, and both are invisible in a screenshot until you go and look: the ramps
// are laid into the routing grid as the ground they are and a road always goes round
// to one, however far round it is, and whatever rise is left where there is no ramp at
// all is graded to the gradient a road is drawn at.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box, scene, vertices } from './stub.mjs';

const { rampsFor } = await import('../static/city.js');
const { Routes } = await import('../static/routes.js');

const TERRACE = 0.28; // layout.js: how far a nested terrace stands above its block

/**
 * A block with two nested terraces on it, a building on each. Both children are long
 * enough on a side for rampsFor to put a ramp against them, so a road from one
 * building to the other has a way up and down that is not the wall.
 */
function twoLevels() {
  const block = box('terrace', 0, 0, 14, 10, { y: 0, h: TERRACE });
  const on = (b, parent) => { b.node.parentNode = parent.node; return b; };
  const left = on(box('terrace', -4, 0, 4, 4, { y: TERRACE, h: TERRACE }), block);
  const right = on(box('terrace', 4, 0, 4, 4, { y: TERRACE, h: TERRACE }), block);
  const top = 2 * TERRACE;
  return [
    box('land', 0, 0, 14, 10, { y: -0.45, h: 0.45 }),
    block, left, right,
    on(box('building', -4, 0, 1, 1, { y: top, h: 1 }), left),
    on(box('building', 4, 0, 1, 1, { y: top, h: 1 }), right),
  ];
}

/** The road a Routes laid between the two buildings of `boxes`, as its vertices. */
function road(boxes) {
  const [from, to] = boxes.filter(b => b.kind === 'building');
  const routes = new Routes(scene());
  routes.setLayout(boxes);
  routes.set([{ from, to, color: '#222222' }], from);
  return vertices(routes);
}

/**
 * The tallest wall of asphalt in a road: the greatest rise between two vertices lying
 * one over the other. The road is built from quads across its width, so a pair in the
 * same place at different heights is a quad standing on end - which is what a road
 * laid straight over an edge looks like, and what none of this may produce.
 */
function wall(laid) {
  let worst = 0;
  for (const v of laid) {
    for (const w of laid) {
      if (w !== v && Math.hypot(w.x - v.x, w.z - v.z) < 0.02) worst = Math.max(worst, Math.abs(w.y - v.y));
    }
  }
  return worst;
}

describe('a road over the city\'s levels', () => {
  it('climbs a terrace on its ramp rather than over its wall', () => {
    const boxes = twoLevels();
    const ramps = rampsFor(boxes);
    assert.equal(ramps.length, 2, 'the two nested terraces should each have a ramp');

    const laid = road(boxes);
    assert.ok(laid.length > 0, 'no road was laid at all');
    // It did climb: a road that stayed on the block never reached either building.
    assert.ok(laid.some(v => v.y > 2 * TERRACE - 0.03), 'the road never reached the upper level');

    // And it climbed on both ramps: a road is wider than a ramp's roadway, so what is
    // asked is that it lies along one at a height only the ramp has.
    for (const r of ramps) {
      const on = laid.filter(v => v.x > r.x0 - 0.2 && v.x < r.x1 + 0.2 && v.z > r.z0 && v.z < r.z1
        && v.y > r.y0 + 0.05 && v.y < r.y1 - 0.05);
      assert.ok(on.length > 0, `nothing runs up the ramp at (${r.x0.toFixed(1)}, ${r.z0.toFixed(1)})`);
    }
    assert.ok(wall(laid) < TERRACE * 0.5, `the road stands ${wall(laid).toFixed(3)} on end somewhere`);
  });

  it('goes round to the ramp however far round it is', () => {
    // A block long enough that its one nested terrace is a fair walk across, with the
    // buildings facing each other along its middle. rampsFor puts the ramp at one end
    // of a side, so the way up is nowhere near the line between the two - and taking it
    // costs a detour several times the length of the direct route. The road takes it
    // anyway, which is the whole of this: a wall is not something a road buys its way
    // over when the way round gets long enough.
    const block = box('terrace', 0, 0, 34, 30, { y: 0, h: TERRACE });
    const on = (b, parent) => { b.node.parentNode = parent.node; return b; };
    const up = on(box('terrace', 9, 0, 12, 26, { y: TERRACE, h: TERRACE }), block);
    const boxes = [
      box('land', 0, 0, 34, 30, { y: -0.45, h: 0.45 }),
      block, up,
      box('building', -13, 0, 1, 1, { y: TERRACE, h: 1 }),
      on(box('building', 9, 0, 1, 1, { y: 2 * TERRACE, h: 1 }), up),
    ];
    const [ramp] = rampsFor(boxes);
    assert.ok(ramp, 'the terrace should have a ramp somewhere on it');
    // The direct crossing is on the line between the buildings; the ramp is not.
    // Ten units off the line between the buildings, so going round to it and back is
    // a detour of more than twenty against a direct route of about twenty-two: no
    // budget a road could be given would buy this one, and it is bought anyway.
    assert.ok(Math.min(Math.abs(ramp.z0), Math.abs(ramp.z1)) > 10,
      `the ramp is only ${Math.min(Math.abs(ramp.z0), Math.abs(ramp.z1)).toFixed(1)} off the line - too near to test anything`);

    const laid = road(boxes);
    assert.ok(laid.some(v => v.y > 2 * TERRACE - 0.03), 'the road never reached the upper level');
    // Everything between the two levels is on the ramp, so the road climbed there and
    // nowhere else - which means it walked the length of the block to get to it.
    const between = laid.filter(v => v.y > TERRACE + 0.04 && v.y < 2 * TERRACE - 0.04);
    assert.ok(between.length > 0, 'the road never climbed at all');
    for (const v of between) {
      assert.ok(v.x > ramp.x0 - 0.3 && v.x < ramp.x1 + 0.3 && v.z > ramp.z0 - 0.3 && v.z < ramp.z1 + 0.3,
        `road at (${v.x.toFixed(2)}, ${v.y.toFixed(2)}, ${v.z.toFixed(2)}) climbs off the ramp`);
    }
    assert.ok(wall(laid) < TERRACE * 0.5, `the road stands ${wall(laid).toFixed(3)} on end somewhere`);
  });

  it('grades the rise where no ramp connects two levels', () => {
    // The shore and the city on it, with nothing nested: rampsFor builds nothing, so
    // the step off the block onto the sand is the one a road has to take itself.
    const boxes = [
      box('land', 0, 0, 20, 10, { y: -0.45, h: 0.45 }),
      box('terrace', 0, 0, 6, 6, { y: 0, h: TERRACE }),
      box('building', 0, 0, 1, 1, { y: TERRACE, h: 1 }),
      box('building', 8, 0, 1, 1, { y: 0, h: 1 }), // out on the shore
    ];
    assert.equal(rampsFor(boxes).length, 0);

    const laid = road(boxes);
    assert.ok(laid.length > 0, 'no road was laid at all');
    assert.ok(laid.some(v => v.y > TERRACE - 0.03), 'the road never got up onto the block');
    assert.ok(laid.some(v => v.y < 0.05), 'the road never got down onto the shore');

    assert.ok(wall(laid) < TERRACE * 0.5, `the road stands ${wall(laid).toFixed(3)} on end somewhere`);
  });
});
