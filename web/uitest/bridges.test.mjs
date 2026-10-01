// The bridges: what links the islands to the mainland, and what carries a walker.
//
// Every island is on the end of one, so a bridge that goes nowhere or carries nobody
// is an island that cannot be reached on foot. Neither failing is visible in a
// screenshot until you go and look, which is what these are for.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box } from './stub.mjs';

const { bridgesFor, bridgeBounds, bridgeHeight, rampsFor, rampHeight, makeProps, RAIL_H } = await import('../static/map/city.js');
const { Walker } = await import('../static/walk/walk.js');

// A body's radius, as walk.js knows it: what a deck is narrowed by before it will
// carry anyone.
const BODY = 0.12;
const SHORE = 0.2; // the land boxes' top, and the height a bridge starts from

/**
 * Two shores with a stretch of water between them, and a building on each. The gap
 * runs from x = 3 to x = 8; the bridge crosses it at z = 0, where the two shores
 * face each other, and both buildings sit well off that line so a road has to come
 * to the crossing rather than happening to be on it.
 */
function twoIslands() {
  return [
    box('land', 0, 0, 6, 6),
    box('land', 10, 0, 4, 4),
    box('building', -2, 2, 0.8, 0.8, { y: SHORE, h: 1 }),
    box('building', 10, 1.2, 0.8, 0.8, { y: SHORE, h: 1 }),
  ];
}

describe('a bridge deck', () => {
  // Verifies: REQ-CITY-026
  it('links every island to the mainland and to nothing twice', () => {
    // Four shores, one of them the mainland: a spanning tree over four nodes has
    // three edges, and every island is on the end of one.
    const boxes = [box('land', 0, 0, 8, 8), box('land', 12, 0, 3, 3), box('land', -12, 0, 3, 3), box('land', 0, 12, 3, 3)];
    const bridges = bridgesFor(boxes);
    assert.equal(bridges.length, 3);
    const joined = new Set(bridges.flatMap(r => [r.a, r.b]));
    for (const island of boxes.slice(1)) assert.ok(joined.has(island), 'an island no bridge reaches');
  });

  // Verifies: REQ-CITY-027, REQ-CITY-028
  it('arches: the middle stands above the shores it leaves', () => {
    const [r] = bridgesFor(twoIslands());
    const middle = bridgeHeight(r, (r.from + r.to) / 2, r.across);
    assert.ok(middle > SHORE, `the deck's middle is at ${middle}, no higher than the shore`);
    // Both ends meet their shore, or stepping on would be a step up onto nothing.
    assert.ok(Math.abs(bridgeHeight(r, r.from, r.across) - SHORE) < 1e-9);
    assert.ok(Math.abs(bridgeHeight(r, r.to, r.across) - SHORE) < 1e-9);
  });

  // Verifies: REQ-CITY-028, REQ-WALK-035
  it('carries a body only while the whole of it is on the deck', () => {
    const [r] = bridgesFor(twoIslands());
    const mid = (r.from + r.to) / 2;
    const half = (bridgeBounds(r).z1 - bridgeBounds(r).z0) / 2;

    // The deck as drawn reaches its own edge.
    assert.notEqual(bridgeHeight(r, mid, r.across + half - 1e-6), -Infinity);
    assert.equal(bridgeHeight(r, mid, r.across + half + 1e-6), -Infinity);

    // Asked with a body's radius it stops a body's radius short, so the walker's
    // shoulder meets the railing instead of their middle reaching it. This is the
    // whole of the fix: with no inset, standing here was standing on the bridge.
    const railing = r.across + half - BODY;
    assert.notEqual(bridgeHeight(r, mid, railing - 1e-6, BODY), -Infinity);
    assert.equal(bridgeHeight(r, mid, railing + 1e-6, BODY), -Infinity,
      'a body may still hang over the railing');

    // And a point beyond the deck is beyond it either way.
    assert.equal(bridgeHeight(r, mid, r.across + half + 0.5, BODY), -Infinity);
  });

  it('is the same deck whether the inset is left off or given as nothing', () => {
    const [r] = bridgesFor(twoIslands());
    const mid = (r.from + r.to) / 2;
    assert.equal(bridgeHeight(r, mid, r.across), bridgeHeight(r, mid, r.across, 0));
  });
});

