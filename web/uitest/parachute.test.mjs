// The parachute: a canopy flown down from a roof or out of the jet.
//
// What matters about it is all numbers - how fast it comes down, how long it takes to
// open, what a flare does and what the ground costs - and every one of them comes out
// of one pure function stepped over time (parachute.js stepChute). So that is what is
// driven here, on flat ground or against a wall laid out as a function, at the frame
// rates real screens run at. The walker's own bookkeeping around it - the pack that
// has to be repacked, the canopy that is cut loose when the hands change - is driven
// through Walker's methods on a stand-in walker, the way walk.test.mjs does it.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const P = await import('../static/walk/parachute.js');
const { Health } = await import('../static/walk/health.js');
const { TOOLS } = await import('../static/walk/tools.js');
const { Avatar } = await import('../static/walk/avatar.js');
const WALK = await import('../static/walk/walk.js');
const THREE = await import('../static/vendor/three.module.min.js');

// A unit is about three and a half meters (health.js), so a speed in units a second
// is this many meters a second.
const METERS = 3.5;
const FLAT = () => 0;

/** A canopy thrown open `up` units over flat ground, falling at `vy`. */
const thrown = (up = 40, vy = 0) => P.deployChute(P.packedChute(), { x: 0, feet: up, z: 0, vy, heading: 0 });

/**
 * Flies `chute` for `seconds` at `fps` frames a second, with `input(chute)` for the
 * keys; returns every event on the way. Stops early once it is on the ground.
 */
function fly(chute, seconds, { fps = 60, input = () => ({ forward: 0, turn: 0 }), ground = FLAT } = {}) {
  const events = [];
  for (let t = 0; t < seconds && P.aloft(chute); t += 1 / fps) events.push(...P.stepChute(chute, input(chute), 1 / fps, ground));
  return events;
}

/**
 * A canopy thrown open well above `height` over flat ground and flown down to it at
 * trim, so that it arrives there in steady flight.
 */
function glidingAt(height) {
  const chute = thrown(height + 25);
  while (!(chute.phase === 'flying' && chute.feet <= height)) P.stepChute(chute, { forward: 0, turn: 0 }, 1 / 60, FLAT);
  return chute;
}

/** Flies on down to the ground from where `chute` is, and says how it arrived. */
function landing(chute, input = () => ({ forward: 0, turn: 0 })) {
  const events = fly(chute, 60, { input });
  return events.find(e => e.kind === 'touchdown');
}

/** A canopy in steady flight at 30 units, `input` held from the moment it opens. */
function steady(input) {
  const chute = thrown(60);
  fly(chute, 14, { input: () => input });
  return chute;
}

