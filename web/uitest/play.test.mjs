// Playing in the parks (walk/play.js), on a stand-in walker standing on flat ground by
// one amenity at a time, stepped at sixty frames a second: the rides, and the ball
// games played to the end - a shot, a penalty, a serve - with nothing but what a
// click and the keys would do.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const { AMENITIES, onAmenity } = await import('../static/map/amenities.js');
const WALK = await import('../static/walk/walk.js');

const CITY = AMENITIES.city;
const [PITCH, , VOLLEY, PLAYGROUND, ROUNDABOUT] = CITY;
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

  // Verifies: REQ-WALK-058
  it('puts a shot from the free-throw line through the hoop', () => {
    const random = Math.random;
    Math.random = () => 0.5;
    try {
      // On a board and in the galaxy as in a city.
      for (const style of ['city', 'circuit', 'galaxy']) {
        const court = AMENITIES[style].find(a => a.play.some(e => e.hoop));
        const { w, said } = walker(court, [-0.3, 0, 0], [-0.565, 0.25, 0]);
        ballAtFeet(w, 'basket');
        assert.equal(w.playClick(), true);
        assert.ok(w.ballHeld, 'the ball was not picked up');
        w.playClick();
        run(w, 3);
        assert.ok(said.some(s => /two points/.test(s)), `the shot in the ${style}: ${said}`);
      }
    } finally {
      Math.random = random;
    }
  });

  // Verifies: REQ-WALK-058
  it('kicks a penalty into the goal', () => {
    const random = Math.random;
    Math.random = () => 0.5;
    try {
      // A pitch laid either way round.
      for (const turn of [0, Math.PI / 2]) {
        const { w, said } = walker(PITCH, [0.55, 0, 0], [0.85, 0.05, 0], turn);
        ballAtFeet(w, 'soccer');
        w.playClick();
        run(w, 3);
        assert.ok(said.includes('Goal!'), `the kick: ${said}`);
        const ball = [...w.balls.values()][0];
        assert.ok(ball.vel.length() < 1, 'the net did not stop the ball');
      }
    } finally {
      Math.random = random;
    }
  });

  // Verifies: REQ-WALK-058
  it('serves over the net into the far half, standing or jumping', () => {
    for (const jump of [false, true]) {
      const { w, said } = walker(VOLLEY, [-0.8, 0, 0.1], [0.5, 0, 0], jump ? Math.PI / 2 : 0);
      ballAtFeet(w, 'volley');
      w.playClick();
      if (jump) Object.assign(w.p, { feet: 0.3, ground: false });
      w.playClick();
      run(w, 4);
      assert.ok(said.includes(jump ? 'Ace! A jump serve in' : 'In!'), `the ${jump ? 'jump ' : ''}serve: ${said}`);
    }
  });
});

const { posture } = await import('../static/walk/legs.js');
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
    const sitting = posture(at({ riding: { entry: { ride: 'swing' }, angle: 0, stage: 0 } }), 0, 0);
    assert.ok(sitting.thigh_L > 1.3 && sitting.thigh_R > 1.3 && sitting.shin_L < -1, 'not sitting on a swing');
    const stride = [0.5, 0.5 + Math.PI].map(phase => posture(at(), phase, 0));
    assert.ok(stride[0].thigh_L > 0.2 && stride[1].thigh_L < -0.2 && stride[0].thigh_R < -0.2, 'no stride');
    assert.equal(posture(at({ pace: 0 }), 1, 0).thigh_L, 0, 'standing still strides');
    // Through the kick: back first, then well forward, then down again.
    const kick = [0.1, 0.28, 0.6].map(s => posture(at({ pace: 0, kicked: 0 }), 0, s * 1000).thigh_R);
    assert.ok(kick[0] < 0 && kick[1] > 1 && kick[2] === 0, `the kick went ${kick}`);
  });
});
