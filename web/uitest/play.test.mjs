// Playing in the parks (walk/play.js), on a stand-in walker standing on flat ground by
// one amenity at a time, stepped at sixty frames a second: the rides, and the ball
// games played to the end - a shot, a penalty, a serve - with nothing but what a
// click and the keys would do.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const { amenitiesFor, onAmenity, inAmenity } = await import('../static/map/amenities.js');
const { PAINT } = await import('../static/map/shapes.js');
const WALK = await import('../static/walk/walk.js');

const CITY = amenitiesFor('city');
const [PITCH, COURT, VOLLEY, PLAYGROUND, ROUNDABOUT] = CITY;
const SPACE_PARK = amenitiesFor('galaxy').find(a => a.play.some(e => e.ride === 'drive'));
const FRAME = 1 / 60;

/** A walker by `a` (an amenity, placed at the origin `turn`ed) standing at its model point `at`, facing its model point `to`. */
function walker(a, at, to, turn = 0) {
  const it = { x: 0, y: 0, z: 0, turn, kind: 0, spec: a };
  it.rigs = a.rigs.map(() => ({ scatter: { pose() {} }, item: {} }));
  const said = [];
  const play = { textContent: '' };
  const w = Object.assign(Object.create(WALK.Walker.prototype), {
    active: true, handsOff: true, secondary: null, frozen: false, wheel: { open: false }, showing: null,
    keys: new Set(), moving: new THREE.Vector3(), riding: null, ballHeld: null, balls: new Map(), settling: [], flung: null,
    p: { x: 0, z: 0, feet: 0, yaw: 0, pitch: 0, vy: 0, ground: true, fly: false },
    hud: { querySelector: q => (q === '.w-play' ? play : null) },
    scene: {
      props: { userData: { amenities: [it] } }, style: 'city', scene: new THREE.Scene(),
      bendable: m => m, walkCamera: new THREE.PerspectiveCamera(), viewScene: new THREE.Scene(),
    },
  });
  w.height = () => 0;
  w.boxAt = () => null;
  w.flash = text => said.push(text);
  w.showBall = () => {};
  w.hideBall = () => {};
  const from = onAmenity(it, a, at), toward = onAmenity(it, a, to);
  Object.assign(w.p, { x: from.x, z: from.z, yaw: Math.atan2(-(toward.x - from.x), -(toward.z - from.z)) });
  return { w, it, said, play };
}

/** Steps the walker's play for `seconds`, the keys in `keys` held. */
function run(w, seconds, keys = []) {
  for (const k of keys) w.keys.add(k);
  for (let t = 0; t < seconds; t += FRAME) {
    if (w.riding) w.rideStep(FRAME);
    else w.coast(FRAME);
    w.updatePlay(FRAME);
  }
  for (const k of keys) w.keys.delete(k);
}

// The court's ball, put at the walker's feet in front of them.
function ballAtFeet(w, kind) {
  w.updatePlay(FRAME);
  const ball = [...w.balls.values()].find(b => b.kind === kind);
  ball.pos.set(w.p.x - Math.sin(w.p.yaw) * 0.25, ball.r, w.p.z - Math.cos(w.p.yaw) * 0.25);
  ball.vel.set(0, 0, 0);
  return ball;
}

/**
 * Looks up or down, as a player would by the guide, for where the ball sent `how` does
 * what `wanted` asks of its flight (Walker.flight), and keeps to the middle of that.
 */
function aimFor(w, ball, how, wanted) {
  const good = [];
  for (let pitch = -0.6; pitch <= 1.2; pitch += 0.005) {
    w.p.pitch = pitch;
    if (wanted(w.flight(ball, w.plan(ball, how)))) good.push(pitch);
  }
  assert.ok(good.length, `nowhere to look sends the ${ball.kind} where it should go`);
  w.p.pitch = good[good.length >> 1];
}