describe('a canopy in flight', () => {
  // Verifies: REQ-TOOL-073
  it('settles to a descent of under three meters a second and a glide of about 4', () => {
    const chute = steady({ forward: 0, turn: 0 });
    assert.equal(chute.phase, 'flying');
    const sink = -chute.vy * METERS, forward = Math.hypot(chute.vx, chute.vz);
    assert.ok(sink >= 2.5 && sink <= 3.2, `it sinks at ${sink.toFixed(2)} m/s`);
    const glide = forward / -chute.vy;
    assert.ok(glide >= 3.6 && glide <= 4.4, `it glides ${glide.toFixed(2)} to 1`);
    // Settled, not still settling: a second later it is doing the same thing.
    const was = chute.vy;
    fly(chute, 1);
    assert.ok(Math.abs(chute.vy - was) < 0.01, 'the descent has not settled after fourteen seconds');
  });

  // Verifies: REQ-TOOL-073
  it('trims faster and steeper on W, slower and sinking a little more on S', () => {
    const neutral = steady({ forward: 0, turn: 0 }), up = steady({ forward: 1, turn: 0 }), down = steady({ forward: -1, turn: 0 });
    const along = c => Math.hypot(c.vx, c.vz), glide = c => along(c) / -c.vy;
    assert.ok(along(up) > along(neutral) && glide(up) < glide(neutral), 'W is not faster and steeper');
    assert.ok(along(down) < along(neutral) * 0.7, 'S does not slow it down');
    assert.ok(-down.vy > -neutral.vy && -down.vy < -neutral.vy * 1.4, 'S does not sink a little more, and only a little');
  });

  // Verifies: REQ-TOOL-073
  it('turns on A and D, banking, and sinks faster for it', () => {
    const straight = steady({ forward: 0, turn: 0 });
    const left = steady({ forward: 0, turn: 1 }), right = steady({ forward: 0, turn: -1 });
    assert.ok(left.turnRate > 0.8 && right.turnRate < -0.8, 'it does not turn');
    assert.ok(left.bank > 0.1 && right.bank < -0.1, 'it turns flat');
    assert.ok(-left.vy > -straight.vy * 1.2, `a turn sinks at ${-left.vy} against ${-straight.vy} straight`);
    // ... and the walker leans out with the lines, the way the view does (walk.js).
    assert.ok(Math.abs(left.swing.roll - left.bank) < 0.1, 'the swing does not follow the bank');
  });

  // Verifies: REQ-TOOL-073
  it('flies the same line at 60 frames a second as at 144, and on an uneven clock', () => {
    // Time is counted in ticks of 1/720 of a second, which all three clocks divide
    // into, so that each of them passes every whole second and the moment the keys
    // change exactly, rather than a float's width either side of it.
    const TICK = 720;
    const at = [];
    for (const clock of [() => 12, () => 5, i => (i % 3 ? 8 : 24)]) {
      const chute = thrown(60, -6);
      const path = [];
      for (let ticks = 0, i = 0; ticks < 8 * TICK;) {
        const t = ticks / TICK, step = clock(i++);
        P.stepChute(chute, { forward: t >= 4 ? 1 : 0, turn: t >= 2 && t < 5 ? 1 : 0 }, step / TICK, FLAT);
        ticks += step;
        if (ticks % TICK === 0) path.push([chute.x, chute.feet, chute.z]);
      }
      at.push(path);
    }
    // Within one substep of travel of each other at every second of an eight-second
    // flight - a fall, the opening, a turn and a change of trim: where a frame ends
    // within a substep is the only difference left.
    const substep = 6 * P.SUBSTEP;
    for (let s = 0; s < at[0].length; s++) {
      for (const other of at.slice(1)) {
        const off = Math.hypot(...at[0][s].map((v, k) => v - other[s][k]));
        assert.ok(off < 2 * substep, `second ${s + 1}: ${off.toFixed(3)} units apart`);
      }
    }
  });
});

describe('opening', () => {
  // Verifies: REQ-TOOL-072
  it('takes the pilot chute a moment and the canopy about a second, snapping open', () => {
    const chute = thrown(60);
    let pilot = 0, inflating = 0, most = 0;
    while (chute.phase !== 'flying') {
      if (chute.phase === 'pilot') pilot += 1 / 240; else inflating += 1 / 240;
      P.stepChute(chute, { forward: 0 }, 1 / 240, FLAT);
      most = Math.max(most, P.opennessOf(chute));
    }
    assert.ok(Math.abs(pilot - P.PILOT_TIME) < 0.01, `the pilot chute took ${pilot.toFixed(2)}s`);
    assert.ok(inflating > 0.8 && inflating < 1.2, `the canopy took ${inflating.toFixed(2)}s to open`);
    assert.ok(most > 1.05, 'it eased open rather than snapping past full');
    assert.equal(P.opennessOf(chute), 1);
  });

  // Verifies: REQ-TOOL-072, REQ-TOOL-076
  it('stops a fast fall with a jolt the walker feels', () => {
    const chute = thrown(80, -15);
    let dip = 0, hardest = 0, last = chute.vy;
    for (let i = 0; i < 180; i++) {
      P.stepChute(chute, { forward: 0 }, 1 / 60, FLAT);
      hardest = Math.max(hardest, (chute.vy - last) * 60);
      last = chute.vy;
      dip = Math.min(dip, chute.swing.sag);
    }
    assert.ok(hardest > 20, `the opening shock was ${hardest.toFixed(1)} units/s², barely more than gravity`);
    assert.ok(dip < -0.04, 'the lines did not give on the opening');
    fly(chute, 10);
    assert.ok(Math.abs(chute.swing.sag) < 0.01, 'the lines stayed stretched in steady flight');
  });

  // Verifies: REQ-TOOL-072
  it('comes out of the opening sinking, not climbing, and settles in about three seconds', () => {
    // A canopy this flat turns the speed the opening leaves it with back into height
    // unless the surge is bled off (parachute.js SURGE_DRAG): then it climbs out of
    // the opening and pitches through a descent that swings the view for seconds.
    for (const vy of [0, -6, -12]) {
      const chute = thrown(200, vy);
      let highest = -Infinity, settled = 0;
      for (let t = 1 / 60; t < 8; t += 1 / 60) {
        P.stepChute(chute, { forward: 0, turn: 0 }, 1 / 60, FLAT);
        if (chute.phase === 'flying') highest = Math.max(highest, chute.vy);
        if (Math.abs(chute.vy + P.TRIM[1].sink) > 0.1) settled = t;
      }
      assert.ok(highest < 0.2, `thrown falling at ${vy}, it climbed at ${highest.toFixed(2)} out of the opening`);
      assert.ok(settled < 3.3, `thrown falling at ${vy}, the descent took ${settled.toFixed(2)}s to settle`);
    }
  });

  // Verifies: REQ-TOOL-072
  it('has not finished opening when thrown too close to the ground', () => {
    const touchdown = landing(thrown(2));
    assert.ok(touchdown, 'it never reached the ground');
    assert.ok(touchdown.open < 1, `it was ${touchdown.open} open at the ground`);
    // ... so it is judged mostly as the fall it nearly was, and it saved next to
    // nothing: within a few points of simply falling those two units.
    const h = new Health({ querySelector: () => null, classList: { toggle() {} } });
    h.reset(0);
    const cost = h.touchdown(touchdown.speed, touchdown.open, P.dropFor(touchdown.vertical));
    const fall = Health.fallShare(2) * h.max;
    assert.ok(Math.abs(cost - fall) < 0.1 * h.max, `the landing cost ${cost} where the fall would have cost ${fall}`);
  });
});

