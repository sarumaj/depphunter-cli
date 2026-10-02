// The parachute: what it does to the walker, and what it looks like doing it.
//
// It is the one secondary tool that is used once and then has to be put back
// together. The jet holds the walker up for as long as its tank lasts and the
// skimmers for as long as theirs; a parachute holds them up for exactly one descent,
// and afterwards it is a heap of nylon on the ground that takes a while to repack.
// So its tank is the pack itself (tools.js, fuel.once): full is packed and ready,
// opening it spends all of it at once, and it fills again only once the canopy is
// back on the ground or gone.
//
// Everything that moves the walker is in the first half of this file, and all of it
// is pure: a plain state object and a function that advances it, which is what the
// tests drive. It is advanced in fixed substeps rather than by the frame, because a
// canopy is a stiff thing to integrate - quadratic drag at the speed of a long fall is
// hundreds of units a second squared for the second it takes to open - and because a
// descent flown on a 144 Hz screen has to come down where the same descent flown at
// 60 Hz does. The second half builds and poses the model: a ram-air wing, its lines,
// risers and pilot chute, built out of whatever material the caller says - lit when it
// hangs over the walker's head in the view pass, unlit and bendable once it is left
// lying in the street or drifting away over it.
//
// Units are the map's: a unit is about three and a half meters (health.js) and walk
// mode's gravity is 13 units a second squared (walk.js), which is several times the
// real thing - the walker is small and the city is quick - so the speeds below are
// chosen against that gravity and then checked against life in meters a second.

import * as THREE from '../vendor/three.module.min.js';
import { clamp } from '../core/numbers.js';

// ------------------------------------------------------------------ the physics

/** Walk mode's gravity (walk.js GRAVITY), which the canopy's trim is solved against. */
const CHUTE_GRAVITY = 13;

// How long the pilot chute takes to drag the bag off the walker's back, and how long
// the canopy then takes to open. A second is what a real one takes from line stretch,
// and what makes a low opening a real risk rather than a technicality.
export const PILOT_TIME = 0.3, INFLATE_TIME = 1;
// How far over full the canopy opens before it settles - the snap a canopy gives as
// the cells fill all at once - and when in the opening that peaks, as a share of it.
const OVERSHOOT = 0.14, SNAP_AT = 0.775;
// The least height above what is below them from which the walker can throw it: off a
// roof or out of the jet, not off the top of a jump.
export const MIN_DEPLOY = 0.8;

// The canopy's trim, by how far the toggles are down (0 full flight, 1 full brakes):
// its forward speed and its sink, in units a second. Neutral is where the hands rest,
// a little way down; W lets the toggles up for the faster, steeper line; S holds them
// most of the way down, slow and sinking a little more. Neutral comes to about
// twelve meters a second forward and under three down - a glide of about 4, a
// docile canopy that leaves time to look around on the way down - and each point is
// solved into lift and drag for walk mode's gravity (coefficientsFor), so the canopy
// settles into exactly this trim.
export const TRIM = [
  { brake: 0, speed: 3.9, sink: 1.1 },
  { brake: 0.3, speed: 3.35, sink: 0.8 },
  { brake: 0.8, speed: 1.9, sink: 0.93 },
];
export const NEUTRAL_BRAKE = 0.3, DEEP_BRAKE = 0.8;
// How long the toggles take to travel, as the time constant of their easing.
const TOGGLE_TAU = 0.22;
// Turning: radians a second at full toggle, how quickly the canopy comes into a turn
// and out of it, what a toggle held down on one side adds to the drag - which is what
// makes a turn cost height as well as the bank - and how hard the canopy pulls the
// flight path round to where it is pointed.
const TURN_RATE = 1.2;
const TURN_TAU = 0.35, BANK_TAU = 0.4, TURN_DRAG = 0.5, SIDE_GRIP = 1.6;
// A canopy flown faster than its trim speed, the way the opening leaves it, is pushed
// off its angle and luffs: this much more drag for each share of speed over trim. It
// is nothing in steady flight and only bleeds off a surge, which a canopy this flat
// would otherwise turn back into height - a climb out of the opening, and a pendulum
// of a descent after it.
const SURGE_DRAG = 1.5;
// The flare: both toggles pulled down, progressively, which pitches the canopy up and
// trades its forward speed for lift. The pull is a quick first bite over FLARE_RISE
// and then a steady draw to the bottom by FLARE_TIME, adding up to FLARE_LIFT of lift
// and FLARE_DRAG of drag: pulled progressively, the lift grows as the speed it is made
// of runs down, which holds the sink near nothing for a second or so. A flare that
// runs out before the ground stalls the canopy for STALL_TIME - under half its lift,
// more drag - and it takes STALL_EASE to fly again.
export const FLARE_TIME = 1.5;
const FLARE_RISE = 0.15, FLARE_LIFT = 2.4, FLARE_DRAG = 2.6;
const STALL_TIME = 1.1;
const STALL_EASE = 0.8, STALL_LIFT = 0.45, STALL_DRAG = 1.3;
// The pilot chute's own drag while it drags the bag out, per unit of speed: next to
// nothing, which is why a pilot chute thrown low changes nothing about the landing.
const PILOT_DRAG = 0.01;
// The walker swinging under it: the length of the pendulum (the lines), how quickly a
// swing dies away (as a damping ratio), and how far the lines can stretch and give
// on the opening jolt, with how stiff and how damped that give is.
const LINE = 1.15;
const SWING_DAMPING = 0.22, SAG_RATE = 11, SAG_DAMPING = 0.35, SAG_GAIN = 0.0045, SAG_MAX = 0.12;
// The substep, and the most a frame is allowed to ask for: a frame that took a second
// (a tab brought back from the background) is not a second of canopy flight.
export const SUBSTEP = 1 / 240;
const MOST = 0.1;

/**
 * Lift and drag, per unit of mass and square of airspeed, for the canopy to fly at
 * `speed` forward and `sink` down in steady flight. In steady flight lift and drag
 * together hold up the walker's weight, and the ratio between them is the glide:
 * lift is gravity times the cosine of the glide path over the airspeed squared, drag
 * the same with the sine.
 */
function coefficientsFor({ speed, sink }) {
  const air = Math.hypot(speed, sink), path = Math.atan2(sink, speed);
  const k = CHUTE_GRAVITY / (air * air);
  return { lift: k * Math.cos(path), drag: k * Math.sin(path) };
}

// The trim points, solved once.
const SOLVED = TRIM.map(t => ({ brake: t.brake, ...coefficientsFor(t) }));

/** The canopy's lift and drag at `brake`, between the trim points. */
function trimAt(brake) {
  const b = clamp(brake, SOLVED[0].brake, SOLVED[SOLVED.length - 1].brake);
  let i = 0;
  while (i < SOLVED.length - 2 && b > SOLVED[i + 1].brake) i++;
  const a = SOLVED[i], c = SOLVED[i + 1], k = (b - a.brake) / (c.brake - a.brake);
  return { lift: a.lift + (c.lift - a.lift) * k, drag: a.drag + (c.drag - a.drag) * k };
}

