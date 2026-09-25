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
const TOOLS_MOD = await import('../static/tools.js');

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
  // Verifies: REQ-HUNT-012
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

  // Verifies: REQ-HUNT-012
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

  // Verifies: REQ-HUNT-030
  it('leaves the map the way the tool that caught it took it', () => {
    const { TOOLS, TOOL_IDS, hits, toolFor } = TOOLS_MOD;
    // Every tool that can catch one says how it takes it; a catch with no gesture
    // named - a finding taken from the panel, over on the map - is still a catch.
    for (const id of TOOL_IDS) {
      const tool = toolFor(id);
      if (hits(tool, 'bugs')) assert.ok(tool.catchAs, `${id} catches bugs and says nothing about how`);
      else assert.ok(!tool.catchAs, `${id} cannot catch a bug but says how it would`);
    }

    // A bubbled one rises; a reeled one comes to the walker; a pinned one is driven
    // down onto what it was standing on. All of them end up gone.
    const at = { x: 3, y: 1, z: 3 };
    const paths = {};
    for (const how of ['bubble', 'reel', 'pin', 'net', 'foam', 'flash']) {
      const bugs = placed(['medium']);
      const [bug] = bugs.bugs;
      const from = bug.pos.clone();
      assert.ok(bugs.catch(bug, how));
      assert.equal(bugs.counts.caught, 1, 'a catch with an animation did not count');
      const seen = [];
      for (let i = 0; i < 40 && bug.take; i++) {
        bugs.update(0.05, i * 50, at);
        if (bug.take) seen.push({ y: bug.pos.y - from.y, size: bug.size, to: bug.pos.distanceTo(at) });
      }
      assert.equal(bug.take, null, `a ${how} never finished; a bug would hang there for good`);
      assert.ok(seen.length > 3, `a ${how} was over before it was seen`);
      paths[how] = seen;
    }
    assert.ok(paths.bubble.at(-1).y > 0.3, 'a bubbled bug did not rise');
    assert.ok(paths.pin.at(-1).y < 0, 'a pinned bug was not driven down');
    assert.ok(paths.foam.at(-1).y < 0, 'a foamed bug did not sink');
    assert.ok(paths.reel.at(-1).to < paths.reel[0].to * 0.6, 'a reeled bug was not drawn in');
    for (const how of Object.keys(paths)) {
      assert.ok(paths[how].at(-1).size < 0.35, `a ${how} left something behind`);
    }
  });

  // Verifies: REQ-HUNT-032
  it('carries a netted one in the hoop rather than towards the walker', () => {
    // The net is the one tool that takes a bug somewhere other than to the walker: it
    // is in the hoop, and the hoop is on the end of a swing. Homing on the walker's
    // eye meant a bug that set off across the street while the net went the other way.
    const eye = { x: 4, y: 1.5, z: 4 }, hoop = { x: -5, y: 2, z: -5 };
    const bugs = placed(['medium']);
    const [bug] = bugs.bugs;
    assert.ok(bugs.catch(bug, 'net'));
    const far = bug.pos.distanceTo(hoop);
    let near = Infinity, turns = 0, above = null, going = 0;
    for (let i = 0; i < 40 && bug.take; i++) {
      bugs.update(0.05, i * 50, eye, hoop);
      if (!bug.take) break;
      near = Math.min(near, bug.pos.distanceTo(hoop));
      // It fights in there: the struggle is what makes a catch something anybody saw,
      // and a struggle is a thing that changes direction.
      const up = bug.pos.y - hoop.y;
      if (above !== null) {
        const way = Math.sign(up - above);
        if (way !== 0 && going !== 0 && way !== going) turns++;
        if (way !== 0) going = way;
      }
      above = up;
    }
    assert.ok(near < far * 0.15, `a netted bug got no nearer the hoop than ${near.toFixed(2)} of ${far.toFixed(2)}`);
    assert.ok(turns > 1, 'a netted bug went quietly');
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