describe('the flare, and the ground', () => {
  const blind = () => {
    const h = new Health({ querySelector: () => null, classList: { toggle() {} } });
    h.reset(0);
    return h;
  };
  const flaredFrom = height => {
    const chute = glidingAt(height);
    return landing(chute, () => ({ forward: 0, turn: 0, flare: true }));
  };

  // Verifies: REQ-TOOL-074, REQ-TOOL-075
  it('comes in slower flared at the right moment, and costs nothing', () => {
    const plain = landing(glidingAt(3));
    // About two meters up, which is where a flare is meant to start.
    const flared = flaredFrom(0.6);
    assert.ok(flared.speed < plain.speed * 0.7, `a flare came in at ${flared.speed.toFixed(2)}, unflared ${plain.speed.toFixed(2)}`);
    assert.ok(flared.vertical < plain.vertical * 0.6, 'the flare did not take the sink out');
    assert.equal(blind().touchdown(flared.speed), 0, 'a good flare still cost something');
    // ... and the right moment is a window, not an instant: started anywhere from
    // under a meter to about two and a half meters up, it lands free.
    for (const height of [0.2, 0.35, 0.5, 0.7]) {
      const at = flaredFrom(height);
      assert.equal(blind().touchdown(at.speed), 0, `a flare from ${height} came in at ${at.speed.toFixed(2)}`);
    }
  });

  // Verifies: REQ-TOOL-074
  it('stalls when it is spent too high, and comes in harder than no flare at all', () => {
    const plain = landing(glidingAt(3));
    const early = flaredFrom(1.6);
    assert.ok(early.vertical > plain.vertical, `a flare spent early sank at ${early.vertical.toFixed(2)}, none at ${plain.vertical.toFixed(2)}`);
    const chute = glidingAt(3);
    fly(chute, P.FLARE_TIME + 0.2, { input: () => ({ flare: true }) });
    assert.ok(P.stalled(chute), 'it did not stall once the flare was spent');
  });

  // Verifies: REQ-TOOL-075
  it('charges a landing by how fast it arrives, within bounds', () => {
    const plain = landing(glidingAt(3));
    const dive = landing(glidingAt(3), () => ({ forward: 1, turn: 0 }));
    const cost = t => blind().touchdown(t.speed) / 100;
    // Unflared costs a little and a dive on full flight more; neither ends a walk.
    assert.ok(cost(plain) >= 0.03 && cost(plain) <= 0.08, `an unflared landing cost ${cost(plain)}`);
    assert.ok(cost(dive) > cost(plain) && cost(dive) < 0.3, `a dive cost ${cost(dive)}`);
    // What it costs is the speed, not the height it came from: the same canopy
    // brought down from ten times as high arrives the same and costs the same.
    assert.equal(cost(landing(glidingAt(30))), cost(plain));
    // At the speed a lethal fall arrives at, it is lethal.
    assert.ok(blind().touchdown(Math.sqrt(2 * 13 * 5)) >= 100, 'the fastest landing is survivable');
    const h = blind();
    h.touchdown(Math.sqrt(2 * 13 * 5));
    assert.ok(h.dead);
  });

  // Verifies: REQ-TOOL-075
  it('stops at a wall, and charges for it', () => {
    // A wall two units ahead of where the canopy is flying, taller than it is high.
    const chute = glidingAt(4);
    const wall = chute.z - 2;
    const ground = (x, z) => (z <= wall ? 20 : 0);
    const events = fly(chute, 3, { ground });
    const hit = events.find(e => e.kind === 'wall');
    assert.ok(hit, 'it flew through the wall');
    assert.ok(chute.z > wall, 'it went on through the wall');
    assert.ok(blind().touchdown(hit.speed) > 10, `flying into a wall at ${hit.speed.toFixed(2)} cost next to nothing`);
  });
});

