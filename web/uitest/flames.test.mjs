// What fire looks like, in the parts of it that are arithmetic.
//
// The look itself is a judgement and no test settles it. What a test can hold is the
// handful of properties the look is built on, each of which was got wrong at least
// once on the way here and none of which announces itself when it breaks: a flame
// pinched at the root floats like a leaf, a bright root draws a lit bar across the
// foot of the fire, a blade with no fall-off at its edges is cut paper however many
// of them you crowd together, and a fire that reshuffles itself on a relayout is a
// fire nobody can point at twice.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const F = await import('../static/flames.js');

describe('the shape of a tongue', () => {
  it('is blunt where it leaves the fire and pointed at the tip', () => {
    assert.ok(F.across(0) > 0.9, 'the root is pinched, so the tongue floats');
    assert.equal(F.across(1), 0, 'the tip is not drawn to a point');
    // Fullest just above the root, which is what stops it being a plain cone.
    const fullest = [...Array(21).keys()].map(i => F.across(i / 20));
    const peak = fullest.indexOf(Math.max(...fullest));
    assert.ok(peak > 0 && peak < 6, `a tongue is fattest at ${peak / 20} up itself`);
    // And never doubles back, or the blade folds through itself.
    for (let i = 8; i < 20; i++) assert.ok(fullest[i] >= fullest[i + 1], 'the taper is not monotonic');
  });

  it('fades to nothing at the root and at both its edges, not to black', () => {
    // The color attribute carries alpha, and the fade is in the alpha alone. Fading
    // the color instead would give every blade a dark rim, which on a sunlit wall is
    // an outline drawn round the fire - and outlines are the one thing a crowd of
    // tongues must not have if it is going to read as a body.
    const g = F.tongueGeometry();
    const pos = g.getAttribute('position'), col = g.getAttribute('color');
    assert.equal(col.itemSize, 4, 'a tongue has no alpha, so it can only fade to black');
    assert.equal(pos.count, col.count, 'a vertex has no color');

    let rootA = 0, spineA = 0, edgeA = 1, dimmest = 9;
    for (let i = 0; i < pos.count; i++) {
      const y = pos.getY(i), a = col.getW(i);
      const lum = col.getX(i) + col.getY(i) + col.getZ(i);
      dimmest = Math.min(dimmest, lum);
      if (y < 0.001) rootA = Math.max(rootA, a);
      else if (Math.abs(pos.getX(i)) < 1e-6 && Math.abs(pos.getZ(i)) < 1e-6) spineA = Math.max(spineA, a);
      else if (y > 0.1 && y < 0.4) edgeA = Math.min(edgeA, a);
    }
    assert.equal(rootA, 0, 'the root is opaque, which draws a bar across the foot of the fire');
    assert.ok(spineA > 0.2, 'the spine is transparent, so there is no fire to see');
    assert.ok(edgeA < spineA * 0.2, 'the edges do not fade, so the blade is cut paper');
    // Every vertex keeps a color somebody could see, however clear it is drawn.
    assert.ok(dimmest > 0.5, `a tongue has a near-black vertex (${dimmest}), which rims it on a pale wall`);
  });

  it('is at its most solid low down, and gone by the tip', () => {
    assert.equal(F.alphaAt(0), 0);
    assert.equal(F.alphaAt(1), 0);
    const run = [...Array(21).keys()].map(i => F.alphaAt(i / 20));
    const peak = run.indexOf(Math.max(...run));
    assert.ok(peak > 0 && peak < 8, `a tongue is most solid at ${peak / 20} up itself`);
    assert.ok(Math.max(...run) > 0.8, 'a tongue is never solid enough to see');
  });

  it('crosses two blades of different widths', () => {
    // Equal blades put their spines through each other and the cross is what you see.
    const g = F.tongueGeometry();
    const pos = g.getAttribute('position');
    let wideX = 0, wideZ = 0;
    for (let i = 0; i < pos.count; i++) {
      wideX = Math.max(wideX, Math.abs(pos.getX(i)));
      wideZ = Math.max(wideZ, Math.abs(pos.getZ(i)));
    }
    assert.ok(wideX > 0 && wideZ > 0, 'the tongue is flat, so it is a cutout side on');
    assert.ok(Math.abs(wideX - wideZ) > 0.05, 'both blades are the same width');
  });
});

describe('the life of a tongue', () => {
  it('climbs, thins and goes out', () => {
    const start = F.climb(0), mid = F.climb(0.5), end = F.climb(0.999);
    assert.equal(start.rise, 0);
    assert.ok(mid.rise > start.rise && end.rise > mid.rise, 'a tongue does not climb');
    assert.ok(end.wide < start.wide, 'a tongue does not thin as it goes');
    assert.ok(end.light < 0.05, 'a tongue is still lit when its life is over');
    assert.ok(start.light > 0.9, 'a tongue is born dim');
  });

  it('lives over and over, without a seam', () => {
    // The phase wraps, so one tongue is a fire on its own rather than a thing that
    // happens once. Just before the wrap it must be all but out, or the fire blinks.
    const just = F.climb(0.995), after = F.climb(1.004);
    assert.ok(just.light < 0.05 && after.light > 0.95, 'the wrap is visible');
    assert.deepEqual(F.climb(2.25), F.climb(0.25), 'lives differ from one another');
  });
});

describe('the jitter', () => {
  it('gives the same fire the same shape every time', () => {
    // A relayout rebuilds everything; a fire that came back arranged differently
    // would look like a different fire on the same roof.
    assert.equal(F.wobble(7, 2), F.wobble(7, 2));
    assert.notEqual(F.wobble(7, 2), F.wobble(8, 2));
    assert.notEqual(F.wobble(7, 2), F.wobble(7, 3));
  });

  it('stays inside nought and one, so nothing it scales turns inside out', () => {
    for (let n = 0; n < 200; n++) {
      for (const salt of [0, 1, 5, 9]) {
        const v = F.wobble(n, salt);
        assert.ok(v >= 0 && v < 1, `wobble(${n}, ${salt}) is ${v}`);
      }
    }
  });
});