const swing = PLAYGROUND.play.find(e => e.ride === 'swing');
const seat = [swing.pivot[0] + 0.05, 0, swing.pivot[2]];

describe('playing in the parks', () => {
  // Verifies: REQ-WALK-057
  it('only with nothing in either hand', () => {
    const { w, play } = walker(PLAYGROUND, seat, [1, 0, swing.pivot[2]]);
    w.handsOff = false;
    w.updatePlay(FRAME);
    assert.equal(play.textContent, 'H puts your tools away to play');
    assert.equal(w.playClick(), false, 'a walker with a tool in hand got on a swing');
    w.handsOff = true;
    w.secondary = { id: 'grapple' };
    w.updatePlay(FRAME);
    assert.match(play.textContent, /left hand/);
    assert.equal(w.playClick(), false);
    w.secondary = null;
    w.updatePlay(FRAME);
    assert.equal(play.textContent, 'Click: ride the swing');
    assert.equal(w.playClick(), true);
    assert.equal(w.riding?.entry.ride, 'swing');
    // Taking a tool out is getting off.
    w.handsOff = false;
    w.updatePlay(FRAME);
    assert.equal(w.riding, null);
  });

  // Verifies: REQ-WALK-057
  it('pumps a swing higher and flings the walker off it, and turns a walker with a roundabout', () => {
    const { w } = walker(PLAYGROUND, seat, [1, 0, swing.pivot[2]]);
    w.playClick();
    let highest = 0;
    for (let t = 0; t < 12; t += FRAME) {
      w.keys.add('KeyW');
      w.rideStep(FRAME);
      highest = Math.max(highest, Math.abs(w.riding.angle));
    }
    w.keys.clear();
    assert.ok(highest > 0.7, `pumped for twelve seconds, the swing went ${highest.toFixed(2)} radians`);
    // Off at the bottom of the swing, going fast: carried on, and up.
    while (Math.abs(w.riding.angle) > 0.1) w.rideStep(FRAME);
    w.keys.add('Space');
    w.rideStep(FRAME);
    assert.equal(w.riding, null);
    assert.ok(w.flung && Math.hypot(w.flung.x, w.flung.z) > 0.5 && w.p.vy > 0, 'jumping off a swing in full flight goes nowhere');
    assert.equal(w.settling.length, 1, 'the swing let go of stops dead');

    const spin = ROUNDABOUT.play.find(e => e.ride === 'spin');
    const r = walker(ROUNDABOUT, [spin.pivot[0] + spin.r, 0, spin.pivot[2]], spin.pivot).w;
    assert.equal(r.playClick(), true);
    const yaw = r.p.yaw;
    run(r, 2, ['KeyW']);
    assert.ok(r.riding.speed > 2, `pushed for two seconds the roundabout turns at ${r.riding.speed}`);
    assert.ok(Math.abs(r.p.yaw - yaw) > 1, 'the walker on a roundabout is not turned with it');
  });

  // Verifies: REQ-WALK-057
  it('sits a walker on a seesaw\'s end and bounces it off the ground', () => {
    const rock = PLAYGROUND.play.find(e => e.ride === 'rock');
    const seat = [rock.pivot[0] + rock.seat[0], 0, rock.pivot[2] + rock.seat[2]];
    const { w, it } = walker(PLAYGROUND, seat, rock.pivot);
    assert.equal(w.playClick(), true, 'no seesaw at the end of a seesaw');
    assert.equal(w.riding.entry.ride, 'rock');
    const at = onAmenity(it, PLAYGROUND, seat);
    assert.ok(Math.hypot(w.p.x - at.x, w.p.z - at.z) < 0.1, 'the walker is not sat on the seesaw\'s end');
    run(w, 0.5); // sat down
    const low = w.p.feet;
    let high = low;
    for (let t = 0; t < 1.5; t += FRAME) {
      if (t < 0.1) w.keys.add('KeyW'); else w.keys.delete('KeyW');
      w.rideStep(FRAME);
      high = Math.max(high, w.p.feet);
    }
    assert.ok(high - low > 0.1, `pushed off, the seat rose ${(high - low).toFixed(3)}`);
  });

  // Verifies: REQ-WALK-057
  it('climbs a slide and slides down it', () => {
    const slide = PLAYGROUND.play.find(e => e.ride === 'slide');
    const { w } = walker(PLAYGROUND, slide.path[0], slide.path[1]);
    assert.equal(w.playClick(), true);
    let top = 0;
    for (let t = 0; t < 10 && w.riding; t += FRAME) {
      w.rideStep(FRAME);
      top = Math.max(top, w.p.feet);
    }
    assert.equal(w.riding, null, 'still on the slide after ten seconds');
    assert.ok(top > 0.4, `the walker climbed to ${top.toFixed(2)}`);
    const end = onAmenity({ x: 0, y: 0, z: 0, turn: 0 }, PLAYGROUND, slide.path.at(-1));
    assert.ok(Math.hypot(w.p.x - end.x, w.p.z - end.z) < 0.05, 'the walker did not come off the end of the slide');
  });

  // Verifies: REQ-WALK-057
  it('turns a capacitor and rocks a transistor on its lead, on a board', () => {
    const park = amenitiesFor('circuit').find(a => a.play.some(e => e.ride === 'spin'));
    const spin = park.play.find(e => e.ride === 'spin');
    const turner = walker(park, [spin.pivot[0] + spin.r, 0, spin.pivot[2]], spin.pivot).w;
    assert.equal(turner.playClick(), true);
    run(turner, 1.5, ['KeyW']);
    assert.ok(turner.riding.speed > 2, `the capacitor turns at ${turner.riding.speed}`);
    const rock = park.play.find(e => e.ride === 'rock');
    const at = [rock.pivot[0], 0, rock.pivot[2]];
    const { w } = walker(park, at, [at[0] + 1, 0, at[2]]);
    assert.equal(w.playClick(), true);
    assert.equal(w.riding.entry.ride, 'rock');
    let most = 0;
    for (let t = 0; t < 1.5; t += FRAME) {
      if (t < 0.1) w.keys.add('KeyW'); else w.keys.delete('KeyW');
      w.rideStep(FRAME);
      most = Math.max(most, Math.abs(w.riding.angle));
    }
    assert.ok(most > 0.1, `the transistor rocked ${most.toFixed(2)}`);
  });

  // Verifies: REQ-WALK-057
  it('slides down a pipe, round and round, and out of its end', () => {
    const pipe = SPACE_PARK.play.find(e => e.ride === 'slide');
    const { w, it } = walker(SPACE_PARK, pipe.path[0], pipe.path[1]);
    assert.equal(w.playClick(), true);
    let turned = 0, last = w.p.yaw, t = 0;
    for (; t < 15 && w.riding; t += FRAME) {
      w.rideStep(FRAME);
      turned += Math.atan2(Math.sin(w.p.yaw - last), Math.cos(w.p.yaw - last));
      last = w.p.yaw;
    }
    assert.equal(w.riding, null, 'still in the pipe after fifteen seconds');
    assert.ok(t < 10, `the ride took ${t.toFixed(1)} seconds`);
    assert.ok(Math.abs(turned) > 5, `turned ${turned.toFixed(1)} going down`);
    const end = onAmenity(it, SPACE_PARK, pipe.path.at(-1));
    assert.ok(Math.hypot(w.p.x - end.x, w.p.z - end.z) < 0.05, 'the walker did not come out of the pipe');
  });

  // Verifies: REQ-WALK-057
  it('drives a bobby car round its playground, steered, and leaves it where it stopped', () => {
    const drive = SPACE_PARK.play.find(e => e.ride === 'drive'), rig = SPACE_PARK.rigs[drive.rig];
    const seat = [rig.at[0] + drive.seat[0], 0, rig.at[2] + drive.seat[2]];
    const { w, it } = walker(SPACE_PARK, [rig.at[0] + 0.03, 0, rig.at[2] + 0.15], rig.at);
    assert.equal(w.playClick(), true);
    assert.equal(w.riding.entry.ride, 'drive');
    const home = onAmenity(it, SPACE_PARK, rig.at), car = it.cars[drive.rig];
    run(w, 1.5, ['KeyW']);
    const moved = onAmenity(it, SPACE_PARK, [car.x, 0, car.z]).distanceTo(home);
    assert.ok(moved > 0.5, `the car went ${moved.toFixed(2)}`);
    const yaw = car.yaw, view = w.p.yaw;
    run(w, 1, ['KeyW', 'KeyA']);
    assert.ok(car.yaw - yaw > 0.5, 'A did not steer');
    assert.ok(Math.abs(w.p.yaw - view - (car.yaw - yaw)) < 1e-9, 'the view did not turn with the car');
    // Held at full speed a long while, it stays on the playground's ground.
    run(w, 8, ['KeyW']);
    const [x0, z0, x1, z1] = SPACE_PARK.floors[0];
    assert.ok(car.x > x0 && car.x < x1 && car.z > z0 && car.z < z1, 'the car left the playground');
    // Off, the car stays where it stopped, and the walker steps out beside it.
    w.keys.add('Space');
    w.rideStep(FRAME);
    w.keys.delete('Space');
    assert.equal(w.riding, null);
    const left = { x: car.x, z: car.z };
    run(w, 1);
    assert.deepEqual({ x: car.x, z: car.z }, left);
    const now = w.carPoint(it, drive, drive.seat, new THREE.Vector3());
    assert.ok(Math.hypot(w.p.x - now.x, w.p.z - now.z) > 0.15, 'the walker is left in the car');
  });

  // Verifies: REQ-WALK-058
  it('puts a shot from the free-throw line through the hoop', () => {
    // On a board and in the galaxy as in a city.
    for (const style of ['city', 'circuit', 'galaxy']) {
      const court = amenitiesFor(style).find(a => a.play.some(e => e.hoop));
      const { w, said } = walker(court, [-0.3, 0, 0], [-0.565, 0.25, 0]);
      const ball = ballAtFeet(w, 'basket');
      assert.equal(w.playClick(), true);
      assert.ok(w.ballHeld, 'the ball was not picked up');
      aimFor(w, ball, 'throw', f => f.played.scored);
      w.playClick();
      run(w, 3);
      assert.ok(said.some(s => /two points/.test(s)), `the shot in the ${style}: ${said}`);
    }
  });

  // Verifies: REQ-WALK-058
  it('kicks a penalty into the goal', () => {
    // A pitch laid either way round.
    for (const turn of [0, Math.PI / 2]) {
      const { w, said } = walker(PITCH, [0.55, 0, 0], [0.85, 0.05, 0], turn);
      const ball = ballAtFeet(w, 'soccer');
      aimFor(w, ball, 'kick', f => f.played.goal);
      w.playClick();
      run(w, 3);
      assert.ok(said.includes('Goal!'), `the kick: ${said}`);
      assert.ok(ball.vel.length() < 1, 'the net did not stop the ball');
    }
  });

  // Verifies: REQ-WALK-058
  it('serves over the net into the far half, standing or jumping', () => {
    for (const jump of [false, true]) {
      const { w, it, said } = walker(VOLLEY, [-0.8, 0, 0.1], [0.5, 0, 0], jump ? Math.PI / 2 : 0);
      const ball = ballAtFeet(w, 'volley');
      w.playClick();
      if (jump) Object.assign(w.p, { feet: 0.3, ground: false });
      // Over the net and down well inside the far half.
      aimFor(w, ball, 'serve', ({ played, point }) => {
        const [x, , z] = inAmenity(it, VOLLEY, point);
        return !played.net && played.over === 1 && x > 0.2 && x < 0.7 && Math.abs(z) < 0.3;
      });
      w.playClick();
      run(w, 4);
      assert.ok(said.includes(jump ? 'Ace! A jump serve in' : 'In!'), `the ${jump ? 'jump ' : ''}serve: ${said}`);
    }
  });
});