describe('the pack', () => {
  /** A walker as far as its tanks and its parachute go. */
  function walker() {
    const W = WALK.Walker.prototype;
    return {
      tanks: new Map(), dry: new Set(), secondary: TOOLS.parachute, chute: P.packedChute(), said: [],
      flash(m) { this.said.push(m); }, quip() {}, tank: W.tank, spend: W.spend, burn: W.burn,
    };
  }

  // Verifies: REQ-TOOL-077
  it('is spent whole when thrown and repacks only on the ground, and only all the way', () => {
    const w = walker();
    assert.equal(w.tank(TOOLS.parachute), 1, 'a parachute starts packed');
    w.spend(TOOLS.parachute);
    P.deployChute(w.chute, { x: 0, feet: 20, z: 0 });
    // In the air it stays empty, however long the descent.
    for (let t = 0; t < 30; t += 0.05) w.burn(0.05, true);
    assert.equal(w.tank(TOOLS.parachute), 0, 'it refilled while it was open');
    // On the ground it repacks over `fills` seconds, and is not ready until it has.
    w.chute.phase = 'landed';
    let t = 0;
    while (w.dry.has('parachute') && t < 60) { w.burn(0.05, false); t += 0.05; }
    const fills = TOOLS.parachute.fuel.fills;
    assert.ok(Math.abs(t - fills) < 0.1, `it took ${t.toFixed(2)}s to repack, not ${fills}`);
    assert.equal(w.chute.phase, 'packed', 'the repacked parachute is not back on the walker\'s back');
    assert.ok(w.said.some(s => /packed - ready again/.test(s)), 'nobody was told it was ready');
  });

  // Verifies: REQ-TOOL-070
  it('opens only from high enough, and only packed', () => {
    const W = WALK.Walker.prototype;
    const w = {
      ...walker(), p: { x: 0, z: 0, feet: 0.3, vy: 0, yaw: 0, ground: true }, drift: { x: 0, z: 0 },
      height: () => 0, cutLine() {}, drawHud() {}, deploy: W.deploy, fell: null,
    };
    w.deploy();
    assert.equal(w.chute.phase, 'packed', 'it opened on the ground');
    Object.assign(w.p, { ground: false, feet: P.MIN_DEPLOY * 0.8 });
    w.deploy();
    assert.equal(w.chute.phase, 'packed', 'it opened off the top of a jump');
    assert.match(w.said.at(-1), /Too low/);
    Object.assign(w.p, { feet: 5, vy: -3, yaw: 1.2 });
    w.deploy();
    assert.equal(w.chute.phase, 'pilot', 'it did not open from a roof');
    assert.equal(w.chute.heading, 1.2, 'it did not open on the heading the walker faced');
    assert.equal(w.chute.vy, -3, 'it did not take over the fall in progress');
    assert.equal(w.tank(TOOLS.parachute), 0);
    // Landed and not yet repacked, it will not open again.
    w.chute.phase = 'landed';
    w.deploy();
    assert.equal(w.chute.phase, 'landed');
    assert.match(w.said.at(-1), /repacked/);
  });

  // Verifies: REQ-TOOL-077
  it('comes off on landing, leaving the off hand free while it repacks', () => {
    const W = WALK.Walker.prototype;
    const w = {
      ...walker(), p: { x: 0, z: 0, feet: 0.2, vy: -2 }, active: false, laid: [], lay(kind) { this.laid.push(kind); },
      height: () => 0, health: { touchdown: () => 0, dead: false }, drawHud() {}, touchdown: W.touchdown,
    };
    w.touchdown({ speed: 2, open: 1, vertical: 2 });
    assert.deepEqual(w.laid, ['landed']);
    assert.equal(w.secondary, null, 'the parachute is still in hand after landing');
    // ... and repacks put away as it would in hand.
    w.spend(TOOLS.parachute);
    w.chute.phase = 'landed';
    for (let t = 0; t < TOOLS.parachute.fuel.fills + 1; t += 0.05) w.burn(0.05, false);
    assert.equal(w.tank(TOOLS.parachute), 1, 'a parachute put away did not repack');
  });

  // Verifies: REQ-TOOL-078
  it('cuts the canopy loose when the hands change, and the walker falls', () => {
    const W = WALK.Walker.prototype;
    const w = {
      ...walker(), primary: TOOLS.rod, p: { x: 0, z: 0, feet: 6, vy: 0 }, hooks: {}, active: false, showing: null,
      laid: [], lay(kind) { this.laid.push(kind); }, drawHud() {}, flying() { return false; },
      setTool: W.setTool, cutAwayCanopy: W.cutAwayCanopy,
    };
    P.deployChute(w.chute, { x: 0, feet: 6, z: 0 });
    fly(w.chute, 2);
    // Another hunting tool leaves the left hand alone ...
    w.setTool('net');
    assert.ok(P.aloft(w.chute), 'changing the hunting hand cut the canopy away');
    // ... another carried tool takes it out of the left one.
    w.setTool('jetpack');
    assert.equal(w.chute.phase, 'cutaway');
    assert.deepEqual(w.laid, ['cutaway'], 'the canopy did not fly off on its own');
    assert.equal(w.fell, w.p.feet, 'the fall did not start where the canopy was let go');
    assert.equal(w.p.vy, w.chute.vy, 'the walker did not keep the canopy\'s sink');
    // And once it is gone it moves nobody.
    const was = { ...w.chute };
    assert.deepEqual(P.stepChute(w.chute, { forward: 1 }, 1, FLAT), []);
    assert.equal(w.chute.feet, was.feet);
  });
});

