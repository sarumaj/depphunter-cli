// What the walk costs, and what it is carried out with.
//
// Two rules decide whether walk mode is a game or a nuisance, and neither of them is
// visible in a screenshot. One is what a fall and a bite are worth: too much and the
// map is lost to a misjudged step, too little and there is no reason to swing at
// anything. The other is the line between the tools that hunt and the tools that carry
// you - a secondary tool that could tag a building would make the division pointless,
// and one the crosshair ignored entirely could not be aimed at the wall it is meant to
// pull you up.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { Health } = await import('../static/health.js');
const { TOOLS, TOOL_IDS, hits, marks, isSecondary, toolFor, DEFAULT_TOOL } = await import('../static/tools.js');

/** Health draws into the walk HUD; none of these tests are about the drawing. */
const blind = () => new Health({ querySelector: () => null, classList: { toggle() {} } });

describe('what the walker can take', () => {
  it('ignores a short drop and kills on a long one', () => {
    const h = blind();
    h.reset(0);
    assert.equal(h.fall(2), 0, 'a drop off a terrace wall cost something');
    assert.equal(h.hp, h.max);
    assert.ok(h.fall(6) > 0, 'a drop off a roof cost nothing');
    assert.ok(!h.dead, 'one roof finished the walk');

    const far = blind();
    far.reset(0);
    far.fall(40);
    assert.ok(far.dead, 'a drop off the tallest building was survived');
  });

  it('charges a bite by severity, and takes several of any of them', () => {
    const worst = Health.biteFor('critical');
    assert.ok(worst > Health.biteFor('high'));
    assert.ok(Health.biteFor('high') > Health.biteFor('medium'));
    assert.ok(Health.biteFor('medium') > Health.biteFor('info'));
    assert.equal(Health.biteFor('nonsense'), Health.biteFor('unknown'), 'an unknown severity has no price');

    const h = blind();
    h.reset(0);
    let bites = 0;
    while (!h.dead && bites < 20) { h.bite('critical'); bites++; }
    assert.ok(bites >= 3, `the worst bug on the map killed in ${bites}`);
  });

  it('raises the ceiling and mends by as much for every bug in the backpack', () => {
    const h = blind();
    h.reset(0);
    const bare = h.max;
    h.bite('high');
    const hurt = h.hp;
    h.caught(1);
    assert.ok(h.max > bare, 'catching one did not raise the bar');
    assert.equal(h.hp, hurt + (h.max - bare), 'catching one did not mend anything');
    assert.ok(h.hp <= h.max);

    // The ceiling stops somewhere: a full backpack is an advantage, not an ending.
    const full = blind();
    full.reset(10000);
    assert.ok(full.max < 10 * bare);
  });

  it('starts every walk whole, however the last one ended', () => {
    const h = blind();
    h.reset(3);
    h.hurt(h.max);
    assert.ok(h.dead);
    h.reset(3);
    assert.ok(!h.dead);
    assert.equal(h.hp, h.max);
  });
});

describe('the two kinds of tool', () => {
  it('gives every tool a kind and its own slot', () => {
    const slots = new Set();
    for (const id of TOOL_IDS) {
      const tool = toolFor(id);
      assert.ok(tool.kind === 'primary' || tool.kind === 'secondary', `${id} has no kind`);
      assert.ok(!slots.has(tool.slot), `two tools in slot ${tool.slot}`);
      slots.add(tool.slot);
    }
    // Ten slots, which is what 1 to 9 and 0 come to; the primary ones come first, so
    // the row reads as the hunt and then what carries you.
    assert.equal(TOOL_IDS.length, 10);
    const kinds = TOOL_IDS.map(id => toolFor(id).kind);
    assert.deepEqual(kinds, [...kinds].sort((a, b) => (a === 'primary' ? 0 : 1) - (b === 'primary' ? 0 : 1)));
    assert.equal(toolFor(DEFAULT_TOOL).kind, 'primary', 'walk mode opens with something that carries you');
  });

  it('lets no secondary tool hit anything, and still lets a line be aimed', () => {
    for (const id of TOOL_IDS) {
      const tool = toolFor(id);
      if (!isSecondary(tool)) continue;
      assert.equal(hits(tool, 'bugs'), false, `${id} catches bugs`);
      assert.equal(hits(tool, 'buildings'), false, `${id} tags buildings`);
    }
    // The grapple pulls you to a wall, so the crosshair has to say which wall - and
    // marking one is still not hitting it.
    assert.equal(marks(TOOLS.grapple, 'buildings'), true);
    assert.equal(marks(TOOLS.jetpack, 'buildings'), false);
    assert.equal(marks(TOOLS.skimmers, 'bugs'), false);
  });

  it('leaves three ways to catch a bug on purpose and two by the way', () => {
    const catchers = TOOL_IDS.filter(id => hits(toolFor(id), 'bugs'));
    for (const id of ['net', 'bubbles', 'extinguisher', 'rod', 'nailer', 'camera']) {
      assert.ok(catchers.includes(id), `${id} cannot catch a bug`);
    }
    assert.equal(hits(TOOLS.dart, 'bugs'), false, 'the tracking dart is for buildings');
  });

  it('flies and floats on a carried tool and on nothing else', () => {
    assert.deepEqual(TOOL_IDS.filter(id => toolFor(id).flies), ['jetpack']);
    assert.deepEqual(TOOL_IDS.filter(id => toolFor(id).floats), ['skimmers']);
  });

  it('flies a dart and a nail by different physics', () => {
    const dart = TOOLS.dart.flight, nail = TOOLS.nailer.flight;
    assert.ok(nail.speed > dart.speed * 2, 'a nail is not markedly faster than a dart');
    assert.ok(dart.arc > nail.arc * 10, 'a dart is not markedly more lobbed than a nail');
    assert.ok(dart.track > 0 && !nail.track, 'only the tracking dart tracks');
    assert.ok(nail.spread > 0 && !dart.spread, 'only the nail gun scatters');
    assert.ok(TOOLS.dart.reach > TOOLS.nailer.reach, 'the dart should out-range the nailer');
  });
});