describe('balls, rides and the guide, kept honest', () => {
  // Verifies: REQ-WALK-058
  it('puts out a ball for every court, not one for each kind of court', () => {
    const { w, it } = walker(COURT, [0, 0, 0], [1, 0, 0]);
    const other = { ...it, x: 6, rigs: [] };
    w.scene.props.userData.amenities.push(other);
    w.updatePlay(FRAME);
    const balls = [...w.balls.values()];
    assert.equal(balls.length, 2);
    assert.notEqual(balls[0].it, balls[1].it);
  });

  // Verifies: REQ-WALK-058
  it('rests a ball on the ground under its middle, and puts one back that is lost', () => {
    const { w } = walker(COURT, [-0.5, 0, 0.3], [1, 0, 0]);
    // A curb at x > 0.3 the walker's corners would find, beside the ball.
    w.height = (x, z, from, probes = [0, 0, 0.12, 0, -0.12, 0]) => {
      let top = 0;
      for (let i = 0; i < probes.length; i += 2) if (x + probes[i] > 0.3) top = 0.05;
      return top;
    };
    w.updatePlay(FRAME);
    const ball = [...w.balls.values()][0];
    ball.pos.set(0.25, 0.3, 0);
    ball.vel.set(0, 0, 0);
    run(w, 2);
    assert.ok(Math.abs(ball.pos.y - ball.r) < 1e-6, `a ball beside a curb rests at ${ball.pos.y.toFixed(3)}, not on the ground`);
    // Kicked off the court, it lies there a moment and is back in the middle.
    ball.pos.set(9, ball.r, 0);
    run(w, 0.5);
    assert.ok(ball.pos.distanceTo(ball.home) > 1, 'put back at once');
    run(w, 1.5);
    assert.ok(ball.pos.distanceTo(ball.home) < 1e-9, 'a ball off its court is never put back');
    // Far away, it is back at once.
    ball.pos.set(40, ball.r, 0);
    run(w, FRAME);
    assert.ok(ball.pos.distanceTo(ball.home) < 1e-9);
    assert.equal(POINT.length, 2);
  });

  // Verifies: REQ-WALK-058
  it('stands the walker and a ball on a court\'s paint, not in it', () => {
    const { w } = walker(COURT, [-0.3, 0, 0.2], [1, 0, 0]);
    Object.assign(w, { height: WALK.Walker.prototype.height, cellAt: () => [], decks: new Map(), spans: new Map() });
    const paint = PAINT * COURT.scale;
    assert.ok(Math.abs(w.height(w.p.x, w.p.z) - paint) < 1e-9, `on the court the ground is at ${w.height(w.p.x, w.p.z)}`);
    assert.ok(w.height(40, 0) < paint, 'the court reaches beyond its edge');
    w.updatePlay(FRAME);
    const ball = [...w.balls.values()][0];
    ball.pos.y += 0.2;
    run(w, 2);
    assert.ok(Math.abs(ball.pos.y - paint - ball.r) < 1e-6, `the ball rests at ${ball.pos.y.toFixed(4)}, in the court`);
  });

  // Verifies: REQ-WALK-058
  it('leaves a ball where it lies when walked into, or clicked without looking at it', () => {
    const { w } = walker(PITCH, [0.3, 0, 0], [0.85, 0, 0]);
    const ball = ballAtFeet(w, 'soccer'), at = ball.pos.clone();
    // Walked straight at, the walker stops against it.
    for (let t = 0; t < 1; t += FRAME) {
      w.p.x -= Math.sin(w.p.yaw) * 0.8 * FRAME;
      w.p.z -= Math.cos(w.p.yaw) * 0.8 * FRAME;
      w.updatePlay(FRAME);
    }
    assert.ok(ball.pos.distanceTo(at) < 1e-9, `walked into, the ball moved ${ball.pos.distanceTo(at).toFixed(3)}`);
    assert.ok(Math.hypot(w.p.x - at.x, w.p.z - at.z) >= 0.12 + ball.r - 1e-9, 'the walker walked through the ball');
    // Turned well away from it, a click kicks nothing.
    w.p.yaw += 1.2;
    assert.equal(w.playClick(), false, 'a ball off to the side was kicked');
    assert.ok(!w.kicking);
  });

  // Verifies: REQ-WALK-058
  it('sends a ball down the very path its guide draws: a shot, a kick and a serve', () => {
    for (const [court, kind, at, to] of [
      [COURT, 'basket', [-0.3, 0, 0], [-0.565, 0.25, 0]],
      [PITCH, 'soccer', [0.55, 0, 0], [0.85, 0.05, 0]],
      [VOLLEY, 'volley', [-0.8, 0, 0.1], [0.5, 0, 0]],
    ]) {
      const { w } = walker(court, at, to);
      const ball = ballAtFeet(w, kind);
      if (kind !== 'soccer') w.playClick();
      const aim = w.aiming();
      assert.ok(aim, `no guide for a ${kind}`);
      const { path } = w.flight(aim.ball, w.plan(aim.ball, aim.how));
      w.playClick();
      if (kind === 'soccer') while (w.kicking) w.updatePlay(FRAME);
      // The guide keeps every other step; the ball, stepped frame by frame, is where it said.
      let worst = 0;
      for (let i = 1; i < path.length - 1; i++) {
        w.updatePlay(FRAME);
        worst = Math.max(worst, ball.pos.distanceTo(path[i]));
      }
      assert.ok(worst < 1e-6, `the ${kind} left its guide by ${worst}`);
    }
  });

  // Verifies: REQ-WALK-057
  it('rides a slide in one run, and gets up off its end without a jump', () => {
    const slide = PLAYGROUND.play.find(e => e.ride === 'slide');
    const { w } = walker(PLAYGROUND, slide.path[0], slide.path[1]);
    w.playClick();
    const eye = () => w.p.feet + EYE + w.eyeShift;
    let last = { x: w.p.x, z: w.p.z, eye: eye(), yaw: w.p.yaw }, worst = { step: 0, eye: 0, yaw: 0 };
    for (let t = 0; t < 12 && w.riding; t += FRAME) {
      w.rideStep(FRAME);
      w.updatePlay(FRAME);
      const now = { x: w.p.x, z: w.p.z, eye: eye(), yaw: w.p.yaw };
      worst.step = Math.max(worst.step, Math.hypot(now.x - last.x, now.z - last.z));
      worst.eye = Math.max(worst.eye, Math.abs(now.eye - last.eye));
      worst.yaw = Math.max(worst.yaw, Math.abs(Math.atan2(Math.sin(now.yaw - last.yaw), Math.cos(now.yaw - last.yaw))));
      last = now;
    }
    assert.equal(w.riding, null, 'still on the slide');
    assert.ok(worst.step < 0.06 && worst.eye < 0.03 && worst.yaw < 0.2, `a jump on the slide: ${JSON.stringify(worst)}`);
    assert.ok(Math.abs(w.p.feet) < 1e-9 && w.p.ground, 'not stood on the ground at the end');
  });

  // Verifies: REQ-WALK-057
  it('gets off a swing with the eye where it was, then up to standing', () => {
    const { w } = walker(PLAYGROUND, seat, [1, 0, swing.pivot[2]]);
    w.playClick();
    run(w, 1);
    const before = w.p.feet + EYE;
    w.leaveRide(false);
    assert.ok(Math.abs(w.p.feet + EYE + w.eyeShift - before) < 1e-9, 'the eye jumped getting off');
    run(w, 1);
    assert.ok(Math.abs(w.eyeShift) < 0.01, 'the eye never got up to standing');
  });
});