describe('the canopy as drawn', () => {
  const unlit = { scene: { add() {}, remove() {} }, bendable: m => m };

  // Verifies: REQ-TOOL-071
  it('builds a wing that opens, breathes and answers the toggles', () => {
    const shape = look => P.shapeOf({ open: 1, brake: 0.3, turn: 0, flare: 0, speed: 3.6, time: 0, ...look });
    const point = (look, s, u) => P.canopyPoint(s, u, 0, 0, shape(look), new THREE.Vector3());
    // Opening spreads it from a bundle to its full span, the middle first.
    const early = shape({ open: 0.3 });
    assert.ok(early.span < 0.5 && early.cells[4] > early.cells[0], 'it does not open from the middle out');
    assert.ok(point({ open: 1 }, 1, 0.5).x > 3 * point({ open: 0.2 }, 1, 0.5).x, 'the span does not spread');
    // The tips hang below the middle on the arc.
    assert.ok(point({}, 1, 0.5).y < point({}, 0, 0.5).y - 0.1, 'the span is not curved');
    // A left toggle pulls the left side of the tail down, and only that side.
    const left = point({ turn: 1 }, -0.9, 1).y, right = point({ turn: 1 }, 0.9, 1).y;
    assert.ok(left < point({}, -0.9, 1).y - 0.05, 'the left toggle did not pull the tail down');
    assert.ok(Math.abs(right - point({}, 0.9, 1).y) < 1e-9, 'the left toggle pulled the right side too');
    // The cells breathe, and the tail flutters faster the faster it goes.
    assert.notEqual(point({ time: 0 }, 0.1, 0.3).y, point({ time: 0.7 }, 0.1, 0.3).y, 'the cells do not breathe');
    assert.ok(shape({ speed: 4 }).flutter > shape({ speed: 1 }).flutter, 'the tail does not flutter with speed');
  });

  // Verifies: REQ-TOOL-071, REQ-TOOL-075
  it('poses every line from the canopy to the harness, with nothing left at the origin', () => {
    const rig = P.canopyRig((geometry, color, options) => new THREE.Mesh(geometry, new THREE.MeshBasicMaterial({ color })),
      color => new THREE.LineBasicMaterial({ color }));
    const chute = thrown(30);
    fly(chute, 3);
    P.poseRig(rig, P.lookOf(chute, 1000), new THREE.Vector3(-0.2, 0.1, -0.3));
    const lines = rig.getObjectByName('lines').geometry;
    const points = lines.getAttribute('position');
    const drawn = lines.drawRange.count === Infinity ? points.count : lines.drawRange.count;
    assert.ok(drawn >= 100 * P.LINE_PIECES, `only ${drawn / 2 / P.LINE_PIECES} lines are drawn`);
    for (let i = 0; i < drawn; i++) {
      const p = new THREE.Vector3().fromBufferAttribute(points, i);
      assert.ok(Number.isFinite(p.length()), `line point ${i} is not a number`);
      assert.ok(p.length() > 0.05, `line point ${i} is at the rig's origin, where no line goes`);
    }
    // The left brake line runs on from its cascade to the hand it was given: it comes
    // after the forty suspension lines and the four lines fanning in from the left
    // side of the tail.
    const hand = new THREE.Vector3().fromBufferAttribute(points, (40 + 4) * P.LINE_PIECES * 2);
    assert.ok(hand.distanceTo(new THREE.Vector3(-0.2, 0.1, -0.3)) < 1e-6, 'the left brake line does not reach the hand');
  });

  // Verifies: REQ-TOOL-071
  it('keeps the risers and the slider out of the middle of the view looking up', () => {
    // Looking straight up under an open canopy, what a skydiver sees is the canopy and
    // the lines fanning down from it; the risers are webbing at the edges of the view,
    // from the shoulders up and out to the links, and the slider is bunched up under
    // the canopy. So the canopy is hung over a stand-in walker the way the walker
    // hangs it (Walker.hangCanopy, from the eye), the view is turned straight up at
    // headings all round the canopy's, and every part of the risers in front of the
    // lens is measured against a cone around the middle of the view.
    const W = WALK.Walker.prototype;
    const camera = new THREE.PerspectiveCamera(70, 1280 / 800, 0.02, 3000);
    camera.rotation.order = 'YXZ';
    const scene = { viewScene: new THREE.Scene(), walkCamera: camera };
    scene.viewScene.add(camera);
    const chute = thrown(40);
    fly(chute, 3);
    assert.equal(chute.phase, 'flying');
    const w = {
      scene, chute, p: { x: 3, feet: chute.feet, z: -2 }, arrival: null, held: null, offhand: null,
      rigWorld: W.rigWorld, eye: W.eye, hideTool() {},
    };
    // The middle of the view: a cone 36 degrees either side of where the eyes point,
    // which is past the top and bottom edges of a 70 degree view.
    const CONE = (36 * Math.PI) / 180;
    const box = new THREE.Box3(), local = new THREE.Vector3(), seen = new THREE.Vector3();
    for (const turn of [0, 0.5, Math.PI / 2, 2.2, Math.PI, -1.1, -Math.PI / 2]) {
      w.eye(camera.position);
      camera.rotation.set(1.5, chute.heading + turn, 0);
      camera.updateMatrixWorld();
      W.hangCanopy.call(w, 1000);
      scene.viewScene.updateMatrixWorld();
      const risers = w.rig.getObjectByName('risers');
      assert.ok(risers.visible, 'no risers over an open canopy');
      let nearest = Infinity;
      risers.traverse(part => {
        if (!part.isMesh) return;
        part.geometry.computeBoundingBox();
        box.copy(part.geometry.boundingBox);
        for (let i = 0; i <= 2; i++) for (let j = 0; j <= 40; j++) for (let k = 0; k <= 2; k++) {
          local.set(box.min.x + ((box.max.x - box.min.x) * i) / 2, box.min.y + ((box.max.y - box.min.y) * j) / 40,
            box.min.z + ((box.max.z - box.min.z) * k) / 2);
          seen.copy(local).applyMatrix4(part.matrixWorld).applyMatrix4(camera.matrixWorldInverse);
          if (seen.z > -camera.near) continue; // behind the lens
          nearest = Math.min(nearest, Math.atan2(Math.hypot(seen.x, seen.y), -seen.z));
        }
      });
      assert.ok(nearest > CONE, `turned ${turn.toFixed(2)} from the heading, a riser is ${(nearest * 180 / Math.PI).toFixed(1)} degrees from the middle of the view`);
    }
    // The slider is up under the canopy, not down in front of the eyes.
    const rig = w.rig, slider = rig.getObjectByName('slider'), canopy = rig.getObjectByName('canopy');
    assert.ok(slider.visible, 'no slider on an open canopy');
    const corners = slider.geometry.getAttribute('position');
    for (let i = 0; i < corners.count; i++) {
      assert.ok(corners.getY(i) > 0.75 * canopy.position.y,
        `a corner of the slider is ${corners.getY(i).toFixed(2)} over the shoulders, under a canopy ${canopy.position.y.toFixed(2)} over them`);
    }
  });

  // Verifies: REQ-TOOL-077, REQ-TOOL-078
  it('keeps every line of a canopy let go of between the canopy and the harness', () => {
    // Taut lines to where the harness was would stand up out of the street once the
    // canopy has come down - a curtain of lines up out of the top of the view. So a
    // canopy let go of has slack lines: collapsing, draped, repacking, cut away, none
    // of them may rise above the canopy or leave the space it and the harness span.
    const chute = thrown(30);
    fly(chute, 4);
    const from = new THREE.Matrix4().makeTranslation(0, 0.3, 0);
    for (const kind of ['landed', 'cutaway']) {
      const loose = new P.LooseCanopy(unlit, kind, from, P.lookOf(chute, 0), { ground: 0, velocity: { x: 0, y: -1, z: -3 }, life: 12 });
      const rig = loose.rig, canopy = rig.getObjectByName('canopy'), wing = rig.getObjectByName('wing');
      const lines = rig.getObjectByName('lines').geometry;
      for (const t of [0.1, 0.3, 0.6, 1, 1.6, 3, 8]) {
        while (loose.t < t) loose.update(0.05);
        wing.geometry.computeBoundingBox();
        const bounds = wing.geometry.boundingBox.clone().applyMatrix4(canopy.matrix);
        const top = bounds.max.y;
        bounds.expandByPoint(loose.look.harness).expandByScalar(0.02);
        const points = lines.getAttribute('position');
        for (let i = 0; i < lines.drawRange.count; i++) {
          const p = new THREE.Vector3().fromBufferAttribute(points, i);
          assert.ok(p.y <= top + 1e-6, `${kind} at ${t}s: a line rises ${(p.y - top).toFixed(3)} above the canopy`);
          assert.ok(bounds.containsPoint(p), `${kind} at ${t}s: a line leaves the canopy and the harness, at ${p.toArray().map(v => v.toFixed(2))}`);
        }
      }
      // Draped, the canopy lies on the ground and its lines along it.
      if (kind === 'landed') {
        const ground = loose.look.harness.y;
        const points = lines.getAttribute('position');
        let highest = -Infinity;
        for (let i = 0; i < lines.drawRange.count; i++) highest = Math.max(highest, points.getY(i));
        assert.ok(highest - ground < 0.15, `a draped canopy's lines stand ${highest - ground} off the ground`);
        wing.geometry.computeBoundingBox();
        const low = wing.geometry.boundingBox.clone().applyMatrix4(canopy.matrix).min.y;
        assert.ok(low >= ground - 0.05, `the draped canopy sinks ${(ground - low).toFixed(3)} into the ground`);
      }
      loose.dispose();
    }
  });

  // Verifies: REQ-TOOL-077, REQ-TOOL-078
  it('leaves a canopy behind out of unlit parts, and takes it away in time', () => {
    const chute = thrown(30);
    fly(chute, 3);
    for (const kind of ['landed', 'cutaway']) {
      const added = [];
      const scene = { ...unlit, scene: { add: o => added.push(o), remove: o => added.splice(added.indexOf(o), 1) } };
      const loose = new P.LooseCanopy(scene, kind, new THREE.Matrix4(), P.lookOf(chute, 0), { ground: -0.4, life: 4 });
      loose.rig.traverse(o => {
        if (o.isMesh) assert.equal(o.material.type, 'MeshBasicMaterial', `a ${kind} canopy has a lit part`);
      });
      assert.equal(added.length, 1);
      let t = 0;
      while (loose.update(0.05)) t += 0.05;
      assert.ok(Math.abs(t - 4) < 0.1, `a ${kind} canopy stayed ${t.toFixed(2)}s, not its life`);
      assert.equal(added.length, 0, `a ${kind} canopy was left on the map`);
    }
  });

  // Verifies: REQ-TOOL-079
  it('shows the figure on the map under a canopy when the walker left under one', () => {
    const scene = { scene: { add() {}, remove() {} }, bendable: m => m, requestRender() {}, setAnimated() {}, camera: { zoom: 20 } };
    const figure = new Avatar(scene);
    const canopies = () => figure.group.children.filter(o => figure.canopy.includes(o));
    figure.set({ x: 0, z: 0, feet: 3, yaw: 0, canopy: true }, '#ff8a1f');
    assert.ok(canopies().length && canopies().every(o => o.visible), 'no canopy over a walker who left under one');
    figure.set({ x: 0, z: 0, feet: 0, yaw: 0 }, '#ff8a1f');
    assert.ok(canopies().every(o => !o.visible), 'a canopy over a walker standing in the street');
  });
});