/** A parachute on the walker's back, packed and not yet thrown. */
export function packedChute() {
  return {
    phase: 'packed', // packed, pilot, inflating, flying, landed, cutaway
    t: 0,            // seconds in this phase
    age: 0,          // seconds since it was thrown, for what is drawn
    x: 0, feet: 0, z: 0, vx: 0, vy: 0, vz: 0,
    heading: 0,      // where the canopy points, as a yaw (walk.js): not where the walker looks
    brake: NEUTRAL_BRAKE, turn: 0, turnRate: 0, bank: 0,
    flare: -1,       // seconds into a flare, -1 when there is none
    flareHeld: false, // Space still down from the press that started it
    swing: { pitch: 0, pitchRate: 0, roll: 0, rollRate: 0, sag: 0, sagRate: 0 },
    carry: 0,        // time asked for that has not yet made a whole substep
    touchdown: null, // how the canopy met the ground or a wall, once it has
  };
}

/**
 * Throws the pilot chute: from here the canopy is on its way out, and the walker
 * starts where `from` says - feet, position, velocity, and the heading they are
 * facing, which is the heading the canopy opens on.
 */
export function deployChute(chute, from) {
  Object.assign(chute, packedChute(), {
    phase: 'pilot', x: from.x, feet: from.feet, z: from.z,
    vx: from.vx || 0, vy: from.vy || 0, vz: from.vz || 0, heading: from.heading || 0,
    brake: NEUTRAL_BRAKE,
  });
  return chute;
}

/** Whether there is a canopy over the walker, or on its way out: the walker is its to fly. */
export const aloft = chute => chute?.phase === 'pilot' || chute?.phase === 'inflating' || chute?.phase === 'flying';

/**
 * How far open the canopy is: 0 while the pilot chute is still dragging the bag, 1 in
 * flight, and in between while it opens - a smoothstep that snaps past full near the
 * end, which is the canopy filling at once rather than easing into shape.
 *
 * Implements: REQ-TOOL-072
 */
export function opennessOf(chute) {
  if (chute.phase === 'pilot' || chute.phase === 'packed') return 0;
  if (chute.phase !== 'inflating') return 1;
  return inflation(chute.t / INFLATE_TIME);
}

/** The opening, u from 0 to 1 over INFLATE_TIME: a smoothstep with a snap past full. */
function inflation(u) {
  const k = clamp(u, 0, 1);
  const snap = OVERSHOOT * Math.sin(Math.PI * clamp((k - (2 * SNAP_AT - 1)) / (2 * (1 - SNAP_AT)), 0, 1));
  return k * k * (3 - 2 * k) + snap;
}

/**
 * How much of the canopy's lift is lift yet: none while it is a bundle of fabric
 * dragging behind the walker, all of it once the cells have filled. An opening
 * canopy is almost all drag, which is what stops a fall.
 */
const liftShare = open => clamp((open - 0.35) / 0.6, 0, 1);

/**
 * What the flare is doing at `t` seconds into it: [lift, drag] multipliers. The first
 * bite comes in over FLARE_RISE and the rest of the pull follows through FLARE_TIME;
 * after that the canopy is stalled for STALL_TIME and eases back to flying over
 * STALL_EASE.
 *
 * Implements: REQ-TOOL-074
 */
function flareAt(t) {
  if (t < 0) return [1, 1];
  if (t < FLARE_TIME) {
    const bite = Math.min(1, t / FLARE_RISE), pull = t / FLARE_TIME;
    return [1 + FLARE_LIFT * (0.2 * bite + 0.8 * pull ** 1.5), 1 + FLARE_DRAG * (0.3 * bite + 0.7 * pull)];
  }
  const after = t - FLARE_TIME;
  if (after < STALL_TIME) return [STALL_LIFT, STALL_DRAG];
  const back = clamp((after - STALL_TIME) / STALL_EASE, 0, 1);
  return [STALL_LIFT + (1 - STALL_LIFT) * back, STALL_DRAG + (1 - STALL_DRAG) * back];
}

/** Whether a flare has run out and left the canopy stalled. */
export const stalled = chute => chute.flare >= FLARE_TIME && chute.flare < FLARE_TIME + STALL_TIME + STALL_EASE;

/**
 * The walker under the canopy, over `deltaTime` seconds of what they are doing with
 * the toggles (`input`: forward +1 for W and -1 for S, turn +1 to the left and -1 to
 * the right, flare while Space is down). `ground(x, z, feet)` is the top of whatever
 * is under a point, and `climb` how far above the feet it may be and still be
 * something to slide over rather than a wall.
 *
 * Returns what happened on the way, oldest first: {kind: 'wall', speed} for a wall
 * struck at `speed`, and {kind: 'touchdown', speed, vertical, horizontal, open} for
 * the ground, after which the canopy is on the ground (phase 'landed') and stops
 * moving the walker.
 *
 * Implements: REQ-TOOL-072, REQ-TOOL-073, REQ-TOOL-074, REQ-TOOL-075
 */
export function stepChute(chute, input, deltaTime, ground, climb = 0.15) {
  const events = [];
  if (!aloft(chute)) return events;
  // A press, not a hold: Space still down from the flare it started does not start
  // another the moment the stall is over.
  if (input.flare && !chute.flareHeld && chute.phase === 'flying' && chute.flare < 0) chute.flare = 0;
  chute.flareHeld = !!input.flare;
  chute.carry = Math.min(MOST, chute.carry + Math.max(0, deltaTime));
  while (chute.carry >= SUBSTEP && aloft(chute)) {
    chute.carry -= SUBSTEP;
    substep(chute, input, ground, climb, events);
  }
  return events;
}

// Scratch vectors for a substep, so that flying allocates nothing.
const AIR = new THREE.Vector3(), RIGHT = new THREE.Vector3(), UP = new THREE.Vector3(), LIFT = new THREE.Vector3();