const { posture, CONTACT } = await import('../static/walk/legs.js');
const { EYE, POINT } = await import('../static/walk/walkbase.js');
const { outfitAt, SKIN } = await import('../static/walk/hands.js');

describe('what the walker wears, and how their legs go', () => {
  // Verifies: REQ-WALK-059
  it('dresses the arms for the map: bare in a city, gloved and sleeved on a board and in the galaxy', () => {
    const HAND = 0.05, FOREARM = -0.15, UPPER = -0.5;
    assert.equal(outfitAt('city', HAND), SKIN);
    assert.equal(outfitAt('city', FOREARM), SKIN);
    assert.notEqual(outfitAt('city', UPPER), SKIN, 'no T-shirt sleeve in a city');
    for (const style of ['circuit', 'galaxy']) {
      for (const z of [HAND, FOREARM, UPPER]) assert.notEqual(outfitAt(style, z), SKIN, `bare skin on a ${style} arm at ${z}`);
    }
    assert.notEqual(outfitAt('circuit', HAND), outfitAt('circuit', FOREARM), 'an electrician\'s glove the color of the sleeve');
  });

  // Verifies: REQ-WALK-059
  it('sits on a seat, strides when walking and kicks with the right leg', () => {
    const at = (extra = {}) => ({ p: { ground: true }, riding: null, pace: 1, kicked: null, ...extra });
    const sitting = posture(at({ riding: { entry: { ride: 'swing' }, angle: 0, blend: 1 } }), 0, 0);
    assert.ok(sitting.thigh_L > 1.3 && sitting.thigh_R > 1.3 && sitting.shin_L < -1, 'not sitting on a swing');
    const stride = [0.5, 0.5 + Math.PI].map(phase => posture(at(), phase, 0));
    assert.ok(stride[0].thigh_L > 0.2 && stride[1].thigh_L < -0.2 && stride[0].thigh_R < -0.2, 'no stride');
    assert.equal(posture(at({ pace: 0 }), 1, 0).thigh_L, 0, 'standing still strides');
    // Through the kick: drawn back with the heel up, the knee whipped straight as the
    // foot meets the ball under the hips, swung on up, and stood again; the other knee
    // giving all the while.
    const kick = s => posture(at({ pace: 0, kicked: 0 }), 0, s * 1000);
    const back = kick(0.12), strike = kick(CONTACT), up = kick(0.36), done = kick(0.7);
    assert.ok(back.thigh_R < -0.4 && back.shin_R < -1.2, `drawn back ${back.thigh_R}, ${back.shin_R}`);
    assert.ok(Math.abs(strike.thigh_R) < 0.5 && strike.shin_R > -0.6 && strike.foot_R < -0.3, `at the ball ${JSON.stringify(strike)}`);
    assert.ok(up.thigh_R > 1 && up.shin_R > -0.3, `followed through ${up.thigh_R}`);
    assert.ok(back.shin_L < -0.2 && strike.shin_L < -0.2, 'the standing leg is stiff');
    assert.ok(done.thigh_R === 0, `still kicking: ${done.thigh_R}`);
    // Eased: no joint jumps from one frame to the next.
    let worst = 0;
    for (let t = 0; t < 0.7; t += 1 / 60) {
      const [a, b] = [kick(t), kick(t + 1 / 60)];
      for (const joint of ['thigh_R', 'shin_R', 'foot_R', 'thigh_L', 'shin_L']) worst = Math.max(worst, Math.abs((b[joint] || 0) - (a[joint] || 0)));
    }
    assert.ok(worst < 0.45, `a joint turns ${worst.toFixed(2)} in a frame`);
    // Sat low - a seesaw's end on the ground - the feet stay clear of it.
    const low = posture(at({ riding: { entry: { ride: 'rock' }, angle: 0, blend: 1 } }), 0, 0, 0.07);
    const drop = 0.115 * Math.cos(low.thigh_L) + 0.13 * Math.cos(low.thigh_L + low.shin_L);
    assert.ok(drop <= 0.07, `the feet go ${(drop - 0.07).toFixed(3)} into the ground`);
  });
});