describe('the walker on the map', () => {
  // Verifies: REQ-MAP-065
  it('pulses a ring out from the figure while it is shown, and keeps the map drawing only then', () => {
    const asked = new Map();
    const scene = { scene: { add() {}, remove() {} }, bendable: m => m, requestRender() {}, camera: { zoom: 20 }, setAnimated(on, why) { asked.set(why, on); } };
    const figure = new Avatar(scene);
    figure.set({ x: 0, z: 0, feet: 0, yaw: 0 }, '#ff8a1f');
    figure.show(true);
    assert.equal(asked.get('avatar'), true, 'the map is not kept drawing for the pulse');
    const [ring] = figure.pulse, seen = [];
    const now = performance.now;
    try {
      for (const t of [0, 250, 500, 750]) {
        performance.now = () => t;
        ring.onBeforeRender();
        seen.push([ring.scale.x, ring.material.opacity]);
      }
    } finally {
      performance.now = now;
    }
    for (let i = 1; i < seen.length; i++) assert.ok(seen[i][0] > seen[i - 1][0] && seen[i][1] < seen[i - 1][1], `the ring does not spread and fade: ${seen}`);
    figure.show(false);
    assert.equal(asked.get('avatar'), false, 'the map keeps drawing for a figure not shown');
  });
});

describe('a canopy in the wind', () => {
  // Verifies: REQ-TOOL-081
  it('is carried over the ground by the wind, at its own airspeed', () => {
    const still = thrown(60), blown = Object.assign(thrown(60), { wind: { x: 2, z: 0 } });
    fly(still, 12);
    fly(blown, 12);
    // Thrown open with no way on, it is carried for the whole of it, opening included.
    const drift = blown.x - still.x;
    assert.ok(Math.abs(drift - 24) < 3, `a canopy in a crosswind of 2 drifted ${drift} in 12 seconds`);
    assert.ok(Math.abs(blown.z - still.z) < 1, 'a crosswind changed how far it flew along its heading');
  });

  // Verifies: REQ-TOOL-081
  it('comes down slower over the ground into the wind than with it', () => {
    const into = landing(Object.assign(glidingAt(10), { wind: { x: 0, z: 2 } }));
    const downwind = landing(Object.assign(glidingAt(10), { wind: { x: 0, z: -2 } }));
    assert.ok(into.horizontal < downwind.horizontal - 3, `${into.horizontal} into the wind, ${downwind.horizontal} with it`);
    assert.ok(into.speed < downwind.speed, 'a landing into the wind was no softer');
  });
});
