// The bridges: what links the islands to the mainland, and what carries a walker.
//
// Every island is on the end of one, so a bridge that goes nowhere or carries nobody
// is an island that cannot be reached on foot. Neither failing is visible in a
// screenshot until you go and look, which is what these are for.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box } from './stub.mjs';

const { bridgesFor, bridgeBounds, bridgeHeight } = await import('../static/city.js');

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
