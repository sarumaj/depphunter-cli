// The bridges: what carries a walker, and what carries a road.
//
// Both used to be wrong in the same place. A deck answered "yes, stand here" for any
// point within its drawn width, so a walker whose body was half over the railing was
// still held up; and the roads knew nothing about bridges at all, so a road to an
// island swam the bay wherever the sweep found it cheapest - beside a crossing that
// was right there. Neither is visible in a screenshot until you go and look, which is
// what these are for.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box, scene, vertices } from './stub.mjs';

const { bridgesFor, bridgeBounds, bridgeHeight } = await import('../static/city.js');
const { Routes } = await import('../static/routes.js');

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

const gap = (v, from = 3.1, to = 7.9) => v > from && v < to;

describe('a bridge deck', () => {
  it('links every island to the mainland and to nothing twice', () => {
    // Four shores, one of them the mainland: a spanning tree over four nodes has
    // three edges, and every island is on the end of one.
    const boxes = [box('land', 0, 0, 8, 8), box('land', 12, 0, 3, 3), box('land', -12, 0, 3, 3), box('land', 0, 12, 3, 3)];
    const bridges = bridgesFor(boxes);
    assert.equal(bridges.length, 3);
    const joined = new Set(bridges.flatMap(r => [r.a, r.b]));
    for (const island of boxes.slice(1)) assert.ok(joined.has(island), 'an island no bridge reaches');
  });

  it('arches: the middle stands above the shores it leaves', () => {
    const [r] = bridgesFor(twoIslands());
    const middle = bridgeHeight(r, (r.from + r.to) / 2, r.across);
    assert.ok(middle > SHORE, `the deck's middle is at ${middle}, no higher than the shore`);
    // Both ends meet their shore, or stepping on would be a step up onto nothing.
    assert.ok(Math.abs(bridgeHeight(r, r.from, r.across) - SHORE) < 1e-9);
    assert.ok(Math.abs(bridgeHeight(r, r.to, r.across) - SHORE) < 1e-9);
  });

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

describe('a road to an island', () => {
  it('crosses on the bridge rather than beside it', () => {
    const boxes = twoIslands();
    const [ashore, across] = boxes.filter(b => b.kind === 'building');
    const [r] = bridgesFor(boxes);
    const deck = bridgeBounds(r);

    const routes = new Routes(scene());
    routes.setLayout(boxes);
    routes.set([{ from: ashore, to: across, color: '#222222' }], ashore);

    const laid = vertices(routes);
    assert.ok(laid.length > 0, 'no road was laid at all');

    const overWater = laid.filter(v => gap(v.x));
    assert.ok(overWater.length > 0, 'the road never crossed the water');

    // Every piece of road out over the gap is on the deck. The road is a ribbon with
    // its own width and its casing, so it may sit a little proud of the deck's line;
    // what it may not do is be somewhere else entirely.
    const half = (deck.z1 - deck.z0) / 2 + 0.2;
    const strays = overWater.filter(v => Math.abs(v.z - r.across) > half);
    assert.equal(strays.length, 0,
      `${strays.length} of ${overWater.length} road vertices cross the water away from the bridge` +
      ` (worst z = ${strays.reduce((m, v) => Math.max(m, Math.abs(v.z - r.across)), 0)})`);

    // ... and it climbs the arch rather than lying flat at the shore's level, which
    // is what a causeway would do.
    const top = overWater.reduce((m, v) => Math.max(m, v.y), -Infinity);
    assert.ok(top > SHORE + 0.1, `the road stayed at ${top}, flat over the water`);
  });

  it('still crosses where there is no bridge to take', () => {
    // One shore, so bridgesFor builds nothing: the causeway is the fallback and has
    // to keep working, or an island nothing could be built to would be unreachable.
    const boxes = [
      box('land', 0, 0, 6, 6),
      box('building', -2, 2, 0.8, 0.8, { y: 0.2, h: 1 }),
      box('building', 2, -2, 0.8, 0.8, { y: 0.2, h: 1 }),
    ];
    assert.equal(bridgesFor(boxes).length, 0);
    const [a, b] = boxes.filter(x => x.kind === 'building');
    const routes = new Routes(scene());
    routes.setLayout(boxes);
    routes.set([{ from: a, to: b, color: '#222222' }], a);
    assert.ok(vertices(routes).length > 0, 'no road between two buildings on one shore');
  });
});