/** One SUBSTEP of flight. */
function substep(chute, input, ground, climb, events) {
  const h = SUBSTEP;
  chute.t += h;
  chute.age += h;
  if (chute.phase === 'pilot' && chute.t >= PILOT_TIME) { chute.phase = 'inflating'; chute.t = 0; }
  if (chute.phase === 'inflating' && chute.t >= INFLATE_TIME) { chute.phase = 'flying'; chute.t = 0; }
  if (chute.flare >= 0) chute.flare += h;
  if (chute.flare >= FLARE_TIME + STALL_TIME + STALL_EASE) chute.flare = -1;

  // The toggles, eased toward where the keys hold them. A flare holds both all the
  // way down while it lasts, whatever else is pressed.
  const toggles = 1 - Math.exp(-h / TOGGLE_TAU);
  const flaring = chute.flare >= 0 && chute.flare < FLARE_TIME;
  const want = flaring ? 1 : input.forward > 0 ? 0 : input.forward < 0 ? DEEP_BRAKE : NEUTRAL_BRAKE;
  chute.brake += (want - chute.brake) * toggles;
  chute.turn += (clamp(input.turn || 0, -1, 1) - chute.turn) * toggles;

  const open = opennessOf(chute);
  const flying = liftShare(open);
  // The canopy flies through the air, and the air moves (ballistics.js Breeze): lift
  // and drag answer to the speed through it, and the canopy is carried over the
  // ground by the wind on top of that - so it drifts, it makes no headway into a
  // strong one, and a landing into the wind is a slower one over the ground.
  // Implements: REQ-TOOL-081
  const wind = chute.wind;
  AIR.set(chute.vx - (wind?.x || 0), chute.vy, chute.vz - (wind?.z || 0));
  const speed = AIR.length();
  // The heading only answers the toggles once the canopy is a wing, and a bank is
  // what a turn at this airspeed needs: the lift tilted just enough to carry it round.
  chute.turnRate += (TURN_RATE * chute.turn * flying - chute.turnRate) * (1 - Math.exp(-h / TURN_TAU));
  chute.heading += chute.turnRate * h;
  const level = Math.hypot(AIR.x, AIR.z);
  const bank = Math.atan2(level * chute.turnRate, CHUTE_GRAVITY);
  chute.bank += (bank - chute.bank) * (1 - Math.exp(-h / BANK_TAU));
  const trim = trimAt(flaring ? NEUTRAL_BRAKE : Math.min(chute.brake, DEEP_BRAKE));
  const [flareLift, flareDrag] = flareAt(chute.flare);
  // An opening canopy is a drogue: drag in proportion to how much of it has filled,
  // and several times as much drag for its size as the wing it becomes.
  const lift = trim.lift * open * flying * flareLift;
  // Faster than trim, the wing luffs (SURGE_DRAG) - once it is a wing: an opening
  // canopy is still the drogue above. Trim speed is where lift and drag together hold
  // up the walker's weight.
  const surge = Math.max(0, speed / Math.sqrt(CHUTE_GRAVITY / Math.hypot(trim.lift, trim.drag)) - 1);
  const drag = trim.drag * open * (1 + 2.5 * (1 - flying)) * flareDrag * (1 + TURN_DRAG * Math.abs(chute.turn) * flying)
      * (1 + SURGE_DRAG * surge * flying)
    + (chute.phase === 'pilot' || open < 0.2 ? PILOT_DRAG : 0);
  let ax = 0, ay = -CHUTE_GRAVITY, az = 0;
  if (speed > 1e-6) {
    ax -= drag * speed * AIR.x;
    ay -= drag * speed * AIR.y;
    az -= drag * speed * AIR.z;
    // Lift is square to the airflow and to the span, and the span is banked into the
    // turn: RIGHT is the right wingtip, raised by the bank in a left turn.
    const s = Math.sin(chute.heading), c = Math.cos(chute.heading);
    RIGHT.set(c, 0, -s).multiplyScalar(Math.cos(chute.bank)).add(UP.set(0, Math.sin(chute.bank), 0));
    LIFT.crossVectors(RIGHT, AIR);
    const across = LIFT.length();
    if (across > 1e-6) {
      const k = lift * speed * speed / across;
      ax += LIFT.x * k;
      ay += LIFT.y * k;
      az += LIFT.z * k;
    }
    // A wing flies where it points: whatever of the airflow comes from the side is
    // taken out of it, which is what carries the flight path round a turn.
    const side = AIR.x * c - AIR.z * s;
    const grip = SIDE_GRIP * open * speed * side;
    ax -= grip * c;
    az += grip * s;
  }
  chute.vx += ax * h;
  chute.vy += ay * h;
  chute.vz += az * h;

  swing(chute, ax, ay, az, h);

  // Axis by axis, the way the walker slides along a wall on foot (walk.js step). A
  // wall is anything more than a step above the feet; running into one stops the
  // canopy's way on that axis and costs what arriving at that speed would.
  const nx = chute.x + chute.vx * h;
  if (ground(nx, chute.z, chute.feet) <= chute.feet + climb) chute.x = nx;
  else { events.push({ kind: 'wall', speed: Math.abs(chute.vx) }); chute.vx = 0; }
  const nz = chute.z + chute.vz * h;
  if (ground(chute.x, nz, chute.feet) <= chute.feet + climb) chute.z = nz;
  else { events.push({ kind: 'wall', speed: Math.abs(chute.vz) }); chute.vz = 0; }
  chute.feet += chute.vy * h;
  const floor = ground(chute.x, chute.z, chute.feet);
  if (chute.feet <= floor) {
    const horizontal = Math.hypot(chute.vx, chute.vz), vertical = Math.max(0, -chute.vy);
    chute.touchdown = { kind: 'touchdown', speed: touchdownSpeed(vertical, horizontal), vertical, horizontal, open: Math.min(1, open) };
    events.push(chute.touchdown);
    chute.feet = floor;
    chute.phase = 'landed';
    chute.t = 0;
  }
}

/**
 * The walker hanging under the canopy: a pendulum on the lines, swinging fore and aft
 * when the canopy speeds up or slows down under them and leaning out into a turn,
 * and a stiff spring along the lines that gives on the opening jolt. Semi-implicit,
 * in the substep, so it is as steady at one frame rate as another.
 *
 * Implements: REQ-TOOL-076
 */
function swing(chute, ax, ay, az, h) {
  const w = chute.swing;
  const rate = Math.sqrt(CHUTE_GRAVITY / LINE);
  // What the canopy's own acceleration along its heading does to the walker below it:
  // it brakes, and they carry on forward, swinging out in front of it.
  const s = Math.sin(chute.heading), c = Math.cos(chute.heading);
  const along = -ax * s - az * c;
  w.pitchRate += (-rate * rate * w.pitch - 2 * SWING_DAMPING * rate * w.pitchRate - along / LINE) * h;
  w.pitch += w.pitchRate * h;
  // Sideways they lean out with the bank, and overshoot it a little on the way in.
  w.rollRate += (-rate * rate * (w.roll - chute.bank) - 2 * SWING_DAMPING * rate * w.rollRate) * h;
  w.roll += w.rollRate * h;
  // Along the lines: whatever holds the walker up beyond their own weight - the opening
  // shock, and a flare - stretches them. Steady flight is the rest length.
  const pull = Math.max(0, ay);
  w.sagRate += (-SAG_RATE * SAG_RATE * w.sag - 2 * SAG_DAMPING * SAG_RATE * w.sagRate - pull * SAG_GAIN * SAG_RATE * SAG_RATE) * h;
  w.sag = clamp(w.sag + w.sagRate * h, -SAG_MAX, SAG_MAX);
}

// What a landing is judged on: the sink, and some of the speed along the ground - a
// canopy landing is run out, so the legs take a share of the forward speed and not all
// of it.
const HORIZONTAL_SHARE = 0.4;

/**
 * The speed a touchdown is judged at, in units a second.
 *
 * Implements: REQ-TOOL-075
 */
function touchdownSpeed(vertical, horizontal) {
  return Math.hypot(vertical, HORIZONTAL_SHARE * horizontal);
}

/** The drop a fall at `speed` would have been, for a touchdown judged as a fall. */
export const dropFor = speed => (speed * speed) / (2 * CHUTE_GRAVITY);

/**
 * Lets the canopy go: it flies on, empty, and the walker falls from where they are.
 * What was the walker's velocity stays with them.
 *
 * Implements: REQ-TOOL-078
 */
export function cutAway(chute) {
  if (!aloft(chute)) return false;
  chute.phase = 'cutaway';
  chute.t = 0;
  return true;
}

