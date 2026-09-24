// What a finding looks like on the street.
//
// Severity is carried by color, and a color is the one thing that says nothing in a
// crowd, in the dark or from behind - which is most of walk mode. So it is carried by
// shape as well, and that only works if the shapes really are different meshes and the
// worst of them really is the one that cannot fly away.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box, scene } from './stub.mjs';

const { Bugs } = await import('../static/bugs.js');

/**
 * A tower with `findings` reported against it, as findings.js hands them over: the
 * whole list, and the node each one belongs to.
 */
function reported(severities) {
  const tower = box('building', 0, 0, 1, 1, { y: 0.2, h: 4 });
  const all = severities.map((severity, i) => ({ id: `f${i}`, severity, title: `${severity} ${i}` }));
  return { boxes: [box('land', 0, 0, 12, 12, { y: -0.45, h: 0.45 }), tower], index: { all, place: () => tower.node } };
}

/** The bugs placed for those severities. */
function placed(severities) {
  const { boxes, index } = reported(severities);
  const bugs = new Bugs(scene());
  bugs.place(index, boxes);
  return bugs;
}

describe('a bug', () => {
  it('walks as the shape its severity calls for', () => {
    const bugs = placed(['critical', 'high', 'medium', 'low', 'info']);
    const shapes = new Map(bugs.bugs.map(b => [b.f.severity, b]));
    assert.equal(shapes.get('critical').shape, 'grub');
    assert.equal(shapes.get('high').shape, 'beetle');
    assert.equal(shapes.get('medium').shape, 'beetle');
    assert.equal(shapes.get('low').shape, 'mite');
    assert.equal(shapes.get('info').shape, 'mite');
    // Size carries it too, so the same shape at two severities is still told apart.
    assert.ok(shapes.get('high').scale > shapes.get('medium').scale);
    assert.ok(shapes.get('info').scale < shapes.get('low').scale);
  });

  it('gives the worst of them nowhere to fly to', () => {
    // Enough of them that the deal reaches every surface several times over: if a
    // caterpillar could be dealt the air, this is where it would happen.
    const bugs = placed(Array.from({ length: 24 }, () => 'critical'));
    assert.ok(bugs.bugs.length > 0);
    for (const bug of bugs.bugs) {
      assert.equal(bug.flying, false, 'a caterpillar took to the air');
      assert.notEqual(bug.lap.kind, 'air');
    }
  });

  it('draws one set of meshes per shape and no more', () => {
    const one = placed(['medium', 'medium', 'high']);
    assert.equal(one.drawn.length, 1, 'one shape should need one set of meshes');
    assert.equal(one.drawn[0].bugs.length, 3);

    const three = placed(['critical', 'medium', 'info']);
    assert.equal(three.drawn.length, 3);
    const geometries = new Set(three.drawn.map(d => d.shell.geometry));
    assert.equal(geometries.size, 3, 'two shapes were drawn from the same mesh');
    // A shape that never flies is not given wings to keep matrices for.
    const grub = three.drawn.find(d => d.bugs[0].shape === 'grub');
    assert.equal(grub.wings, null);
  });

  it('still remembers what was caught when the map is rebuilt under it', () => {
    const { boxes, index } = reported(['critical', 'low']);
    const bugs = new Bugs(scene());
    bugs.place(index, boxes);
    assert.ok(bugs.catch(bugs.bugs[0]));
    assert.equal(bugs.counts.caught, 1);
    bugs.place(index, boxes);
    assert.equal(bugs.counts.caught, 1, 'a relayout revived a bug that was caught');
  });
});