describe('the railings', () => {
  // A walker as far as the railings concern it: where their feet are, and the bridges
  // indexed the way walk.js indexes them.
  function onBridge() {
    const [r] = bridgesFor(twoIslands());
    const w = Object.assign(Object.create(Walker.prototype), { p: {}, ramps: [], bridges: [r] });
    w.indexDecks();
    const mid = (r.from + r.to) / 2, half = (bridgeBounds(r).z1 - bridgeBounds(r).z0) / 2;
    return { r, w, mid, deck: bridgeHeight(r, mid, r.across, BODY), side: r.across + half };
  }

  // Verifies: REQ-CITY-028
  it('hold a walker on the deck, standing or in the air below their top', () => {
    const { w, mid, deck, side } = onBridge();
    for (const feet of [deck, deck - 0.05, deck + RAIL_H / 2]) {
      w.p.feet = feet;
      assert.ok(w.railed(mid, side - BODY - 0.01, mid, side + 0.2), `a walker at ${feet - deck} went through the railing`);
    }
  });

  // Verifies: REQ-CITY-028
  it('let a jump that clears them go over, and leave the ends and the water under the deck open', () => {
    const { r, w, mid, deck, side } = onBridge();
    w.p.feet = deck + RAIL_H + 0.01;
    assert.equal(w.railed(mid, side - BODY - 0.01, mid, side + 0.2), false, 'a jump over the railing was held');
    w.p.feet = SHORE;
    assert.equal(w.railed(r.to - 0.01, r.across, r.to + 0.2, r.across), false, 'the end of the bridge is railed off');
    w.p.feet = SHORE - 0.6;
    assert.equal(w.railed(mid, r.across, mid, side + 0.2), false, 'the deck holds a swimmer under it');
  });
});

describe('the road off a bridge', () => {
  // Two shores, each with its terrace standing 1.2 in from the water, as layout.js
  // places them: the bridge lands on the shore, short of the terrace's streets.
  function shoresWithStreets() {
    const main = box('land', 0, 0, 8.4, 8.4), island = box('land', 14, 0, 6.4, 6.4);
    const mainBlock = box('terrace', 0, 0, 6, 6, { y: SHORE, h: 0.28 });
    const islandBlock = box('terrace', 14, 0, 4, 4, { y: SHORE, h: 0.28 });
    mainBlock.node = main.node;
    islandBlock.node = island.node;
    for (const b of [main, island]) Object.assign(b, { y: 0, h: SHORE });
    return [main, island, mainBlock, islandBlock];
  }

  // Verifies: REQ-CITY-038
  it('carries the deck on up each shore to the streets of its terrace', () => {
    const boxes = shoresWithStreets();
    const [bridge] = bridgesFor(boxes);
    const ways = rampsFor(boxes).filter(r => r.approach);
    assert.equal(ways.length, 2, 'a bridge end with no road up to the streets');
    for (const r of ways) {
      assert.equal(r.width, bridgeBounds(bridge).z1 - bridgeBounds(bridge).z0, 'the road narrows off the bridge');
      const middle = r.origin[1] + r.width / 2;
      // From the deck's end, at the shore's height, to the terrace's top at its wall.
      assert.ok(Math.abs(rampHeight(r, r.origin[0], middle) - SHORE) < 1e-9);
      const top = r.origin[0] + r.u[0] * r.len;
      assert.ok(Math.abs(rampHeight(r, top, middle) - (SHORE + 0.28)) < 1e-9);
      assert.ok(Math.abs(bridgeHeight(bridge, r.origin[0], middle) - SHORE) < 1e-9, 'the road and the deck meet at different heights');
    }
  });

  // Verifies: REQ-CITY-038
  it('plants no tree on it', () => {
    const boxes = shoresWithStreets();
    const ways = rampsFor(boxes).filter(r => r.approach);
    const group = makeProps(boxes, m => m, 'city');
    for (const o of group.userData.obstacles) {
      for (const r of ways) assert.ok(o.x < r.x0 || o.x > r.x1 || o.z < r.z0 || o.z > r.z1, 'a tree or a lamp stands on the road');
    }
  });
});