// ------------------------------------------------------------------ the model

// The wing: its span and chord in map units - about seven and a half meters by two and
// three quarters, a student's canopy - how many cells it has, the airfoil's thickness
// and camber for its chord, and the radius of the arc the span is curved on, which is
// what makes the tips hang lower than the middle.
const SPAN = 2.1, CHORD = 0.78, CELLS = 9, THICKNESS = 0.17, CAMBER = 0.035, ARC = 1.9;
// How finely it is drawn: columns across a cell, rows along the chord of a surface.
const SEGMENTS = 6, ROWS = 14;
// How far back from the nose the lower surface begins: the cell mouths, which is how
// a ram-air canopy fills.
const MOUTH = 0.07;
// Where along the chord the lines are sewn to the ribs (the A to D lines), and where
// along the half span the brake lines fan out from the tail.
const LINE_ROWS = [0.07, 0.3, 0.56, 0.84];
const BRAKE_AT = [0.18, 0.42, 0.66, 0.92];
// Where the risers leave the harness - on top of the walker's shoulders, level with
// the rig's origin, out to the sides of the head and a little behind the eyes, so
// that they start outside the view's near edge - and where they end at the links the
// lines are gathered on, up and further out; and how far the lines from the tail are
// gathered below the canopy before they run down to the toggle.
const SHOULDER = 0.17, SHOULDER_BACK = 0.03, LINK_X = 0.25, LINKS = 0.42, CASCADE = 0.55;
// The links' depth, front riser and back.
const FRONT_LINK = -0.02, BACK_LINK = 0.035;
// The risers as webbing: how wide and thick the strap is, and how far up it the strap
// fades in - the part of it that passes the eyes is nearer the lens than anything
// else hanging there, and at full strength it would be a bar across the view.
const RISER_WIDTH = 0.0035, RISER_THICKNESS = 0.0015, RISER_FADE = [0.22, 0.62];
// How far down the lines the slider comes once the canopy is open, from where the
// lines are sewn on toward the links, and how far its fabric bunches up toward its
// middle as it does: it stays up near the canopy, collapsed under it, rather than
// coming all the way down to the links in front of the eyes.
const SLIDER_DOWN = 0.18, SLIDER_GATHER = 0.6;
// The cells, left to right, in the colors a canopy is sewn in: white and red, with
// the middle cell blue so the heading can be read off it from below.
const CELL_COLORS = ['#f2f4f7', '#e4402f', '#f2f4f7', '#e4402f', '#2f7fb8', '#e4402f', '#f2f4f7', '#e4402f', '#f2f4f7'];
// The fabric and the hardware.
const LINE_COLOR = '#3a3f46', RISER_COLOR = '#2a2e33', LINK_COLOR = '#6d737a', PILOT_COLOR = '#e4402f', BAG_COLOR = '#39424c';
// How far the tail flutters at a given airspeed, and how fast; and how the cells
// breathe as the air through the mouths comes and goes.
const FLUTTER = 0.012, FLUTTER_RATE = 21, BREATHE = 0.035, BREATHE_RATE = 2.3;
// How far ahead of where the walker came down a landed canopy ends up lying: it flies
// on over them as they stop (walk.js looks there for the ground it lies on).
export const DRAPE_AHEAD = 1.25;

/** The airfoil's half thickness at u along the chord (0 the nose, 1 the tail), in chords. */
const halfThickness = u =>
  5 * THICKNESS * (0.2969 * Math.sqrt(u) - 0.126 * u - 0.3516 * u * u + 0.2843 * u ** 3 - 0.1015 * u ** 4);
/** ... and the camber line under it. */
const camberAt = u => 4 * CAMBER * u * (1 - u);

/**
 * The wing as a surface to be reshaped every frame: the positions are worked out from
 * where each vertex sits on the canopy (its span, its chord, which surface) and from
 * how open, braked and draped the canopy is at the time, so opening, breathing and
 * collapsing are all the same function of a few numbers.
 *
 * Every cell has its own columns of vertices, so the seams at the ribs are creases in
 * the shading and a sharp edge between two colors, the way sewn panels look.
 */
function wingGeometry(shade) {
  const where = [];  // [s, u, surface, depth] per vertex
  const colors = [];
  const index = [];
  const color = new THREE.Color();
  const add = (s, u, surface, depth, cell, tone) => {
    where.push(s, u, surface, depth);
    color.set(CELL_COLORS[cell]).multiplyScalar(tone);
    colors.push(color.r, color.g, color.b);
    return where.length / 4 - 1;
  };
  const rows = Array.from({ length: ROWS + 1 }, (_, i) => (1 - Math.cos((Math.PI * i) / ROWS)) / 2);
  for (let c = 0; c < CELLS; c++) {
    const grid = [[], [], []]; // upper, lower, and the mouth between them
    for (let k = 0; k <= SEGMENTS; k++) {
      const s = -1 + (2 * (c + k / SEGMENTS)) / CELLS;
      // A cell is darker toward its ribs, where the fabric is drawn in, and the tips
      // turn away from the sky - in the unlit copy that is all the shading there is.
      const rib = 1 - shade.rib * (1 - Math.sin((Math.PI * k) / SEGMENTS));
      const tip = 1 - shade.tip * s * s;
      grid[0].push(rows.map(u => add(s, u, 0, 0, c, shade.upper * rib * tip)));
      grid[1].push(rows.map(u => add(s, MOUTH + u * (1 - MOUTH), 1, 0, c, shade.lower * rib)));
      grid[2].push([add(s, 0, 2, 0, c, shade.mouth)]);
    }
    const quad = (a, b, d, e) => index.push(a, b, d, b, e, d);
    for (let k = 0; k < SEGMENTS; k++) {
      for (let i = 0; i < ROWS; i++) {
        quad(grid[0][k][i], grid[0][k + 1][i], grid[0][k][i + 1], grid[0][k + 1][i + 1]);
        quad(grid[1][k][i], grid[1][k][i + 1], grid[1][k + 1][i], grid[1][k + 1][i + 1]);
      }
      // The mouth: from the nose of the top skin back into the cell and out again to
      // the front of the bottom one - a dark hollow where the air goes in.
      quad(grid[0][k][0], grid[2][k][0], grid[0][k + 1][0], grid[2][k + 1][0]);
      quad(grid[2][k][0], grid[1][k][0], grid[2][k + 1][0], grid[1][k + 1][0]);
    }
  }
  // The stabilizers: a fin of fabric hanging from each tip, which is what keeps a
  // ram-air canopy flying straight.
  for (const side of [-1, 1]) {
    const cell = side < 0 ? 0 : CELLS - 1;
    const along = [0.1, 0.3, 0.5, 0.7, 0.9];
    const top = along.map(u => add(side, u, 1, 0, cell, shade.lower));
    const bottom = along.map(u => add(side, u, 3, Math.sin((Math.PI * (u - 0.1)) / 0.8), cell, shade.lower * 0.92));
    for (let i = 0; i < along.length - 1; i++) index.push(top[i], bottom[i], top[i + 1], bottom[i], bottom[i + 1], top[i + 1]);
  }
  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute('position', new THREE.Float32BufferAttribute(new Float32Array(where.length / 4 * 3), 3));
  geometry.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  geometry.setIndex(index);
  geometry.userData.where = Float32Array.from(where);
  return geometry;
}