const { layer, CLOTH } = await import('../static/walk/cloth.js');

describe('clothes as layers', () => {
  // Verifies: REQ-WALK-059
  it('cuts a sleeve straight across where it ends, stands it off the arm, and rims its end', () => {
    // An arm: a tube along z, skinned to one bone.
    const arm = new THREE.CylinderGeometry(0.03, 0.03, 0.4, 16, 20, true).rotateX(Math.PI / 2);
    const n = arm.getAttribute('position').count;
    arm.setAttribute('skinIndex', new THREE.Uint16BufferAttribute(new Array(n * 4).fill(0), 4));
    arm.setAttribute('skinWeight', new THREE.Float32BufferAttribute(Array.from({ length: n }, () => [1, 0, 0, 0]).flat(), 4));
    const sleeve = layer(arm, { to: -0.033, inflate: 0.004, color: '#2a9d8f', kind: CLOTH.knit });
    const p = sleeve.getAttribute('position');
    let furthest = -Infinity, onCut = 0, outside = 0, under = 0;
    for (let i = 0; i < p.count; i++) {
      const z = p.getZ(i), r = Math.hypot(p.getX(i), p.getY(i));
      furthest = Math.max(furthest, z);
      if (Math.abs(z + 0.033) < 1e-6) onCut++;
      if (Math.abs(r - 0.034) < 1e-3) outside++;
      if (r < 0.03) under++;
    }
    assert.ok(furthest <= -0.033 + 1e-6, `the sleeve runs on to ${furthest}, past its end`);
    assert.ok(onCut > 16, 'not cut straight across');
    assert.ok(outside > 0 && under > 0, 'no rim from the sleeve down to the arm at its end');
    assert.ok([...sleeve.getAttribute('cloth').array].every(k => k === CLOTH.knit));
  });
});

