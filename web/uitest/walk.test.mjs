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
const { Wind } = await import('../static/wind.js');
const { TOOLS, TOOL_IDS, PRIMARY_IDS, SECONDARY_IDS, hits, isSecondary, toolFor, idleTool, studyTool, DEFAULT_TOOL } = await import('../static/tools.js');
const SWITCH = await import('../static/switcher.js');
const THREE = await import('../static/vendor/three.module.min.js');

// How close two parts of a tool have to be to count as touching, in map units: a
// couple of millimeters at this scale, which forgives a join drawn flush and does not
// forgive a part left hanging beside the one it belongs to.
const TOUCH = 0.004;

/** Health draws into the walk HUD; none of these tests are about the drawing. */
const blind = () => new Health({ querySelector: () => null, classList: { toggle() {} } });

describe('what the walker can take', () => {
  // Verifies: REQ-WALK-027
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

  // Verifies: REQ-WALK-028
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

  // Verifies: REQ-WALK-029
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

  // Verifies: REQ-WALK-030
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

describe('mending', () => {
  it('puts a walker back together once nothing is happening to them', () => {
    const h = blind();
    h.reset(0);
    h.hurt(50);
    const at = h.hurtAt;
    // Straight away, nothing: being bitten and then standing still is not a cure.
    assert.equal(h.mend(1, at + 500), 0, 'a walker mended while still being hurt');
    assert.equal(h.hp, h.max - 50);
    // After the wait, steadily.
    assert.ok(h.mend(1, at + 8000) > 0, 'a walker never mends');
    assert.ok(h.hp > h.max - 50);
  });

  it('stops at the ceiling and never lifts it', () => {
    const h = blind();
    h.reset(0);
    const ceiling = h.max;
    h.hurt(5);
    for (let i = 0; i < 200; i++) h.mend(1, h.hurtAt + 60000);
    assert.equal(h.hp, ceiling, 'mending overshot or fell short of the ceiling');
    assert.equal(h.max, ceiling, 'mending raised the ceiling, which is the backpack\'s job');
  });

  it('does nothing for the dead', () => {
    const h = blind();
    h.reset(0);
    h.hurt(h.max);
    assert.ok(h.dead);
    assert.equal(h.mend(10, h.hurtAt + 60000), 0, 'a dead walker mended');
    assert.equal(h.hp, 0);
  });

  it('is slow enough that a bad street still costs something', () => {
    // A walk is meant to be survivable, not free: the worst bite must take longer to
    // mend than the bug that gave it takes to come round again and give another.
    const h = blind();
    h.reset(0);
    const bite = Health.biteFor('critical');
    h.hurt(bite);
    let seconds = 0;
    while (h.hp < h.max && seconds < 120) { h.mend(1, h.hurtAt + 60000); seconds++; }
    assert.ok(seconds >= 5, `a critical bite mends in ${seconds}s, which costs nothing`);
    assert.ok(seconds <= 30, `a critical bite takes ${seconds}s to mend, which is a wait`);
  });
});

describe('what running costs', () => {
  /** Wind draws into the walk HUD; none of these tests are about the drawing. */
  const blown = () => new Wind({ querySelector: () => null });

  // Verifies: REQ-WALK-038
  it('runs out after a dash and not after a stride', () => {
    const w = blown();
    w.reset();
    for (let i = 0; i < 20; i++) w.breathe(0.05, true); // a second of it
    assert.ok(w.ready, 'one second of running finished the walker');
    assert.ok(w.share < 1, 'a second of running cost nothing');
    for (let i = 0; i < 400 && w.ready; i++) w.breathe(0.05, true);
    assert.ok(!w.ready, 'a walker could sprint for twenty seconds');
    assert.equal(w.share, 0);
  });

  // Verifies: REQ-WALK-039
  it('holds a winded walker to a walk until they have their breath back', () => {
    const w = blown();
    w.reset();
    while (w.ready) w.breathe(0.05, true);
    // One frame of standing about is not a second wind: without the threshold this is
    // where a walker at nought sprints again, a frame at a time, for ever.
    w.breathe(0.05, false);
    assert.ok(!w.ready, 'one frame of recovery bought another sprint');
    assert.ok(!w.spend(Wind.jumpCost), 'a winded walker jumped');
    for (let i = 0; i < 400 && !w.ready; i++) w.breathe(0.05, false);
    assert.ok(w.ready, 'the walker never got their breath back');
    assert.ok(w.spend(Wind.jumpCost), 'a rested walker could not jump');
  });

  // Verifies: REQ-WALK-038
  it('charges a jump, and lets go of a few of them before it is spent', () => {
    const w = blown();
    w.reset();
    let jumps = 0;
    while (w.spend(Wind.jumpCost) && jumps < 100) jumps++;
    assert.ok(jumps >= 4, `a rested walker managed ${jumps} jumps`);
    assert.ok(jumps <= 12, `a rested walker managed ${jumps} jumps, which is a staircase`);
  });

  // Verifies: REQ-WALK-038
  it('fills again, and no further', () => {
    const w = blown();
    w.reset();
    for (let i = 0; i < 40; i++) w.breathe(0.05, true);
    for (let i = 0; i < 400; i++) w.breathe(0.05, false);
    assert.equal(w.share, 1, 'standing about did not fill it, or filled it past full');
    assert.ok(!w.spent);
  });
});

describe('the two kinds of tool', () => {
  // Verifies: REQ-TOOL-021
  it('gives every tool a kind and its own slot', () => {
    const slots = new Set();
    for (const id of TOOL_IDS) {
      const tool = toolFor(id);
      assert.ok(tool.kind === 'primary' || tool.kind === 'secondary', `${id} has no kind`);
      assert.ok(!slots.has(tool.slot), `two tools in slot ${tool.slot}`);
      slots.add(tool.slot);
    }
    assert.equal(TOOL_IDS.length, 10);
    const kinds = TOOL_IDS.map(id => toolFor(id).kind);
    assert.deepEqual(kinds, [...kinds].sort((a, b) => (a === 'primary' ? 0 : 1) - (b === 'primary' ? 0 : 1)));
    assert.equal(toolFor(DEFAULT_TOOL).kind, 'primary', 'walk mode opens with something that carries you');
  });

  // Verifies: REQ-TOOL-022
  it('lets no secondary tool hit anything', () => {
    for (const id of SECONDARY_IDS) {
      const tool = toolFor(id);
      assert.ok(isSecondary(tool));
      assert.equal(hits(tool, 'bugs'), false, `${id} catches bugs`);
      assert.equal(hits(tool, 'buildings'), false, `${id} tags buildings`);
    }
  });

  // Verifies: REQ-TOOL-021
  it('splits the row into the hand each tool goes in', () => {
    assert.deepEqual([...PRIMARY_IDS, ...SECONDARY_IDS], TOOL_IDS, 'the rows do not make up the row');
    assert.equal(PRIMARY_IDS.length, 7);
    assert.equal(SECONDARY_IDS.length, 3);
    for (const id of PRIMARY_IDS) assert.equal(isSecondary(toolFor(id)), false);
  });

  // Verifies: REQ-TOOL-026
  it('leaves three ways to catch a bug on purpose and two by the way', () => {
    const catchers = TOOL_IDS.filter(id => hits(toolFor(id), 'bugs'));
    for (const id of ['net', 'bubbles', 'extinguisher', 'rod', 'nailer', 'camera']) {
      assert.ok(catchers.includes(id), `${id} cannot catch a bug`);
    }
    assert.equal(hits(TOOLS.dart, 'bugs'), false, 'the tracking dart is for buildings');
  });

  // Verifies: REQ-TOOL-047
  it('puts a tank on what carries the walker, and on nothing else', () => {
    // A line does not run out; a jet and a pair of floats do, or flying is simply the
    // way you get about and the walk has no shape to it.
    for (const id of TOOL_IDS) {
      const tool = toolFor(id);
      if (!tool.fuel) continue;
      assert.ok(tool.flies || tool.floats, `${id} has a tank and nothing to spend it on`);
      assert.ok(tool.fuel.full > 5, `${id} runs out before it is any use`);
      assert.ok(tool.fuel.fills >= tool.fuel.full, `${id} fills faster than it empties`);
    }
    assert.ok(TOOLS.jetpack.fuel && TOOLS.skimmers.fuel, 'the two that hold the walker up should both run out');
    assert.ok(!TOOLS.grapple.fuel, 'a line does not run out');
  });

  // Verifies: REQ-TOOL-023, REQ-TOOL-025
  it('flies and floats on a carried tool and on nothing else', () => {
    assert.deepEqual(TOOL_IDS.filter(id => toolFor(id).flies), ['jetpack']);
    assert.deepEqual(TOOL_IDS.filter(id => toolFor(id).floats), ['skimmers']);
  });

  // Verifies: REQ-TOOL-027, REQ-TOOL-041
  it('makes the dart and the nail gun opposites rather than near-copies', () => {
    const dart = TOOLS.dart.flight, nail = TOOLS.nailer.flight;
    assert.ok(nail.speed > dart.speed * 2, 'a nail is not markedly faster than a dart');
    assert.ok(dart.arc > nail.arc * 10, 'a dart is not markedly more lobbed than a nail');
    assert.ok(dart.track > 0 && !nail.track, 'only the tracking dart tracks');
    assert.ok(nail.spread > 0 && !dart.spread, 'only the nail gun scatters');
    // One is a marksman's tool and the other a hose, which is the whole of the
    // difference: the dart out-ranges the nailer several times over and goes off once
    // per click, the nailer keeps going while the trigger is held.
    assert.ok(TOOLS.dart.reach > 3 * TOOLS.nailer.reach, 'the dart barely out-ranges the nailer');
    assert.ok(!TOOLS.dart.auto, 'the dart should be one aimed shot at a time');
    assert.ok(TOOLS.nailer.auto > 0, 'the nail gun should keep firing while held');
  });

  // Verifies: REQ-TOOL-043
  it('gives a cadence only to the tools that hose rather than aim', () => {
    const held = TOOL_IDS.filter(id => toolFor(id).auto);
    assert.deepEqual(held.sort(), ['extinguisher', 'nailer']);
    for (const id of held) {
      const tool = toolFor(id);
      assert.ok(tool.auto >= 0.08 && tool.auto <= 0.3, `${id} fires at an absurd rate`);
      assert.ok(tool.projectile, `${id} has a cadence but throws nothing`);
    }
  });

  // Verifies: REQ-HUNT-037
  it('keeps what it is pointed at with one tool, and that one is the camera', () => {
    // `keeps` is what makes a use of a tool produce a photograph, and walk.js asks no
    // tool anything else about it. Two tools answering to it would mean two tools
    // filling the stash, which is not what either of them is for.
    const keepers = TOOL_IDS.filter(id => toolFor(id).keeps);
    assert.deepEqual(keepers, ['camera']);
    // ... and it is a tool that can be pointed at either kind of thing, since a
    // photograph of a bug is as good as a photograph of a wall.
    assert.equal(TOOLS.camera.targets, 'both');
  });

  // Verifies: REQ-TOOL-037
  it('builds each tool as one thing, with nothing floating beside it', () => {
    // A viewmodel is a few dozen boxes and cylinders placed by hand, and the failure
    // they have is always the same: a part is put at an angle with a length of its
    // own, the far end lands somewhere nobody measured, and the tool is carried about
    // with a piece hanging in the air next to it. Touching is enough - every part has
    // to reach some other part, and through them the one the hand is closed around.
    for (const id of TOOL_IDS) {
      const vm = TOOLS[id].viewmodel();
      vm.updateMatrixWorld(true);
      const parts = [];
      vm.traverse(o => {
        if (!o.isMesh || !o.geometry?.attributes?.position) return;
        o.geometry.computeBoundingBox();
        parts.push({
          name: o.name || o.geometry.type,
          box: o.geometry.boundingBox.clone().applyMatrix4(o.matrixWorld),
        });
      });
      assert.ok(parts.length > 2, `${id} is not much of a tool`);
      // Breadth-first from the part the tool is built around, over "these two touch".
      const touching = (a, b) => a.box.clone().expandByScalar(TOUCH).intersectsBox(b.box);
      const joined = new Set([0]), queue = [0];
      while (queue.length) {
        const i = queue.pop();
        for (let j = 0; j < parts.length; j++) {
          if (joined.has(j) || !touching(parts[i], parts[j])) continue;
          joined.add(j);
          queue.push(j);
        }
      }
      const loose = parts.filter((_, i) => !joined.has(i)).map(p => p.name);
      assert.deepEqual(loose, [], `${id} carries ${loose.length} part(s) that touch nothing: ${loose.join(', ')}`);
    }
  });

  // Verifies: REQ-TOOL-020, REQ-WALK-042
  it('keeps the walk cycle of a held tool steady when the pace changes', () => {
    // The cycle used to be read off the clock as a rate times the elapsed time, which
    // meant a change of rate re-scaled the whole of that time and threw the phase
    // hundreds of radians forward in one frame. It only showed up on a session that
    // had been open a while - which is every session by the time anybody runs.
    const vm = TOOLS.rod.viewmodel();
    let now = 3600 * 1000, was = null, jumped = 0;
    for (let i = 0; i < 400; i++) {
      now += 16;
      idleTool(vm, now, Math.min(1.5, i / 100), 0.016); // easing from a stand into a run
      if (was !== null) jumped = Math.max(jumped, Math.abs(vm.rotation.z - was));
      was = vm.rotation.z;
    }
    assert.ok(jumped < 0.02, `the tool swung ${jumped.toFixed(3)} radians between two frames`);
  });

  // Verifies: REQ-HUNT-046
  it('fits a photograph to the shape of the camera screen, not the picture', () => {
    // A photograph is the shape of the window it was taken through and the screen on
    // the back of the camera is squarer than that, so it is filled to the screen and
    // cropped. Squashing it instead is the one thing a picture must not do.
    const screen = 0.135 / 0.097;
    for (const [w, h] of [[1600, 900], [900, 1600], [1000, 723]]) {
      const vm = TOOLS.camera.viewmodel();
      const shot = new THREE.Texture();
      shot.image = { width: w, height: h };
      TOOLS.camera.shows(vm, shot);
      assert.equal(vm.userData.photo, shot);
      assert.ok(shot.repeat.x <= 1 && shot.repeat.y <= 1, 'the picture was tiled rather than cropped');
      const shown = (w * shot.repeat.x) / (h * shot.repeat.y);
      assert.ok(Math.abs(shown - screen) < 1e-6, `${w}x${h} came out at ${shown.toFixed(3)}, not ${screen.toFixed(3)}`);
    }
  });

  // Verifies: REQ-HUNT-047
  it('draws no second pass over the map while a photograph is up', () => {
    // The live view on the camera's back is the whole map rendered again. A still
    // picture is not, and asking for one while a photograph is up would be a frame's
    // work thrown away every other frame.
    const vm = TOOLS.camera.viewmodel();
    let films = 0;
    const scene = { film: () => { films++; return new THREE.Texture(); } };
    TOOLS.camera.live(vm, scene, null);
    assert.equal(films, 1, 'the live view was not drawn');
    const shot = new THREE.Texture();
    shot.image = { width: 8, height: 6 };
    TOOLS.camera.shows(vm, shot);
    TOOLS.camera.live(vm, scene, null);
    assert.equal(films, 1, 'the map was drawn again for a screen showing a photograph');
    assert.equal(vm.getObjectByName('screen').material.map, shot);
    // ... and taking it off puts the live view back.
    TOOLS.camera.shows(vm, null);
    TOOLS.camera.live(vm, scene, null);
    assert.equal(films, 2, 'the live view never came back');
  });

  // Verifies: REQ-HUNT-043
  it('brings the camera up until the picture is nearly all you can see', () => {
    // What "held up to look at" has to come to: the screen square on to the eye,
    // centred on the line of sight, filling most of the view - and no part of the tool
    // nearer than the screen itself, or the near plane cuts a hole in the picture.
    // These follow from a position and a rotation solved once and written down, so a
    // nudge to either is worth catching here rather than in the street.
    const VIEW_NEAR = 0.5, FOV = 70, NEAR = 0.02, ASPECT = 16 / 9;
    const halfV = Math.tan(((FOV / 2) * Math.PI) / 180), halfH = halfV * ASPECT;

    const held = new THREE.Group(); // walk.js hangs the tool in one of these
    held.scale.setScalar(VIEW_NEAR);
    const vm = TOOLS.camera.viewmodel();
    vm.userData.restY = vm.position.y;
    held.add(vm);
    studyTool(vm, 1);
    held.updateMatrixWorld(true);

    const screen = vm.getObjectByName('screen');
    screen.geometry.computeBoundingBox();
    const b = screen.geometry.boundingBox;
    const corners = [];
    for (const x of [b.min.x, b.max.x]) {
      for (const y of [b.min.y, b.max.y]) {
        corners.push(new THREE.Vector3(x, y, b.min.z).applyMatrix4(screen.matrixWorld));
      }
    }
    const mid = corners
      .reduce((a, c) => a.add(c), new THREE.Vector3())
      .multiplyScalar(1 / corners.length);
    const adrift = Math.hypot(mid.x, mid.y);
    assert.ok(adrift < 0.004, `the picture sits ${adrift.toFixed(3)} off the line of sight`);

    // Square on: its normal within a couple of degrees of looking straight back.
    const facing = new THREE.Vector3(0, 0, 1).transformDirection(screen.matrixWorld);
    const off = (Math.acos(Math.min(1, facing.z)) * 180) / Math.PI;
    assert.ok(off < 3, `the picture is turned ${off.toFixed(0)} degrees away from the eye`);

    const high = Math.max(...corners.map(c => Math.abs(c.y) / (-c.z * halfV)));
    const wide = Math.max(...corners.map(c => Math.abs(c.x) / (-c.z * halfH)));
    assert.ok(high > 0.8 && high <= 1, `the picture covers ${(high * 100).toFixed(0)}% of the view's height`);
    assert.ok(wide > 0.55 && wide <= 1, `the picture covers ${(wide * 100).toFixed(0)}% of the view's width`);

    let near = Infinity;
    vm.traverse(o => {
      if (!o.isMesh || !o.geometry?.attributes?.position) return;
      o.geometry.computeBoundingBox();
      near = Math.min(near, -o.geometry.boundingBox.clone().applyMatrix4(o.matrixWorld).max.z);
    });
    assert.ok(near > NEAR * 1.4, `something is ${near.toFixed(3)} from the eye, against a near plane of ${NEAR}`);

    // ... and none of it applies until it is asked for: at rest the tool is untouched.
    const rest = TOOLS.camera.viewmodel();
    const was = rest.position.clone();
    studyTool(rest, 0);
    assert.deepEqual(rest.position.toArray(), was.toArray());
  });

  // Verifies: REQ-TOOL-039
  it('builds everything it throws out of unlit parts', () => {
    // What leaves the hand ends up in the scene, and the scene has no lights in it at
    // all - so a lit material out there is drawn black, which is what a soap bubble and
    // a gout of white foam made impossible to miss.
    const scene = { bendable: m => m };
    for (const id of TOOL_IDS) {
      const tool = toolFor(id);
      if (!tool.projectile) continue;
      tool.projectile(scene).traverse(o => {
        if (o.material) assert.equal(o.material.type, 'MeshBasicMaterial', `${id} throws something lit`);
      });
    }
  });
});

// Changing hands. The wheel is a circle of wedges and a cursor, and both halves of
// that are arithmetic: which wedge an angle falls in, and which tool a ring steps to.
// Neither needs a browser, and both are what a walker feels when they flick the mouse
// - so a wedge that is off by one is a tool taken by mistake in the middle of a chase.
describe('the tool switcher', () => {
  // Verifies: REQ-TOOL-054, REQ-TOOL-055
  it('numbers the row it draws, left to right', () => {
    // The fix this all exists for. The row is laid out with the carried tools on the
    // left, because that is the hand they go in, and the digits are counted along
    // that same list - so the row reads 1 to 0 from left to right rather than
    // 8, 9, 0, 1, 2, which is what assigning them by kind produced.
    const row = SWITCH.rowOrder();
    assert.deepEqual(row, [...SECONDARY_IDS, ...PRIMARY_IDS], 'the row is not laid out by hand');
    assert.deepEqual(row.map(SWITCH.keyFor), ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']);
    assert.deepEqual(SECONDARY_IDS.map(SWITCH.keyFor), ['1', '2', '3'], 'the left hand is not 1, 2, 3');
    assert.deepEqual(PRIMARY_IDS.map(SWITCH.keyFor), ['4', '5', '6', '7', '8', '9', '0']);

    // And every digit gets back the tool whose slot wears it.
    for (const id of row) assert.equal(SWITCH.toolForKey(`Digit${SWITCH.keyFor(id)}`), id);
    // Ten slots is exactly ten digits, which is the only reason the numbering can be
    // this simple; an eleventh tool would take a digit another one already wears.
    assert.equal(new Set(row.map(SWITCH.keyFor)).size, TOOL_IDS.length, 'two tools share a digit');
    assert.equal(row.length, 10, 'ten slots no longer fit the ten digits');

    // Nothing that is not a digit key picks a tool. 'KeyQ'.slice(5) is an empty
    // string and +'' is nought, so an unguarded lookup answers Q with the 0 key -
    // which would have put a nail gun in the hunting hand instead of walking the
    // carried row, and only while somebody was reaching for something else.
    for (const code of ['KeyQ', 'KeyE', 'KeyR', 'KeyW', 'Escape', 'Enter', 'Space', 'Digit', 'DigitX']) {
      assert.equal(SWITCH.toolForKey(code), undefined, `${code} picked a tool`);
    }

    // A carried tool answers to its digit and to Q, its digit first because that is
    // what its slot shows.
    assert.deepEqual(SWITCH.keysFor(SECONDARY_IDS[0]), ['1', 'Q']);
    assert.deepEqual(SWITCH.keysFor(PRIMARY_IDS[0]), ['4']);
  });

  // Verifies: REQ-TOOL-056
  it('cycles the off hand back to an empty one', () => {
    const ring = SWITCH.carriedRing();
    assert.deepEqual(ring, [...SECONDARY_IDS, SWITCH.EMPTY], 'nothing is not on the ring');
    // Four presses from empty come back to empty, which is what makes Q safe to lean
    // on: whatever it did, pressing it round again undoes it.
    let at = SWITCH.EMPTY;
    const seen = [];
    for (let i = 0; i < ring.length; i++) { at = SWITCH.cycle(ring, at); seen.push(at); }
    assert.deepEqual(seen, ring);
    assert.equal(at, SWITCH.EMPTY);
    // And backwards, for a ring that is walked the other way.
    assert.equal(SWITCH.cycle(ring, SWITCH.EMPTY, -1), SECONDARY_IDS[SECONDARY_IDS.length - 1]);
  });

  // Verifies: REQ-TOOL-057
  it('cycles the hunting hand and never empties it', () => {
    let at = PRIMARY_IDS[0];
    for (let i = 0; i < PRIMARY_IDS.length; i++) {
      at = SWITCH.cycle(PRIMARY_IDS, at);
      assert.ok(PRIMARY_IDS.includes(at), 'E left the hunting hand holding nothing');
    }
    assert.equal(at, PRIMARY_IDS[0], 'the hunt does not come round');
  });

  // Verifies: REQ-TOOL-058
  it('lays the wheel out the way the walker is', () => {
    const w = SWITCH.wedges();
    assert.equal(w.length, TOOL_IDS.length + 1, 'the wheel has no wedge for an empty hand');
    // Every wedge is somewhere, once, and the two halves are whole: the hunt owns the
    // right, what carries you owns the left, and there is no gap between them.
    for (const id of [...TOOL_IDS, SWITCH.EMPTY]) {
      assert.equal(w.filter(x => x.id === id).length, 1, `${id} is on the wheel ${w.filter(x => x.id === id).length} times`);
    }
    const right = w.filter(x => x.from < 180), left = w.filter(x => x.from >= 180);
    assert.deepEqual(right.map(x => x.id), PRIMARY_IDS, 'the hunt is not down the right side');
    assert.deepEqual(left.map(x => x.id).sort(), SWITCH.carriedRing().slice().sort());
    assert.equal(Math.min(...right.map(x => x.from)), 0);
    assert.equal(Math.max(...right.map(x => x.to)), 180);
    assert.equal(Math.min(...left.map(x => x.from)), 180);
    assert.equal(Math.max(...left.map(x => x.to)), 360);
  });

  // Verifies: REQ-TOOL-060
  it('points at what the cursor is over, and at nothing in the middle', () => {
    // The hub: a wheel opened and let go without moving the mouse changes no hands.
    assert.equal(SWITCH.wedgeAt(0, 0), null);
    assert.equal(SWITCH.wedgeAt(SWITCH.HUB - 1, 0), null, 'the dead zone is not the hub');
    assert.ok(SWITCH.wedgeAt(SWITCH.HUB + 2, 0), 'the ring starts somewhere else than the hub ends');
    // Straight up is the first of the hunt, straight down-left is the bare hand, and
    // the middle of every wedge is the wedge it is the middle of.
    assert.equal(SWITCH.wedgeAt(0, -120).id, PRIMARY_IDS[0]);
    assert.equal(SWITCH.wedgeAt(120, 0).id, PRIMARY_IDS[Math.floor(PRIMARY_IDS.length / 2)]);
    for (const wedge of SWITCH.wedges()) {
      const { x, y } = SWITCH.seatOf(wedge);
      assert.equal(SWITCH.wedgeAt(x, y)?.id, wedge.id, `the middle of ${wedge.id} points at something else`);
    }
    // All the way round, every angle is on exactly one wedge and never on none.
    for (let a = 0; a < 360; a += 0.5) {
      const r = (a - 90) * Math.PI / 180;
      assert.ok(SWITCH.wedgeAt(Math.cos(r) * SWITCH.RING, Math.sin(r) * SWITCH.RING), `${a} degrees is on nothing`);
    }
  });
});