// Scratch for the shape: one point, so that reshaping allocates nothing.
const AT = new THREE.Vector3();

/**
 * Where a point of the canopy is, in the canopy's own frame (+x the right tip, +y up,
 * the nose toward -z), for `shape`:
 *   span, chord  how far the canopy has spread, as shares of its full size
 *   arc          how curved the span is: 1 as built, more while it is still curled
 *                up, 0 flat on the ground
 *   thick        how much air is in it: 1 flying, less as it empties
 *   cells        how full each cell is, 0 to 1
 *   left, right  how far each toggle has pulled that side of the tail down
 *   flutter      how far the tail shivers, and `time` for when
 *   drape        0 flying, 1 lying in a heap of folds
 */
export function canopyPoint(s, u, surface, depth, shape, out = AT) {
  const cell = Math.min(CELLS - 1, Math.max(0, Math.floor(((s + 1) / 2) * CELLS)));
  const q = ((s + 1) / 2) * CELLS - cell;
  const pillow = Math.sin(Math.PI * clamp(q, 0, 1));
  const chord = CHORD * shape.chord;
  const full = (shape.cells[cell] ?? 1) * (1 + BREATHE * Math.sin(shape.time * BREATHE_RATE + cell * 1.3));
  const thick = halfThickness(u) * full * shape.thick;
  let h = surface === 0 ? camberAt(u) + thick * (1 + 0.22 * pillow)
    : surface === 1 || surface === 3 ? 0.25 * camberAt(u) - 0.3 * thick * (1 + 0.1 * pillow)
      : camberAt(0.03) * 0.6; // the mouth, set back in the cell
  // The stabilizer hangs below the tip rib.
  if (surface === 3) h -= 0.17 * depth * Math.min(1, shape.cells[cell] ?? 1) * (1 - (shape.drape || 0));
  // The tail: pulled down by the toggle on its side, more toward the tip, and never
  // quite still in the air going past it.
  const tail = Math.max(0, (u - 0.55) / 0.45);
  const pull = s < 0 ? shape.left : shape.right;
  h -= 0.32 * pull * (0.35 + 0.65 * Math.abs(s)) * tail * tail;
  const edge = Math.max(0, (u - 0.72) / 0.28);
  h += shape.flutter * Math.sin(shape.time * FLUTTER_RATE + s * 7 + cell * 0.9) * edge * edge / CHORD;
  // Lying down, the canopy is a sheet in folds: the airfoil pressed flat and the
  // fabric rucked up in a few long waves across it.
  if (shape.drape > 0) {
    const d = shape.drape;
    h = h * (1 - 0.9 * d) + d * 0.045 * (Math.sin(s * 9.3 + 1.1) * Math.sin(u * 6.1 + s * 2) + 0.6) / CHORD;
  }
  // The span on its arc, and the airfoil stood up square to it.
  const half = (SPAN / 2) * shape.span;
  const radius = ARC / Math.max(1e-3, shape.arc);
  const angle = (s * half) / radius;
  const cx = radius * Math.sin(angle), cy = radius * (Math.cos(angle) - 1);
  const nx = Math.sin(angle), ny = Math.cos(angle);
  // A cell's nose bulges forward a little between its ribs.
  const nose = surface === 2 ? 0.035 : 0.012 * pillow * (1 - u) ** 4;
  return out.set(cx + nx * h * chord, cy + ny * h * chord, (u - 0.5) * chord - nose * chord * full);
}

/**
 * Reshapes the wing mesh for `shape` (canopyPoint). The lit copy needs its normals
 * again afterwards; the unlit one never reads them.
 */
function reshape(mesh, shape, normals) {
  const geometry = mesh.geometry, where = geometry.userData.where;
  const position = geometry.getAttribute('position');
  for (let i = 0, n = position.count; i < n; i++) {
    const p = canopyPoint(where[i * 4], where[i * 4 + 1], where[i * 4 + 2], where[i * 4 + 3], shape);
    position.setXYZ(i, p.x, p.y, p.z);
  }
  position.needsUpdate = true;
  if (normals) geometry.computeVertexNormals();
  geometry.computeBoundingSphere();
}

// Where the right toggle is stowed on its rear riser, and the left one too when no hand
// is holding it: most of the way up the riser, on its outside.
const STOWED = new THREE.Vector3(-(SHOULDER + (LINK_X - SHOULDER) * 0.72) - 0.004, LINKS * 0.72,
  SHOULDER_BACK + (BACK_LINK - SHOULDER_BACK) * 0.72 - 0.001);

// A strap from one point to another, for the risers: webbing that fades in from
// nothing at `from` over `fade` (the fractions of the way along it where it starts
// and finishes coming in).
const ALONG_Y = new THREE.Vector3(0, 1, 0);
function strap(make, from, to, color, fade) {
  const run = new THREE.Vector3().subVectors(to, from);
  const length = run.length();
  const geometry = new THREE.BoxGeometry(RISER_WIDTH, length, RISER_THICKNESS, 1, 12, 1);
  const positions = geometry.getAttribute('position');
  const colors = new Float32Array(positions.count * 4);
  for (let i = 0; i < positions.count; i++) {
    const along = clamp((positions.getY(i) / length + 0.5 - fade[0]) / (fade[1] - fade[0]), 0, 1);
    colors.set([1, 1, 1, along * along * (3 - 2 * along)], i * 4);
  }
  geometry.setAttribute('color', new THREE.Float32BufferAttribute(colors, 4));
  const mesh = make(geometry, color, { vertexColors: true, transparent: true, depthWrite: false, shininess: 4 });
  mesh.position.copy(from).addScaledVector(run, 0.5);
  mesh.quaternion.setFromUnitVectors(ALONG_Y, run.normalize());
  return mesh;
}

/**
 * The canopy and everything between it and the walker: the wing, its lines, the four
 * risers and the slider on them, the pilot chute on its bridle and the bag it pulls
 * out. The rig's origin is where the risers meet the harness, at the walker's
 * shoulders, with +y up the lines and the canopy's nose toward -z.
 *
 * `make(geometry, color, options)` makes a mesh and `lineMaterial(color, options)` a
 * line material, which is how one rig is built twice: lit, over the walker's head in
 * the pass that draws what they hold, and unlit and bent around the planet once it
 * has been left behind in the street (LooseCanopy).
 *
 * Implements: REQ-TOOL-071
 */
