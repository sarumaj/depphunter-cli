// How many pixels a frame is drawn with, as the frame rate allows (resolution.js).

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

const { FLOOR, Resolution } = await import('../static/resolution.js');

// Feeds `count` frames `gap` milliseconds apart from `start`; returns the time after.
function run(resolution, start, gap, count) {
  let now = start;
  for (let i = 0; i < count; i++) resolution.frame(now += gap);
  return now;
}

describe('dynamic resolution', () => {
  // Verifies: REQ-PERF-011
  it('keeps full resolution while frames keep up', () => {
    const resolution = new Resolution();
    run(resolution, 0, 1000 / 60, 600);
    assert.equal(resolution.scale, 1);
  });

  // Verifies: REQ-PERF-011
  it('draws smaller while frames fall behind, never below the floor', () => {
    const resolution = new Resolution();
    const now = run(resolution, 0, 40, 60);
    assert.ok(resolution.scale < 1, 'slow frames did not shrink the picture');
    run(resolution, now, 40, 2000);
    assert.equal(resolution.scale, FLOOR);
  });

  // Verifies: REQ-PERF-011
  it('grows back once frames keep up, but not straight back to a size that was too slow', () => {
    const resolution = new Resolution();
    let now = run(resolution, 0, 40, 30);
    const tooSlow = resolution.tooSlow.scale;
    assert.ok(resolution.scale < tooSlow);
    now = run(resolution, now, 1000 / 60, 120);
    assert.ok(resolution.scale < tooSlow, 'grew back within moments to the size that was too slow');
    run(resolution, now, 1000 / 60, 600);
    assert.equal(resolution.scale, 1);
  });

  // Verifies: REQ-PERF-011
  it('takes a pause for a pause, not a slow frame', () => {
    const resolution = new Resolution();
    let now = 0;
    for (let i = 0; i < 100; i++) now = run(resolution, now + 5000, 1000 / 60, 3);
    assert.equal(resolution.scale, 1, 'a page put away');
    for (let i = 0; i < 100; i++) {
      resolution.frame(now += 500, false);
      now = run(resolution, now, 1000 / 60, 3);
    }
    assert.equal(resolution.scale, 1, 'a frame asked for long after the one before');
  });

  // Verifies: REQ-PERF-011
  it('takes a hitch for a hitch, not a slow GPU', () => {
    const resolution = new Resolution();
    let now = 0;
    for (let i = 0; i < 200; i++) {
      resolution.frame(now += 600);
      now = run(resolution, now, 1000 / 60, 4);
    }
    assert.equal(resolution.scale, 1);
  });

  // Verifies: REQ-PERF-011
  it('rests at full resolution', () => {
    const resolution = new Resolution();
    run(resolution, 0, 40, 60);
    resolution.rest();
    assert.equal(resolution.scale, 1);
  });
});