const { TOOLS } = await import('../static/walk/tools.js');

describe('empty hands', () => {
  const holding = () => Object.assign(Object.create(WALK.Walker.prototype), {
    active: false, primary: TOOLS.nailer, secondary: TOOLS.grapple, bare: false, stowed: null, showing: null,
    dry: new Set(), p: { fly: false }, hooks: {}, keys: new Set(), darts: [], firing: false, scene: {},
  });

  // Verifies: REQ-TOOL-032
  it('puts both tools down with H, so a click uses none, and takes them out again', () => {
    const w = holding();
    w.setHandsOff(true);
    assert.equal(w.secondary, null, 'the left hand still holds its tool');
    assert.ok(w.bare && w.handsOff && w.freeHands());
    let used = false;
    w.playClick = () => false;
    w.shotFrom = () => { used = true; return {}; };
    w.frozen = false;
    w.wheel = { open: false };
    w.dying = null;
    w.fire();
    assert.equal(used, false, 'a click with empty hands used the tool put down');
    w.setHandsOff(false);
    assert.equal(w.secondary, TOOLS.grapple, 'the left hand\'s tool did not come back');
    assert.ok(!w.bare && w.primary === TOOLS.nailer);
  });

  // Verifies: REQ-TOOL-032
  it('puts the right hand\'s tool down on its own key, and its key takes it out', () => {
    const w = holding();
    w.secondary = null;
    w.putDown();
    assert.ok(w.bare && w.freeHands());
    w.setTool('nailer');
    assert.ok(!w.bare);
  });
});