export function canopyRig(make, lineMaterial, lit = true) {
  const rig = new THREE.Group();
  rig.name = 'canopy-rig';
  // Lit, the wing is shaded by the view's lights and only the mouths and the underside
  // are darkened in; unlit, the shading is all baked in.
  const shade = lit ? { upper: 1, lower: 0.82, mouth: 0.38, rib: 0.08, tip: 0 }
    : { upper: 0.96, lower: 0.62, mouth: 0.3, rib: 0.14, tip: 0.22 };
  const canopy = new THREE.Group();
  canopy.name = 'canopy';
  const wing = make(wingGeometry(shade), '#ffffff', { vertexColors: true, side: THREE.DoubleSide, shininess: 12 });
  wing.name = 'wing';
  canopy.add(wing);
  rig.add(canopy);

  // The lines, all of them one set of segments rewritten every frame: forty from the
  // ribs to the links, eight from the tail to the two cascades, two from the cascades
  // to the toggles, and the bridle to the pilot chute - each in LINE_PIECES pieces,
  // so that a line can go slack.
  const count = ((CELLS + 1) * LINE_ROWS.length + BRAKE_AT.length * 2 + 2 + 1) * LINE_PIECES;
  const lines = new THREE.LineSegments(new THREE.BufferGeometry(), lineMaterial(LINE_COLOR));
  lines.geometry.setAttribute('position', new THREE.Float32BufferAttribute(new Float32Array(count * 6), 3));
  lines.name = 'lines';
  rig.add(lines);

  // The risers, front and back on each shoulder, up to the links.
  const risers = new THREE.Group();
  risers.name = 'risers';
  for (const side of [-1, 1]) {
    for (const [z0, z1] of [[SHOULDER_BACK - 0.012, FRONT_LINK], [SHOULDER_BACK + 0.012, BACK_LINK]]) {
      const from = new THREE.Vector3(side * SHOULDER, 0, z0), to = new THREE.Vector3(side * LINK_X, LINKS, z1);
      risers.add(strap(make, from, to, RISER_COLOR, RISER_FADE));
      const link = make(new THREE.TorusGeometry(0.006, 0.0016, 5, 10), LINK_COLOR, { shininess: 8 });
      link.position.copy(to);
      risers.add(link);
    }
    // The toggle on the right hand's side stays stowed on its riser: that hand is
    // hunting. The left one is in the left hand (tools.js).
    if (side > 0) {
      const toggle = make(new THREE.BoxGeometry(0.005, 0.02, 0.004), '#c98a14');
      toggle.position.copy(STOWED).setX(side * Math.abs(STOWED.x));
      risers.add(toggle);
    }
  }
  rig.add(risers);

  // The slider: a square of fabric with a grommet at each corner that the four line
  // groups run through, which slows the opening; it comes down to the links as the
  // canopy spreads.
  const slider = make(new THREE.BufferGeometry(), '#e6ebf0', { side: THREE.DoubleSide, transparent: true, opacity: 0.18, depthWrite: false });
  slider.geometry.setAttribute('position', new THREE.Float32BufferAttribute(new Float32Array(12), 3));
  slider.geometry.setIndex([0, 1, 2, 0, 2, 3]);
  slider.name = 'slider';
  rig.add(slider);

  // The pilot chute: a small dome of fabric over a mesh skirt, which is what is thrown
  // and what drags everything else out after it.
  const pilot = new THREE.Group();
  pilot.name = 'pilot';
  pilot.add(make(new THREE.SphereGeometry(0.075, 14, 6, 0, Math.PI * 2, 0, Math.PI / 2).scale(1, 0.62, 1),
    PILOT_COLOR, { side: THREE.DoubleSide }));
  pilot.add(make(new THREE.CylinderGeometry(0.075, 0.018, 0.08, 14, 1, true).translate(0, -0.04, 0), '#2b3138',
    { side: THREE.DoubleSide, transparent: true, opacity: 0.55 }));
  pilot.add(make(new THREE.CylinderGeometry(0.007, 0.007, 0.03, 6).translate(0, 0.05, 0), '#f2f4f7'));
  rig.add(pilot);
  const bag = make(new THREE.BoxGeometry(0.1, 0.065, 0.14), BAG_COLOR);
  bag.name = 'bag';
  rig.add(bag);

  rig.traverse(o => { o.frustumCulled = false; });
  rig.userData.lit = lit;
  return rig;
}

// Scratch for posing a rig, so that a frame of it allocates nothing.
const LINK = new THREE.Vector3(), TIP = new THREE.Vector3(), JOIN = new THREE.Vector3();
const TOP = new THREE.Vector3(), PC = new THREE.Vector3(), HAND = new THREE.Vector3(), BACK = new THREE.Vector3();
const CONTAINER = new THREE.Vector3(0, -0.28, 0.12);
const THROWN = [new THREE.Vector3(-0.25, -0.05, -0.45), new THREE.Vector3(-0.4, 0.15, -0.75), new THREE.Vector3(0, 0.95, 1.2)];
const GROUPS = Array.from({ length: 4 }, () => new THREE.Vector3()), PIECE = new THREE.Vector3();
// How many pieces each line is drawn in, so that a slack one can sag.
export const LINE_PIECES = 4;

/**
 * What an open canopy looks like at a moment of `chute`'s flight: how far open, how
 * its toggles are set, how hard the air is going past it. `now` is milliseconds.
 */
export function lookOf(chute, now) {
  const flaring = chute.flare >= 0 && chute.flare < FLARE_TIME;
  return {
    phase: chute.phase, t: chute.t, open: opennessOf(chute), brake: chute.brake, turn: chute.turn,
    flare: flaring ? chute.flare / FLARE_TIME : 0, stall: stalled(chute),
    speed: Math.hypot(chute.vx, chute.vy, chute.vz), time: now / 1000, drape: 0, deflate: 0,
  };
}

/**
 * The shape of the wing for a `look`: how far it has spread and filled as it opens
 * (the middle cells first, the tips last, a snap past full at the end), the tail
 * where the toggles have it, and the flutter the air puts into it.
 */
export function shapeOf(look, shape = { cells: new Float32Array(CELLS) }) {
  const open = look.open, spread = Math.min(1, open), over = Math.max(0, open - 1);
  shape.span = 0.12 + 0.88 * spread + 0.45 * over;
  shape.chord = 0.45 + 0.55 * spread + 0.2 * over;
  shape.arc = (1 + 1.6 * (1 - spread)) * (1 - (look.drape || 0));
  shape.thick = 1 - 0.8 * (look.deflate || 0);
  for (let c = 0; c < CELLS; c++) {
    const out = Math.abs(c - (CELLS - 1) / 2) / ((CELLS - 1) / 2);
    shape.cells[c] = clamp(open * 1.7 - out * 0.7, 0.06, 1.15);
  }
  const both = look.flare > 0 ? 1 : look.brake;
  shape.left = clamp(both + Math.max(0, look.turn) * 0.9, 0, 1.2);
  shape.right = clamp(both + Math.max(0, -look.turn) * 0.9, 0, 1.2);
  shape.flutter = FLUTTER * clamp(look.speed / 4, 0, 1.5) * spread * (look.stall ? 2.5 : 1) * (1 - (look.drape || 0));
  shape.time = look.time;
  shape.drape = look.drape || 0;
  return shape;
}

/**
 * Poses a rig for a `look` (lookOf): where the pilot chute and the bag are while the
 * canopy is coming out, the canopy's height on its lines, its pitch and its shape,
 * and every line between it and the harness. `hand` is where the left toggle is, in
 * the rig's frame, when a hand is holding it; otherwise it is stowed on its riser.
 *
 * Implements: REQ-TOOL-071, REQ-TOOL-072
 */
export function poseRig(rig, look, hand = null) {
  const canopy = rig.getObjectByName('canopy'), wing = rig.getObjectByName('wing');
  const lines = rig.getObjectByName('lines'), risers = rig.getObjectByName('risers');
  const slider = rig.getObjectByName('slider'), pilot = rig.getObjectByName('pilot'), bag = rig.getObjectByName('bag');
  const shape = rig.userData.shape ||= { cells: new Float32Array(CELLS) };
  const points = lines.geometry.getAttribute('position');
  let n = 0;
  // One line from a to b, in pieces. Taut in flight; slack on a canopy that has been
  // let go of, sagging by `look.slack` in the middle but never below the lower of its
  // two ends - so a line on the ground lies on it, and no line ever rises above the
  // canopy or drops out of the space between the canopy and the harness.
  const slack = look.slack || 0;
  const at = (a, b, k, out) => {
    out.lerpVectors(a, b, k);
    out.y = Math.max(Math.min(a.y, b.y), out.y - slack * 4 * k * (1 - k));
    points.setXYZ(n++, out.x, out.y, out.z);
  };
  const segment = (a, b) => {
    for (let i = 0; i < LINE_PIECES; i++) {
      at(a, b, i / LINE_PIECES, PIECE);
      at(a, b, (i + 1) / LINE_PIECES, PIECE);
    }
  };

  // The pilot chute's throw, and the bag it drags off the walker's back: from the
  // left hand out and up behind them, trailing the bridle from the container.
  if (look.phase === 'pilot') {
    // Out of the hand to the side, and up and back into the air behind: a curve
    // through the three points, fast at first and slowing as the pilot chute
    // catches the air.
    const k = clamp(look.t / PILOT_TIME, 0, 1), e = 1 - (1 - k) ** 2;
    PC.set(0, 0, 0).addScaledVector(THROWN[0], (1 - e) ** 2).addScaledVector(THROWN[1], 2 * e * (1 - e)).addScaledVector(THROWN[2], e * e);
    pilot.visible = true;
    pilot.position.copy(PC);
    pilot.quaternion.setFromUnitVectors(ALONG_Y, JOIN.subVectors(PC, CONTAINER).normalize());
    bag.visible = k > 0.55;
    bag.position.lerpVectors(CONTAINER, PC, clamp((k - 0.55) / 0.45, 0, 1) * 0.35);
    canopy.visible = risers.visible = slider.visible = false;
    segment(CONTAINER, PC);
    lines.geometry.setDrawRange(0, n);
    points.needsUpdate = true;
    return;
  }

  // Line stretch comes first, over the first fifth of the opening; then the canopy
  // spreads on its lines, nose down while it surges, and flies trimmed a little nose
  // down; a flare rocks it back.
  const stretch = look.phase === 'inflating' ? clamp(look.t / (INFLATE_TIME * 0.2), 0, 1) : 1;
  canopy.visible = risers.visible = true;
  canopy.position.set(0, LINE * (0.5 + 0.5 * stretch), 0);
  canopy.rotation.set(-0.1 - 0.25 * (1 - Math.min(1, look.open)) + 0.3 * look.flare, 0, 0);
  if (look.drape) {
    // Draping: carried on forward and down onto the ground ahead, tipping over onto
    // its nose on the way.
    const d = look.drape, e = d * d * (3 - 2 * d);
    canopy.position.set(0, LINE + (look.ground - LINE) * Math.min(1, e * 1.15), -DRAPE_AHEAD * Math.sin((e * Math.PI) / 2));
    canopy.rotation.set(-0.1 * (1 - e) - 0.9 * Math.sin(Math.PI * e), 0, 0); // flat once it is down
  }
  canopy.updateMatrix();
  reshape(wing, shapeOf(look, shape), rig.userData.lit);

  // The lines, from each rib to the links, the A and B lines to the front risers and
  // the C and D lines to the back ones. Loose, the links hang where the harness was.
  const harness = look.harness;
  for (let r = 0; r <= CELLS; r++) {
    const s = -1 + (2 * r) / CELLS, side = s < 0 ? -1 : 1;
    for (let row = 0; row < LINE_ROWS.length; row++) {
      canopyPoint(s, LINE_ROWS[row], 1, 0, shape, TIP).applyMatrix4(canopy.matrix);
      if (harness) LINK.copy(harness);
      else LINK.set(side * LINK_X, LINKS, row < 2 ? FRONT_LINK : BACK_LINK);
      segment(LINK, TIP);
    }
  }
  // The brake lines fan in from the tail to a cascade on each side, and one line runs
  // on from there to the toggle.
  for (const side of [-1, 1]) {
    JOIN.set(side * 0.42 * Math.min(1, shape.span), CASCADE * canopy.position.y, 0.34);
    // Let go of, the cascade is wherever the slack brake lines fall: halfway from the
    // harness to the middle of that side of the tail.
    if (harness) JOIN.lerpVectors(harness, canopyPoint(side * 0.55, 1, 1, 0, shape, TIP).applyMatrix4(canopy.matrix), 0.5);
    for (const at of BRAKE_AT) {
      canopyPoint(side * at, 1, 1, 0, shape, TIP).applyMatrix4(canopy.matrix);
      segment(JOIN, TIP);
    }
    if (harness) HAND.copy(harness);
    else if (side < 0 && hand) HAND.copy(hand);
    else HAND.copy(STOWED).setX(side * Math.abs(STOWED.x));
    segment(HAND, JOIN);
  }

  // The slider comes down the lines as the canopy spreads, but only a little way: it
  // stays up under the canopy, well above the head, and bunches up as it goes. Each
  // of its corners starts on a line group, between the middle of where that group's
  // lines are sewn to the canopy and the links.
  const down = clamp((look.open - 0.2) / 0.85, 0, 1);
  slider.visible = !harness;
  if (slider.visible) {
    const corners = slider.geometry.getAttribute('position');
    const order = [[-1, 0], [1, 0], [1, 2], [-1, 2]];
    order.forEach(([side, row], i) => {
      GROUPS[i].set(0, 0, 0);
      for (let r = 0; r <= CELLS; r++) {
        const s = -1 + (2 * r) / CELLS;
        if ((s < 0) !== (side < 0)) continue;
        GROUPS[i].add(canopyPoint(s, (LINE_ROWS[row] + LINE_ROWS[row + 1]) / 2, 1, 0, shape, TIP).applyMatrix4(canopy.matrix));
      }
      GROUPS[i].multiplyScalar(1 / ((CELLS + 1) / 2));
      LINK.set(side * LINK_X, LINKS, row < 2 ? FRONT_LINK : BACK_LINK);
      GROUPS[i].lerp(LINK, 0.05 + (SLIDER_DOWN - 0.05) * down);
    });
    JOIN.set(0, 0, 0);
    for (const corner of GROUPS) JOIN.addScaledVector(corner, 1 / GROUPS.length);
    GROUPS.forEach((corner, i) => {
      corner.lerp(JOIN, SLIDER_GATHER * down);
      corners.setXYZ(i, corner.x, corner.y, corner.z);
    });
    corners.needsUpdate = true;
    slider.geometry.computeBoundingSphere();
  }

  // The pilot chute rides behind and above the top of the canopy on its bridle, and
  // the bag it pulled off is on the bridle between them. Let go of, the canopy has
  // nothing to hold the pilot chute up and it lies collapsed on the fabric, out of
  // sight.
  pilot.visible = bag.visible = !harness;
  if (harness) {
    lines.geometry.setDrawRange(0, n);
    points.needsUpdate = true;
    lines.geometry.computeBoundingSphere();
    return;
  }
  canopyPoint(0, 0.42, 0, 0, shape, TOP).applyMatrix4(canopy.matrix);
  const bob = Math.sin(look.time * 3.1) * 0.03;
  PC.set(0.02 * Math.sin(look.time * 1.7), 0.32 + bob, 0.9).applyMatrix4(canopy.matrix);
  if (look.phase === 'inflating') PC.lerp(BACK.set(0, LINE * 0.5 + 0.7, 1.1), 1 - clamp(look.t / (INFLATE_TIME * 0.3), 0, 1));
  pilot.position.copy(PC);
  pilot.quaternion.setFromUnitVectors(ALONG_Y, JOIN.subVectors(PC, TOP).normalize());
  bag.position.lerpVectors(TOP, PC, 0.35);
  segment(TOP, PC);
  lines.geometry.setDrawRange(0, n);
  points.needsUpdate = true;
  lines.geometry.computeBoundingSphere();
}

// ------------------------------------------------------------------ left behind

// A canopy on the ground: how long it takes to come down over its nose and settle,
// and when, as a share of the time it lies there, it starts to fade.
const DRAPE_TIME = 1.6, FADE_FROM = 0.65;
// A canopy cut away: how long it is watched drifting off, how fast it settles into
// drifting - forward on what is left of its trim, and down - how quickly it turns
// and how far it empties.
const DRIFT_TIME = 6, DRIFT_TAU = 1.2, DRIFT_SPEED = 1.4, DRIFT_SINK = 1.1, DRIFT_SPIN = 0.35;
const DRIFTING = new THREE.Vector3();

/**
 * A canopy the walker has let go of: landed and lying in the street, or cut away and
 * drifting off over it. It is the same rig as the one over their head, rebuilt out of
 * unlit, bendable materials, because it is out in the scene now - where nothing is
 * lit and everything is bent around the planet - rather than in the pass that draws
 * what the walker holds.
 *
 * `from` is the rig's world matrix at the moment of letting go, `look` how the canopy
 * looked then (lookOf), `ground` the height of what is under it and `life` how long
 * it stays: a landed one fades as it is repacked, so it is gone when the pack is
 * ready again.
 *
 * Implements: REQ-TOOL-077, REQ-TOOL-078
 */
export class LooseCanopy {
  constructor(scene, kind, from, look, { ground = 0, velocity = null, life = DRIFT_TIME } = {}) {
    this.scene = scene;
    this.kind = kind;
    this.t = 0;
    this.life = life;
    this.materials = [];
    const basic = (color, options = {}) => {
      const { shininess, ...rest } = options; // what a lit material takes and an unlit one does not
      const material = scene.bendable(new THREE.MeshBasicMaterial({ color, transparent: true, depthWrite: true, ...rest }));
      this.materials.push({ material, opacity: rest.opacity ?? 1 });
      return material;
    };
    const lineMaterial = (color, options = {}) => {
      const material = scene.bendable(new THREE.LineBasicMaterial({ color, transparent: true, ...options }));
      this.materials.push({ material, opacity: 1 });
      return material;
    };
    this.rig = canopyRig((geometry, color, options) => new THREE.Mesh(geometry, basic(color, options)), lineMaterial, false);
    this.rig.getObjectByName('risers').visible = false;
    from.decompose(this.rig.position, this.rig.quaternion, new THREE.Vector3());
    this.from = this.rig.quaternion.clone();
    // Level, on the heading it had: what a canopy lying on the ground is turned to.
    const heading = new THREE.Euler().setFromQuaternion(this.from, 'YXZ').y;
    this.level = new THREE.Quaternion().setFromEuler(new THREE.Euler(0, heading, 0, 'YXZ'));
    this.ground = ground;
    this.velocity = velocity ? new THREE.Vector3(velocity.x, velocity.y, velocity.z) : new THREE.Vector3();
    this.drift = new THREE.Vector3(-Math.sin(heading) * DRIFT_SPEED, -DRIFT_SINK, -Math.cos(heading) * DRIFT_SPEED);
    this.look = { ...look, phase: 'flying', t: INFLATE_TIME, drape: 0, deflate: 0, harness: new THREE.Vector3() };
    this.time = look.time;
    scene.scene.add(this.rig);
    this.update(0);
  }

  /** One frame of it, in a wind of `wind` ({x, z}); returns false once it is gone and has been taken off the map. */
  update(deltaTime, wind = null) {
    this.t += deltaTime;
    const look = this.look, rig = this.rig;
    look.time = this.time + this.t;
    if (this.kind === 'landed') {
      // Down over its nose onto the ground ahead, and flat there: level first, then the
      // drape, with the lines lying back to where the walker came down.
      const d = Math.min(1, this.t / DRAPE_TIME);
      rig.quaternion.slerpQuaternions(this.from, this.level, Math.min(1, d * 3));
      look.drape = d;
      look.deflate = d;
      look.ground = this.ground - rig.position.y + 0.03;
      look.harness.set(0, look.ground, 0);
      look.brake = look.turn = look.flare = 0;
      look.speed *= Math.exp(-deltaTime * 3);
      // The lines lose their tension as it comes down, and end up lying on the ground.
      look.slack = 0.1 + 2 * d;
    } else {
      // Empty and flying itself: it slows into a drift, sinks, turns slowly away and
      // empties, trailing its lines and risers from where the walker was.
      // An empty canopy goes where the wind takes it.
      DRIFTING.copy(this.drift);
      if (wind) { DRIFTING.x += wind.x; DRIFTING.z += wind.z; }
      this.velocity.lerp(DRIFTING, 1 - Math.exp(-deltaTime / DRIFT_TAU));
      rig.position.addScaledVector(this.velocity, deltaTime);
      rig.rotateY(DRIFT_SPIN * deltaTime);
      look.deflate = Math.min(0.55, this.t / 5);
      look.open = 1 - 0.25 * look.deflate;
      look.brake = 0.1;
      look.turn = 0.3 * Math.sin(this.t * 0.7);
      look.flare = 0;
      look.speed = this.velocity.length();
      look.harness.set(0.05 * Math.sin(this.t * 2.3), 0.12, 0.18 + 0.05 * Math.sin(this.t * 1.9));
      // Nothing hangs from them any more: they belly out below the canopy.
      look.slack = 0.15 + 0.3 * look.deflate;
    }
    poseRig(rig, look);
    const fade = clamp((this.life - this.t) / (this.life * (1 - FADE_FROM)), 0, 1);
    for (const { material, opacity } of this.materials) material.opacity = opacity * fade;
    if (this.t < this.life) return true;
    this.dispose();
    return false;
  }

  /** Off the map, and its buffers freed. */
  dispose() {
    this.scene.scene.remove(this.rig);
    this.rig.traverse(o => o.geometry?.dispose());
    for (const { material } of this.materials) material.dispose();
    this.materials = [];
  }
}
