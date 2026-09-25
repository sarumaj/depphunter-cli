// Walk mode: explore the map on foot, in first person, on a small planet. The walker
// lives on the flat map (layout coordinates) - collisions, heights and darts are all
// computed there - and MapScene bends what is drawn around the walker's feet.
//
// The dependency hunt: whatever the primary tool in your hands is - a fishing rod, a
// net, a camera, a bubble wand, an extinguisher, a tracking dart (tools.js) - using it
// on a building tags the module: it is selected, so its dependency trails light up,
// and a beacon marks it for the rest of the session. Using it a second time on a
// tagged building opens its details. The hand and the tool are drawn in front of the
// camera and swing when used, so the gesture is visible rather than implied.
//
// The secondary tools do none of that. They are how the walker gets about: a line to
// a wall, a jet to fly on, floats to cross the bay with. One of each is carried at a
// time, one to a hand - the primary in the right and the secondary in the left - so a
// walker can fly over the map and net what they find without putting either down.
// Which secondary is in the off hand is what decides whether the walker flies or the
// water holds them up, so putting one away is how you come down, or get wet.
//
// The other quarry is real: every finding a scanner reported walks the streets as a
// bug (bugs.js), and catching one reads out what was said about it. They bite back,
// and a roof is a long way down, so the walker has a condition to keep (health.js).

import * as THREE from './vendor/three.module.min.js';
import { rampsFor, rampHeight, bridgesFor, bridgeHeight, bridgeBounds } from './city.js';
import { Health } from './health.js';
import { Wind } from './wind.js';
import { severityColors } from './findings.js';
import { PRIMARY_IDS, SECONDARY_IDS, DEFAULT_TOOL, toolFor, idleTool, restTool, studyTool, viewLights, hits, isMelee } from './tools.js';
import { ToolWheel, EMPTY, carriedRing, cycle, keyFor, keysFor, rowOrder, toolForKey } from './switcher.js';
import { inBlaze } from './flames.js';

// A building is one unit wide and its storeys 0.3 high (city.js): the walker is
// about a storey and a half tall.
const EYE = 0.45;           // eye height above the feet
// Implements: REQ-WALK-004
const WALK = 2.6, RUN = 7, FLY = 10; // units per second
// A jump clears a curb and a terrace wall and nothing more. At this gravity it tops
// out about 0.4 units up, which against a storey of 0.3 is a person leaving the ground
// rather than one clearing a tree.
// Implements: REQ-WALK-005
const JUMP = 3.2, GRAVITY = 13;
// Half a storey (0.3): a curb, a ramp's slope and a bridge's arch are walked, a
// terrace wall (0.28 in layout.js) is not - that takes the ramp or a jump.
// Implements: REQ-WALK-006
const STEP = 0.15;          // highest ledge walked up without jumping
const BODY = 0.12;          // walker radius for collisions
const WATER = -0.45;        // the water surface (layout LAND_H below the mainland)
const REACH = 90;           // aiming distance
const CELL = 2;             // spatial grid for box lookups
// A grid cell as one number rather than "gx,gz". These lookups happen several hundred
// times a frame - the crosshair marches a ray through them, and every step the walker
// takes probes five points - and a key built by concatenation is that many strings a
// frame for the collector to take away again. Packing two cell indexes into one
// integer costs an add and a multiply. Cells beyond 32768 from the origin - 65 536
// map units, which no layout comes within orders of magnitude of - would share a key
// with another cell; even then the callers' own footprint tests reject the stranger,
// so the cost would be a comparison rather than a wrong answer.
const cellOf = (gx, gz) => (gx + 32768) * 65536 + (gz + 32768);
const cellKey = (x, z) => cellOf(Math.floor(x / CELL), Math.floor(z / CELL));
// Handed back where a cell holds nothing, so that a miss allocates as little as a hit.
const NO_CELL = [];
// A prop only stops a walker standing at its own level: a tree on the terrace above
// is not in the way, and one on the shore below is not either.
const PROP_REACH = 0.7;
const LOOK = 0.0022;        // radians per pixel of mouse movement
const TURN = 2.2;           // radians per second with the arrow keys
const MIN_R = 6, MAX_R = 2000;
// Implements: REQ-WALK-011
const MAX_LOOK_STEP = 250;  // pixels; larger pointer movements are glitches, not looks
// How near the crosshair ray a bug counts as aimed at. It is generous, and grows with
// distance: the ray is sampled ever more coarsely the farther it goes, a bug fifty
// units away is two pixels wide, and the building behind it is one more click away in
// any case.
const BUG_AIM = t => 0.3 + t * 0.012;
// What a tool throws when it says nothing about how: a middling lob under ordinary
// gravity. Every tool that throws does say (tools.js); this is here so that adding
// one cannot make it fall through the floor of the world instead.
const DEFAULT_FLIGHT = { speed: 24, arc: 1, gravity: 6, drag: 0 };
// A line that has stuck: the longest it may pull before letting go (so a hook on
// something that moved cannot strand anyone), how far in from a roof's edge it sets
// the walker down, and how close a thing has to be before pulling to it is nothing.
// How fast and how near is the tool's business (tools.js).
const GRAPPLE_TIME = 5, ROOF_IN = 0.5, NO_PULL = 1.2;
// The bug tracker: how far around the walker it sweeps, and how often it is redrawn.
// A dozen times a second is plenty for something that turns as slowly as a walker.
// The range follows the hunt - a four-file repository and a thousand-file one are both
// worth seeing whole - within these bounds, and eases rather than jumping.
// Implements: REQ-HUNT-025
const RADAR_MIN = 12, RADAR_MAX = 400, RADAR_MS = 85, RADAR_SIZE = 150;
// Closing in: inside this, the sweep stops trying to hold the whole map and draws the
// neighborhood instead, growing by up to RADAR_GROW as it does. The last few steps
// to a bug are the ones worth seeing, and they are the ones a whole-map sweep loses.
// 14 rather than something roomier because a city block is a few units across: any
// wider and a map with bugs on every street is zoomed in the whole time, which is
// the same as not zooming at all.
const RADAR_NEAR = 14, RADAR_GROW = 0.55;
// How long a fire takes to pulse on the sweep, in milliseconds. Slow enough to read
// as breathing rather than blinking: a blink is an alarm and there is nothing sudden
// about a fire, which is already alight and will still be alight in a second.
const RADAR_PULSE = 1500;
// What a fire is drawn in on the sweep. Not a severity color: severity is what the
// scanners said, and every fire here is the same thing whatever they rated it - a
// vulnerability somebody can reach. It is the color of the flames instead, so the
// dot and the thing it stands for are plainly the same.
const FIRE_DOT = '#f26a1b';
// How long the range and the dial take to settle, in seconds. Eased by elapsed time
// rather than per redraw, so the growth is the same on a slow frame as on a fast one.
const RADAR_RANGE_TAU = 0.5, RADAR_ZOOM_TAU = 0.55;
// Degrees: the default view, the wheel's zoom range, and the view through the scope
// (right button).
// Implements: REQ-WALK-012, REQ-WALK-013
const FOV = 70, MIN_FOV = 30, MAX_FOV = 90, SCOPE_FOV = 22;
// How far in the held tool sits, as a fraction of where it is modelled. See showTool.
const VIEW_NEAR = 0.5;
const SWING = 0.45;         // seconds a tool takes to swing and settle
// How far the view rides up and down, and how fast, for each of the two things that
// carry the walker: the long slow heave of a jet holding them up, and the swell of
// water under a pair of floats. Feet on solid ground do not ride at all - a bob on
// every footfall is what makes people put a first-person view down. It is the view
// alone: nothing about where the walker is or what they can reach moves with it.
// Implements: REQ-WALK-036
const RIDE = {
  fly: { lift: 0.055, rate: 1.5 },
  float: { lift: 0.045, rate: 2.1 },
};
// The shore stands half a unit above the water, which is further than a step. WADE is
// what is added to a step to climb out of the bay - without it, anything down there is
// down there for good; with it, a step and a wade (0.57) clear the shore's 0.45 and
// nothing much higher, so it is the water that sets how far that is, not STEP - and
// WADE_IN is how far below the feet the water may be to be
// walked into rather than jumped into.
// Implements: REQ-WALK-040, REQ-WALK-041
const WADE = 0.42, WADE_IN = 0.6;
// How far the walker may leave the map: over the water beyond the outermost shore,
// and above its tallest building when flying.
// Implements: REQ-WALK-007, REQ-WALK-008
const SHORE_MARGIN = 3, SKY_MARGIN = 12;
// A photograph held up to look at: how long it stays up altogether, and how long the
// camera takes to come all the way to the face and to go back down again. The travel
// is most of the way from the hip to the eye, so it is given longer than a gesture.
const SHOWING = 4.2, LIFTING = 0.6;
// A burst on the jet backpack: how long it lasts and how much faster it goes.
// Implements: REQ-TOOL-024
const BURST = 0.9, BURST_SPEED = 3;
// Out of your depth: how fast the water takes a walker who is in it with nothing to
// hold them up. A couple of seconds, so wading ashore is possible and standing in the
// bay when the skimmers go away is not.
// Implements: REQ-WALK-031
const DROWN = 45;
// How long the screen stays red after the walk ends, before the map comes back.
// Implements: REQ-WALK-033
const DYING = 1.1;
// Being bitten: how near a bug has to be to reach the walker, and how often it can.
// The reach is a stride, so standing in the middle of a lap is what does it rather
// than walking past one; a bug on a wall three storeys up cannot reach anybody.
// Implements: REQ-WALK-028
const BITE_REACH = 0.75, BITE_EVERY = 1.1;
// Standing in a fire: how often it takes something, and what a full blaze takes each
// time. Less than a bite from anything serious, and it lands over and over, which is
// the difference between the two dangers. A bug bites and you turn and deal with it;
// a fire does not bite, it is simply somewhere you cannot be - so what it costs is a
// reason to get off the roof rather than a reason to fight it where you stand. Six
// seconds in a blaze at full heat is most of a walker, and walking through the edge
// of one costs a few points and a fright.
const BURN_EVERY = 0.75, BURN = 13;
// How far a dart looks for a wall to steer towards, and how nearly ahead of itself it
// will accept one, as a cosine: about forty degrees either side, which is wide enough
// to save a lobbed shot and narrow enough that a dart cannot turn round.
const TRACK_REACH = 30, TRACK_AHEAD = 0.75;

// Keys the walker owns while active, by KeyboardEvent.code; the map's own shortcuts
// for these letters are suspended. A key that is not here never reaches walk mode -
// the map keeps it - so this has to list every one the handler below acts on. Q and E
// rotate the map's view and expand things, which would only reshuffle the city around
// a walker; out in it they change hands instead (switcher.js).
// Implements: REQ-WALK-023
const KEYS = new Set([
  'KeyW', 'KeyA', 'KeyS', 'KeyD', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight',
  'Space', 'ShiftLeft', 'ShiftRight', 'KeyC', 'KeyE', 'KeyQ', 'KeyF', 'Enter',
  'Escape', 'KeyV', 'KeyM', 'KeyR', 'KeyH',
  // Every tool's digit: the row is ten slots and the digits count along it, 1 to 9
  // and then 0 for the tenth (switcher.js).
  'Digit0', ...Array.from({ length: 9 }, (_, i) => `Digit${i + 1}`),
]);

// Planet curvature, by the character typed rather than the key's place on the board:
// on a German keyboard the key at BracketRight types '+', which would otherwise make
// '+' curve the planet while '-' still changed the map's depth.
// Implements: REQ-WALK-003
const BIGGER = new Set(['+', '=', ']']), SMALLER = new Set(['-', '_', '[']);

export class Walker {
  /**
   * hooks: {
   *   onAim(i, x, y)   box index under the crosshair (-1: none) and its client position
   *   onHit(box, n)    a dart tagged a box; n: modules tagged so far
   *   onInspect(box)   show the box's details: a dart in a tagged building, or
   *                    Enter (null: nothing aimed at)
   *   onExit()         the walker left walk mode (V, Esc, or a fall they did not
   *                    survive)
   *   caught()         how many findings are in the backpack, which is what the
   *                    walker's health is built on
   *   onPhoto(where)   keep the view as a photograph, captioned with whatever was in
   *                    the frame; what every use of the camera asks for, except a
   *                    second one on a module already tagged, which reads it instead
   *   onResume()       the walker is walking again, so whatever was being read - the
   *                    details, the backpack, a menu - can be put away
   *   busy()           whether anything on screen wants the mouse. The walker never
   *                    takes the pointer back while it does
   *   severityOf(node) the worst thing the scanners said about a module, for its
   *                    beacon and its ring on the tracker
   *   onRender()       after every frame (labels)
   * }
   */
  constructor(scene, hud, hooks) {
    this.scene = scene;
    this.hud = hud;
    this.hooks = hooks;
    this.active = false;
    this.boxes = [];
    this.grid = new Map();
    this.keys = new Set();
    this.darts = [];
    this.tagged = new Set(); // node ids
    this.beacons = new THREE.Group();
    this.ramps = [];
    this.bridges = [];
    this.decks = new Map(); // ramps, by grid cell (indexDecks)
    this.spans = new Map(); // bridge decks, likewise, but tested differently (height)
    this.aim = { i: -1, point: null, bug: null, box: null };
    this.bugs = null; // set by setBugs once there are findings to walk the streets
    // What is alight and what that looks like (fires.js, flames.js), set by setFires.
    // Fire burns on the map as well as in the street, so the walker does not own it -
    // it only puts it out.
    this.fires = null;
    // Frozen: the details panel has the pointer, so the view holds still. Otherwise
    // the freed cursor and the reticle in the centre both steer the same scene, and
    // reading about a building means fighting it.
    this.frozen = false;
    // Set once the pointer lock has been asked for and refused for good. Somewhere
    // that never grants it, asking again on every click is a click that never does
    // anything else, so from then on a click is the tool and the view is turned by
    // dragging. lockFails counts refusals since the last lock, to tell that apart
    // from a browser that refuses one request and grants the next.
    this.noLock = false;
    this.lockFails = 0;
    // One tool to a hand: the hunt in the right, whatever carries the walker in the
    // left. There is always a primary; the off hand may be empty.
    this.primary = toolFor(hooks.tool?.() || DEFAULT_TOOL);
    this.secondary = null;
    // The wheel, which is the third way of changing hands and the only one that shows
    // all ten at once. It builds its own face the first time it comes up, and while it
    // is up the mouse points at it rather than looking around.
    this.wheel = new ToolWheel(hud.querySelector('.w-wheel'));
    this.health = new Health(hud);
    this.bitAt = 0;   // when a bug last got a bite in
    this.burnAt = 0;  // ... and when a fire last took something
    this.burst = 0;   // seconds of jet backpack thrust left
    this.fell = null; // the height a fall in progress started from
    // How full each carried tool's tank is, as a share, by tool id. A tank is the
    // tool's rather than the walker's, so putting the jet down and picking it up again
    // does not refill it - but time on the ground does.
    this.tanks = new Map();
    // What has run out. A tool that empties in use is dead until it is taken out
    // again, so an all-but-empty tank cannot flicker the walker in and out of the air
    // a frame at a time; it fills in the meantime like everything else.
    this.dry = new Set();
    this.fuelShown = -1; // what the gauge currently says, so it is only written when it moves
    // What running and jumping are paid for out of.
    this.wind = new Wind(hud);
    this.blown = false; // ... and whether the walker has been told they are out of it
    this.rideLift = 0;   // how far the view is currently riding, eased rather than switched
    this.ridePhase = 0;  // ... and where in the swell it is
    this.showing = null; // a photograph being looked at on the back of the camera
    this.dying = null;    // seconds into the red, null while the walker is alive
    this.sinking = false; // ... and whether they are in the water with nothing to float on
    this.viewmodel = null;      // the right hand and its tool
    this.offhand = null;        // ... and the left, when something is carried in it
    this.held = null;           // what holds them in front of the walk camera
    this.props = null;          // the prop obstacle list this grid was built from
    this.propGrid = new Map();  // cell -> the props standing in it
    this.firing = false;        // the trigger is held; a tool with a cadence keeps going
    this.firedAt = 0;           // ... and when it last went off
    this.swing = -1;            // seconds into the right hand's gesture, -1 when idle
    this.offSwing = -1;         // ... and the left hand's
    this.pace = 0;              // how hard the walker is moving, for the tool's sway
    this.p = { x: 0, z: 0, feet: 0, vy: 0, yaw: 0, pitch: 0, ground: true, fly: false };
    this.handsOff = false; // H: the view with nothing held in it
    this.home = null;      // where the walker stood when they last left the street
    this.radius = 40;
    this.fov = FOV;
    this.bindInput();
  }

  /** Keys that belong to walk mode while it is active. */
  owns(e) {
    return this.active && (KEYS.has(e.code) || BIGGER.has(e.key) || SMALLER.has(e.key));
  }

  /** Hands the walker the fires burning on it (fires.js); null takes them away. */
  setFires(fires) { this.fires = fires; }

  /** Hands the walker the bugs patrolling the map (bugs.js); null takes them away. */
  setBugs(bugs) {
    this.bugs = bugs;
    bugs?.show(this.active);
    if (this.active) this.drawHud();
  }

  // Implements: REQ-PERF-005, REQ-PERF-008
  setBoxes(boxes) {
    this.boxes = boxes;
    this.grid.clear();
    const lim = { minX: Infinity, maxX: -Infinity, minZ: Infinity, maxZ: -Infinity, maxY: 0 };
    for (const b of boxes) {
      lim.minX = Math.min(lim.minX, b.x - b.w / 2); lim.maxX = Math.max(lim.maxX, b.x + b.w / 2);
      lim.minZ = Math.min(lim.minZ, b.z - b.d / 2); lim.maxZ = Math.max(lim.maxZ, b.z + b.d / 2);
      lim.maxY = Math.max(lim.maxY, b.y + b.h);
    }
    this.limits = boxes.length ? lim : null;
    this.ramps = rampsFor(boxes);
    this.bridges = bridgesFor(boxes);
    this.indexDecks();
    // Darts in flight aim at boxes of the old layout: let them go.
    for (const dart of this.darts) this.scene.scene.remove(dart.mesh);
    this.darts = [];
    this.cutLine();
    for (const b of boxes) {
      const x0 = Math.floor((b.x - b.w / 2) / CELL), x1 = Math.floor((b.x + b.w / 2) / CELL);
      const z0 = Math.floor((b.z - b.d / 2) / CELL), z1 = Math.floor((b.z + b.d / 2) / CELL);
      for (let x = x0; x <= x1; x++) for (let z = z0; z <= z1; z++) {
        const cell = this.grid.get(cellOf(x, z));
        if (cell) cell.push(b); else this.grid.set(cellOf(x, z), [b]);
      }
    }
    this.drawBeacons();
    // A relayout can put a building where the walker stands, or shrink the map away
    // from under them.
    if (this.active) {
      this.confine();
      this.p.feet = Math.max(this.p.feet, this.height(this.p.x, this.p.z));
      this.setRadius(this.radius);
    }
  }

  // Ramps and bridge decks into the same spatial grid as the boxes: height() is called
  // several times a frame, at five points each, and a large map has a ramp per block.
  //
  // They are kept apart because height() asks them different questions. A ramp is
  // narrower than the walker and is asked at their footprint, the way a box is. A
  // bridge is asked once, at their middle, with the deck already narrowed by their
  // own radius - which is what keeps a body from hanging over the railing.
  // Implements: REQ-PERF-008
  indexDecks() {
    this.decks = new Map();
    this.spans = new Map();
    const put = (map, r, at) => {
      for (let x = Math.floor(r.x0 / CELL); x <= Math.floor(r.x1 / CELL); x++) {
        for (let z = Math.floor(r.z0 / CELL); z <= Math.floor(r.z1 / CELL); z++) {
          const cell = map.get(cellOf(x, z));
          if (cell) cell.push(at); else map.set(cellOf(x, z), [at]);
        }
      }
    };
    for (const r of this.ramps) put(this.decks, r, (x, z) => rampHeight(r, x, z));
    for (const b of this.bridges) put(this.spans, bridgeBounds(b), (x, z) => bridgeHeight(b, x, z, BODY));
  }

  /**
   * Where the walker is, for the map to draw them standing there (avatar.js), or null
   * before they have ever been out. The anchor is the block under their feet, so the
   * spot can be found again after the city has been rebuilt around it.
   */
  stance() {
    if (!this.active) return this.home;
    const { x, z, feet, yaw, pitch } = this.p;
    return { x, z, feet, yaw, pitch, anchor: this.anchorFor() };
  }

  /**
   * Starts walking, at full health: in front of `box` when the map has somewhere in
   * mind - a building just selected - and where the walker left off when it has not,
   * or on the south road of `block` if they have never been out. `bounds` sizes the
   * planet, and `grab` is false when something else wants the pointer first.
   *
   * Implements: REQ-WALK-001, REQ-WALK-009, REQ-WALK-010, REQ-WALK-030
   */
  enter(box, block, bounds, grab = true) {
    const diag = Math.hypot(bounds.maxX - bounds.minX, bounds.maxZ - bounds.minZ);
    this.radius = clamp(diag * 0.6, 15, Math.min(600, this.maxRadius()));
    this.active = true;
    this.hud.hidden = false;
    this.scene.setWalking(true, this.radius);
    this.scene.scene.add(this.beacons);
    this.bugs?.show(true);
    this.showTool();
    this.setFog();
    this.p.fly = this.flying();
    if (box) {
      this.teleport(box);
      this.p.vy = 0;
      this.p.feet = this.height(this.p.x, this.p.z);
    } else if (this.home) {
      this.resume(this.home);
    } else {
      Object.assign(this.p, { x: block.x, z: block.z + block.d / 2 - 0.15, yaw: 0, pitch: -0.15, vy: 0 });
      this.p.feet = this.height(this.p.x, this.p.z);
    }
    this.last = performance.now();
    this.scoped = false;
    this.fov = FOV;
    // Full health on the way in, which is also the way back after dying.
    this.health.reset(this.hooks.caught?.() ?? 0);
    this.bitAt = 0;
    this.burnAt = 0;
    this.burst = 0;
    this.fell = null;
    this.dying = null;
    this.sinking = false;
    this.tanks.clear();
    this.dry.clear();
    this.wind.reset();
    this.blown = false;
    this.rideLift = 0;
    this.ridePhase = 0;
    this.hud.classList.remove('dead');
    this.hud.style.removeProperty('--dead');
    // The tracker starts where it belongs rather than easing in from wherever it was
    // left the last time walk mode was entered, possibly half a map away.
    this.radarRange = this.radarZoom = undefined;
    this.radarAt = 0;
    this.drawSlots();
    this.drawHud();
    this.loop();
    // Like a first-person shooter: the pointer is captured at the reticle at once (V
    // and the Walk button are user gestures, which browsers require for this). A first
    // walk is being explained instead, and asks for the pointer when it is done.
    if (grab) this.lockPointer();
  }

  // Implements: REQ-WALK-001
  exit() {
    if (!this.active) return;
    // Where they stood, so coming back is coming back rather than starting again.
    this.home = this.stance();
    this.active = false;
    this.keys.clear();
    this.firing = false;
    this.closeWheel(false);
    for (const dart of this.darts) this.scene.scene.remove(dart.mesh);
    this.darts = [];
    this.cutLine();
    this.endShow(false);
    this.scene.scene.remove(this.beacons);
    this.bugs?.show(false);
    this.showTarget(null);
    this.hideTool();
    if (document.pointerLockElement) document.exitPointerLock();
    cancelAnimationFrame(this.frame);
    this.hud.hidden = true;
    this.hud.classList.remove('compact');
    this.hud.classList.remove('hit');
    this.hud.classList.remove('dead');
    this.hud.classList.remove('held');
    this.hud.style.removeProperty('--dead');
    this.dying = null;
    this.hud.querySelector('.w-flash').textContent = '';
    clearTimeout(this.flashTimer);
    this.setScoped(false);
    this.fov = FOV;
    this.scene.walkCamera.fov = FOV;
    this.scene.walkCamera.updateProjectionMatrix();
    clearTimeout(this.hudTimer);
    this.hudTimer = 0;
    this.scene.setWalking(false);
    this.hooks.onAim(-1);
    this.hooks.onExit();
  }

  /**
   * Back to a remembered stance. The city may have been rebuilt while the walker was
   * away - a depth change, a filter, a live update - so the block they were standing
   * on is looked for by name and they are put back against it; failing that they land
   * where they were and step aside from whatever now stands there.
   */
  resume(home) {
    Object.assign(this.p, { x: home.x, z: home.z, feet: home.feet, yaw: home.yaw, pitch: home.pitch, vy: 0 });
    const now = home.anchor && this.boxes?.find(b => b.node?.id === home.anchor.node.id);
    if (now) {
      this.reanchor(home.anchor, now);
      return;
    }
    this.confine();
    this.makeRoom(this.p.feet);
    const floor = this.height(this.p.x, this.p.z);
    this.p.feet = this.p.fly ? Math.max(this.p.feet, floor) : floor;
  }

  /**
   * Stands the walker where `box` can be seen: on a block (terrace), at its south
   * edge looking across it; beside anything else, on its lowest side that is not
   * water, looking at it.
   *
   * Implements: REQ-WALK-019
   */
  teleport(box) {
    const p = this.p, gap = 1.0;
    if (box.kind === 'terrace') {
      const z = box.z + box.d / 2 - 0.25;
      Object.assign(p, { x: box.x, z, feet: this.height(box.x, z), vy: 0, yaw: 0, pitch: -0.15 });
      this.confine();
      this.makeRoom(p.feet);
      p.feet = this.height(p.x, p.z);
      return;
    }
    const spots = [[0, box.d / 2 + gap], [0, -box.d / 2 - gap], [box.w / 2 + gap, 0], [-box.w / 2 - gap, 0]]
      .map(([dx, dz]) => ({ x: box.x + dx, z: box.z + dz }))
      .map(s => ({ ...s, h: this.height(s.x, s.z) }))
      .sort((a, b) => (a.h <= WATER) - (b.h <= WATER) || a.h - b.h);
    const s = spots[0];
    Object.assign(p, { x: s.x, z: s.z, feet: s.h, vy: 0 });
    this.confine();
    this.makeRoom(s.h);
    p.yaw = Math.atan2(-(box.x - s.x), -(box.z - s.z));
    const dist = Math.hypot(box.x - s.x, box.z - s.z);
    p.pitch = clamp(Math.atan2(box.y + box.h / 2 - (s.h + EYE), dist), -0.6, 0.9);
  }

  // ------------------------------------------------------------------ the tool

  /** Put the hand and its tool in front of the camera. */
  /**
   * Whether anything is held. An empty view is worth having - a hand and a rod take
   * up the lower right of the screen, and a screenshot or a look straight down at a
   * street wants neither - so H puts the tool away and takes it out again. It is only
   * the drawing: the tool still works, and a shot with nothing in hand leaves from
   * the walker's eye, which is where it left from before any of them had a muzzle.
   *
   * Implements: REQ-TOOL-032
   */
  setHandsOff(off) {
    this.handsOff = off;
    if (!this.active) return;
    if (off) this.hideTool(); else this.showTool();
    const back = this.secondary ? `${this.primary.label} and ${this.secondary.label.toLowerCase()}` : this.primary.label;
    this.flash(off ? 'Empty-handed - H takes them out again' : `${back} back in hand`);
    this.drawHud();
  }

  // Implements: REQ-TOOL-001, REQ-TOOL-006, REQ-TOOL-028
  showTool() {
    this.hideTool();
    // The reticle is the tool's, not the tool's name: several tools share one, and
    // style.css keys the crosshair off it. It is the primary's, because the crosshair
    // is what the hunt is aimed with; what the off hand carries is not aimed at all.
    this.hud.dataset.tool = this.primary.reticle || 'scope';
    if (this.handsOff) return;
    // A camera draws its children only when it is itself part of a scene, and this
    // one belongs to the pass that draws the tool over the world (MapScene.renderNow).
    this.scene.viewScene.add(this.scene.walkCamera);
    // The only lights in the scene, and they travel with the view: everything on the
    // map is drawn with unlit materials, so they reach nothing but what is held.
    this.lights ||= viewLights();
    this.scene.walkCamera.add(this.lights);
    // Held near the lens rather than out in the street. A viewmodel is drawn in the
    // same pass as the map, so at arm's length it is half a metre off the ground and
    // the pavement is drawn straight through it; brought in and scaled down by the
    // same amount, the picture is identical and nothing can reach it.
    this.held = new THREE.Group();
    this.held.scale.setScalar(VIEW_NEAR);
    this.viewmodel = this.take(this.primary);
    if (this.secondary) this.offhand = this.take(this.secondary, true);
    this.scene.walkCamera.add(this.held);
  }

  /**
   * Builds a tool's viewmodel and hangs it in front of the camera. `left` puts it in
   * the other hand, which is the same viewmodel turned inside out: one negative scale
   * across the frame mirrors the placement, the grip and the hand all at once, so
   * nothing in tools.js has to know which hand it is in. A mirror reverses the winding
   * of every triangle in it, so what it holds is drawn with both faces - which is also
   * what keeps the lighting on the mirrored hand the right way round.
   *
   * Implements: REQ-TOOL-028
   */
  take(tool, left = false) {
    const vm = tool.viewmodel();
    vm.userData.restY = vm.position.y;
    if (!left) {
      this.held.add(vm);
      return vm;
    }
    const hand = new THREE.Group();
    hand.scale.x = -1;
    hand.add(vm);
    vm.traverse(o => { if (o.material) o.material.side = THREE.DoubleSide; });
    this.held.add(hand);
    return vm;
  }

  hideTool() {
    if (this.lights) this.scene.walkCamera.remove(this.lights);
    if (this.held) this.scene.walkCamera.remove(this.held);
    this.held = null;
    this.viewmodel = null;
    this.offhand = null;
    this.scene.viewScene.remove(this.scene.walkCamera);
  }

  /**
   * Take another tool out. Which hand it goes in is the tool's own business: a primary
   * one replaces what the hunt is being done with, and a secondary one is picked up in
   * the off hand - or put down again, if it is already the one being carried, which is
   * how you come down out of the air or step off the water on purpose.
   *
   * Implements: REQ-TOOL-023, REQ-TOOL-028, REQ-TOOL-033, REQ-TOOL-050, REQ-HUNT-045
   */
  setTool(id) {
    // Reaching for another tool is done with the photograph: it comes down, and what
    // was in hand before it went up is not put back, because the walker has just said
    // what they want in their hand.
    if (this.showing) this.endShow(false);
    const tool = toolFor(id);
    if (tool.kind === 'secondary') {
      this.secondary = this.secondary === tool ? null : tool;
      this.offSwing = -1;
      // Taking one out is what brings it back after it has run dry - with whatever has
      // filled in the meantime, which may be little enough to run out again at once.
      if (this.secondary) this.dry.delete(tool.id);
    } else {
      this.primary = tool;
      this.swing = -1;
    }
    // Flight is a thing you are carrying, not a mode you are in: putting the jet
    // backpack away is how you come down, and so is running its tank dry.
    this.p.fly = this.flying();
    if (!this.p.fly) this.burst = 0;
    if (this.active) {
      this.setFog();
      this.showTool();
      this.drawSlots();
      this.drawHud();
      this.flash(tool.kind === 'secondary' && this.secondary !== tool
        ? `${tool.label} stowed`
        : tool.hint);
      this.aim = { i: -1, point: null, bug: null, box: null, far: false }; // it may want another target
    }
    this.hooks.onTool?.(this.primary.id);
  }

  /**
   * The row of tools along the bottom, built once: a slot per tool, carrying the
   * number that picks it. Only the one in hand is named, so the row stays a row of
   * shapes rather than a sentence.
   *
   * It is laid out as the walker is: what the left hand carries on the left, what the
   * right hand hunts with on the right, each group behind a small hand of its own. A
   * word would have said which is which as well, and worse - the row is read at a
   * glance in the middle of something else, and a hand is the one thing nobody has to
   * stop and parse.
   *
   * Implements: REQ-TOOL-053, REQ-TOOL-054, REQ-TOOL-055
   */
  drawSlots() {
    const row = this.hud.querySelector('.w-slots');
    // The same list the digits count along, so laying the row out is what numbers it.
    const order = rowOrder();
    if (row.querySelectorAll('.w-slot').length !== order.length) {
      const slots = [];
      order.forEach((id, n) => {
        const tool = toolFor(id);
        const hand = tool.kind === 'secondary' ? 'left' : 'right';
        if (!n || toolFor(order[n - 1]).kind !== tool.kind) {
          const mark = document.createElement('span');
          mark.className = 'w-hand';
          mark.dataset.hand = hand;
          mark.setAttribute('aria-hidden', 'true'); // a listbox's children are its options
          mark.title = hand === 'left'
            ? 'Your left hand: what carries you'
            : 'Your right hand: what the hunt is done with';
          slots.push(mark);
        }
        const el = document.createElement('div');
        el.className = 'w-slot';
        el.dataset.tool = id;
        el.dataset.kind = tool.kind;
        el.setAttribute('role', 'option');
        el.title = `${tool.label} (${hand} hand, ${keysFor(id).join(' or ')}) - ${tool.hint}`;
        el.append(
          // Its place in the row, which is its key: the row reads 1 to 0 from left to
          // right because the digits are counted along the order it is drawn in.
          Object.assign(document.createElement('kbd'), { textContent: keyFor(id) }),
          document.createElement('i'),
          Object.assign(document.createElement('span'), { className: 'w-slot-name', textContent: tool.label }),
        );
        slots.push(el);
      });
      row.replaceChildren(...slots);
    }
    // Two of them are in hand at once now, so two of them are lit.
    for (const el of row.querySelectorAll('.w-slot')) {
      const out = el.dataset.tool === this.primary.id || el.dataset.tool === this.secondary?.id;
      el.setAttribute('aria-selected', String(out));
    }
  }

  // ------------------------------------------------------------------ changing hands

  /**
   * E: the next tool for the hunting hand. The ring has no empty place in it - there
   * is always something to hunt with - so this only ever swaps one for another.
   *
   * Implements: REQ-TOOL-057
   */
  nextPrimary(dir = 1) {
    this.setTool(cycle(PRIMARY_IDS, this.primary.id, dir));
  }

  /**
   * Q: the next tool for the off hand, and after the last of them, nothing. An empty
   * hand belongs in that ring rather than outside it: putting the jet backpack away
   * is how a walker comes down and stepping off the skimmers is how they go in the
   * water, so "nothing" is a thing to reach for and not just what is left when you
   * stop reaching.
   *
   * Which does mean Q can drop a walker out of the sky, exactly as pressing the jet's
   * own key twice always could. It takes four presses to come back round to it, and
   * the flash says which one you are on, so it is a decision rather than a slip.
   *
   * Implements: REQ-TOOL-056
   */
  nextCarried(dir = 1) {
    const next = cycle(carriedRing(), this.secondary?.id ?? EMPTY, dir);
    if (next !== EMPTY) this.setTool(next);
    else if (this.secondary) this.setTool(this.secondary.id); // same tool again: stowed
    else this.flash('Left hand empty');
  }

  /**
   * R: the wheel. Flicked, it is one gesture - hold R, throw the mouse at a wedge, let
   * go, and what it landed on is in hand. Tapped, or let go without having pointed
   * anywhere, it stays up to be read, and R again, Enter or a click takes whatever is
   * under the cursor; Esc or the right button leaves it and changes nothing.
   *
   * The walker is held where they stand while it is up (`still`), and the city
   * behind it is blurred. Not because a menu wants a pause for its own sake, but
   * because the wheel covers the view: a walker who cannot see the street should not
   * be walking off a roof behind it, and stopping to change hands should not burn a
   * tank, drown anybody or hand the bug at your ankle a free bite. The bugs go on
   * walking their laps, since nothing they do can reach a held walker anyway.
   *
   * So a wheel opened by accident costs nothing at all: the cursor starts in the
   * middle where it points at nothing, and the world waits.
   *
   * Implements: REQ-TOOL-058, REQ-TOOL-062
   */
  openWheel() {
    if (this.wheel.open) { this.closeWheel(true); return; }
    this.setScoped(false);
    this.firing = false;
    this.wheel.raise(this.holding());
  }

  /** What is in hand, for the wheel to mark and name. */
  holding() {
    return { primary: this.primary.id, secondary: this.secondary?.id ?? null, dry: this.dry };
  }

  /**
   * R let go. What decides whether that was the whole gesture is the mouse rather than
   * the clock: let go pointing at something and the flick is finished, so it is taken;
   * let go pointing at nothing - the cursor never left the hub - and the walker was
   * asking to look rather than to choose, so the wheel stays up to be read.
   *
   * Which is deliberately not a hold-versus-tap threshold. A threshold has to be
   * measured against a clock, and this page can drop a frame or several while a city
   * is drawn behind the wheel; a tap that took one of those would have shut the wheel
   * in the walker's face for no reason they could see. Nothing here is timed.
   *
   * Implements: REQ-TOOL-059, REQ-TOOL-060
   */
  releaseWheel() {
    if (this.wheel.pick) this.closeWheel(true);
  }

  /**
   * Put the wheel away, taking what it is pointing at (take) or leaving it alone.
   *
   * Pointing at what is already in that hand keeps it, where the tool's own key would
   * have put a carried one down. The wheel has a wedge for an empty hand and the row
   * has not, so the key has to serve as both and the wheel does not: a walker who
   * meant to put the jet backpack down aims at the bare hand, and one who let the
   * cursor drift back onto the tool they are flying with does not fall out of the sky
   * for it.
   *
   * Implements: REQ-TOOL-059, REQ-TOOL-061, REQ-TOOL-064
   */
  closeWheel(take) {
    if (!this.wheel.open) return;
    const pick = take ? this.wheel.pick : null;
    this.wheel.close();
    if (!pick) return;
    if (pick === EMPTY) {
      if (this.secondary) this.setTool(this.secondary.id); // the same tool again: stowed
      else this.flash('Left hand empty');
    } else if (pick === this.primary.id || pick === this.secondary?.id) {
      this.flash(toolFor(pick).hint); // already in hand, so it only says what it is for
    } else this.setTool(pick);
  }

  /** The mouse, while the wheel has it. */
  aimWheel(dx, dy) {
    this.wheel.move(dx, dy, this.holding());
  }

  /**
   * A key pressed while the wheel is up. Returns whether the wheel took it, because
   * everything it takes is a key that means something else out on the street.
   *
   * Implements: REQ-TOOL-060, REQ-TOOL-061
   */
  wheelKey(code) {
    if (!this.wheel.open) return false;
    const digit = toolForKey(code);
    if (digit) { this.wheel.close(); this.setTool(digit); return true; }
    switch (code) {
      case 'KeyQ': this.wheel.close(); this.nextCarried(); return true;
      case 'KeyE': this.wheel.close(); this.nextPrimary(); return true;
      // Esc backs out of it; V and M are let through to leave walk mode altogether,
      // which puts the wheel away on the way out like everything else.
      case 'Escape': this.closeWheel(false); return true;
      case 'Enter': this.closeWheel(true); return true;
      default: return false; // walking, jumping and running carry on underneath it
    }
  }

  // ------------------------------------------------------------------ photographs

  /**
   * Puts a photograph up on the back of the camera and holds it in front of the
   * walker to look at. The camera is taken out to do it and whatever was in that hand
   * goes back into it afterwards, so looking at a picture costs the walker nothing
   * except the few seconds they spend on it - which is what the map's own panel does
   * with the details, and for the same reason.
   *
   * Returns whether it took: there is nothing to put a picture on outside walk mode.
   *
   * Implements: REQ-HUNT-042
   */
  showPhoto(photo) {
    if (!this.active || !photo?.url) return false;
    // Looking at a second picture without having put the first one down keeps the tool
    // the first one displaced, rather than settling for the camera it left in the hand.
    const was = this.showing ? this.showing.was
      : this.primary.id === 'camera' ? null : this.primary.id;
    this.endShow(false);
    if (this.primary.id !== 'camera') this.setTool('camera');
    // The image arrives after the frame that asked for it, and how it is fitted to the
    // screen depends on its shape - so it is hung once now, for the texture, and again
    // when the picture itself is there to be measured.
    const texture = new THREE.TextureLoader().load(photo.url, () => this.hangPhoto());
    this.showing = { t: 0, texture, was };
    this.hangPhoto();
    this.setFrozen(false);
    this.flash(`Photograph ${photo.n}${photo.where ? ` - ${photo.where}` : ''} - click to put it away`);
    this.drawHud();
    return true;
  }

  /** Hangs whatever is being looked at on the camera in hand, if that is what is in it. */
  hangPhoto() {
    if (this.showing && this.viewmodel) this.primary.shows?.(this.viewmodel, this.showing.texture);
  }

  // Implements: REQ-HUNT-044
  /** One frame of looking at one: it comes up, it is held, and it goes back down. */
  study(dt) {
    this.showing.t += dt;
    if (this.showing.t >= SHOWING) this.endShow();
  }

  /**
   * How far up at the face the camera is: in over LIFTING, held there, and back down.
   * Eased at both ends, because the last stretch of the way in is the one that fills
   * the view, and a straight ramp arrives at it like a slammed door.
   */
  raised() {
    const t = this.showing?.t ?? 0;
    const k = Math.max(0, Math.min(1, t / LIFTING, (SHOWING - t) / LIFTING));
    return k * k * (3 - 2 * k);
  }

  /**
   * Takes the photograph off the camera. `restore` puts back the tool that was in hand
   * before it went up, which is right when the picture's time is up and wrong when the
   * walker has just asked for a different tool.
   *
   * Implements: REQ-HUNT-044, REQ-HUNT-045
   */
  endShow(restore = true) {
    const show = this.showing;
    if (!show) return;
    this.showing = null;
    if (this.viewmodel) this.primary.shows?.(this.viewmodel, null);
    show.texture.dispose();
    if (restore && show.was && this.active) this.setTool(show.was);
    else this.drawHud();
  }

  /**
   * The lens the camera tool's screen looks through: the walk camera's place and
   * heading, a little tighter, so the picture on the back is what a camera held up
   * would actually be framing rather than the whole view.
   */
  lens() {
    const cam = this.scene.walkCamera;
    this.filmCamera ||= new THREE.PerspectiveCamera(1, 4 / 3, 0.02, 3000);
    const lens = this.filmCamera;
    lens.position.copy(cam.position);
    lens.quaternion.copy(cam.quaternion);
    const fov = cam.fov * 0.8;
    if (lens.fov !== fov) { lens.fov = fov; lens.updateProjectionMatrix(); }
    lens.updateMatrixWorld();
    return lens;
  }

  // Each tool breathes while it waits and swings while it is used; the swing is what
  // makes the gesture legible, so it runs to its end even if the shot lands sooner.
  // The two hands keep their own gestures, so netting a bug while the jet is running
  // is one hand doing each.
  // Implements: REQ-TOOL-031
  poseTool(dt, now) {
    // A tool with something live on it gets its frame here. Every other frame is
    // enough for a screen this size, and halves what it costs.
    const live = !((this.frames = (this.frames || 0) + 1) & 1);
    this.swing = this.poseOne(this.viewmodel, this.primary, this.swing, dt, now, live);
    this.offSwing = this.poseOne(this.offhand, this.secondary, this.offSwing, dt, now, live);
    // A photograph being looked at is laid over the right hand afterwards: whatever
    // that hand was doing, it is now holding the camera up at the face.
    if (this.showing && this.viewmodel) studyTool(this.viewmodel, this.raised());
  }

  // Implements: REQ-TOOL-002
  /** One hand's frame; returns how far into its gesture it now is, -1 once it is over. */
  poseOne(vm, tool, swing, dt, now, live) {
    if (!vm || !tool) return -1;
    if (live && tool.live) tool.live(vm, this.scene, this.lens());
    if (swing >= 0) {
      swing += dt;
      const u = swing / SWING;
      if (u < 1) {
        tool.pose(vm, u, now);
        return swing;
      }
      tool.pose(vm, 1, now); // land the gesture on its own end state
      restTool(vm);
    }
    idleTool(vm, now, this.pace, dt);
    return -1;
  }

  /**
   * Whether the walker is held where they stand: they do not move, look, aim or fire,
   * and nothing that acts on them over time - the jet's tank, their wind, the water
   * closing over them, a bug's bite - advances either, because all of it lives in
   * step() and step() does not run.
   *
   * Two different things ask for it. A panel being read has taken the pointer away
   * (frozen). The wheel is up in front of a walker who still has it, and is held for
   * the opposite reason: changing hands is not a thing to be punished for. Stopping
   * to pick a tool should not burn a tank, drown anybody, or hand the bug at your
   * ankle a free bite - and a walker who cannot see past the wheel should not be
   * walking off a roof behind it.
   *
   * The city is not held, only the walker: the bugs go on walking their laps. Two
   * hundred of them stopped mid-stride and started again is a worse thing to watch
   * than a street that carries on, and with bites frozen none of them can charge for
   * the pause.
   *
   * Implements: REQ-TOOL-062, REQ-TOOL-063
   */
  get still() { return this.frozen || this.wheel.open; }

  /**
   * Hold the view still while something else has the pointer (the details panel), or
   * let it go again. Frozen, the walker does not move, look, aim or fire; the scene
   * keeps rendering, so what is being read about stays on screen.
   *
   * Implements: REQ-WALK-021, REQ-WALK-022
   */
  setFrozen(on) {
    if (this.frozen === on || (!this.active && on)) return;
    this.frozen = on;
    // Softening the street says which of the two things on screen is waiting: the
    // walker, not the reader (.w-veil).
    this.hud.classList.toggle('held', on);
    if (!on) this.hooks.onResume?.(); // whatever was being read is done with
    if (on) {
      this.keys.clear(); // a key held when the panel opened must not walk on
      this.firing = false;
      this.closeWheel(false); // ... and a wheel left up under a panel is unreachable
      this.setScoped(false);
      // The aim is not recomputed while frozen, so whatever the crosshair was on
      // would keep its hover card - on top of the panel that is being read.
      this.aim = { i: -1, point: null, bug: null, box: null };
      this.showTarget(null);
      this.hooks.onAim(-1);
    }
    this.drawHud();
  }

  // Locking is asynchronous: a lock asked for just before leaving would be granted
  // afterwards and hide the cursor over the map, with nothing listening to it.
  // Implements: REQ-WALK-010, REQ-WALK-037
  lockPointer() {
    // Not while anything on screen wants the mouse. The introduction, the help, the
    // backpack, the photographs and the export menu all do, and a reticle that takes
    // it back underneath one of them leaves a panel nobody can click and a street that
    // answers every click instead. The map owns that list, because the map is what
    // opens them (hooks.busy).
    if (!this.active || this.hooks.busy?.()) return;
    this.setFrozen(false); // taking the pointer back is how you walk on
    const done = this.scene.renderer.domElement.requestPointerLock?.();
    // Older browsers return nothing and report the refusal on the document instead,
    // which is why refused is also wired to pointerlockerror below.
    if (!done?.then) return;
    done.then(() => this.active || document.exitPointerLock(), err => this.refused(err));
  }

  /**
   * The pointer lock was refused. Refusals come in two kinds and they have to be
   * told apart: a frame that withholds the lock refuses every time - an editor's
   * built-in browser puts the page in one, and so does anything else that frames
   * the map without allowing it - while a browser still winding down a lock that
   * was just released refuses once and grants the next request.
   *
   * A sandboxed frame says which it is in the error. Where it does not say, two
   * refusals with no lock in between are taken as the same answer.
   *
   * Walk mode is still worth having without the lock, because dragging already
   * turns the view. But a click has to stop asking and start using the tool, or the
   * tool can never be used at all - and it is said out loud once, because a mouse
   * that will not be captured looks like something broken rather than something
   * that was decided elsewhere.
   *
   * Implements: REQ-WALK-047
   */
  refused(err) {
    const sandboxed = err?.name === 'SecurityError' && /sandbox/i.test(err.message || '');
    if (!sandboxed && ++this.lockFails < 2) return;
    if (this.noLock) return;
    this.noLock = true;
    this.flash('The mouse cannot be captured here: drag to look, click to use the tool');
    this.drawHud();
  }

  /** A message about what just happened, shown for a few seconds. */
  flash(text) {
    const el = this.hud.querySelector('.w-flash');
    el.textContent = text;
    clearTimeout(this.flashTimer);
    this.flashTimer = setTimeout(() => { el.textContent = ''; }, 5000);
  }

  setScoped(on) {
    this.scoped = on;
    this.hud.classList.toggle('scoped', on);
  }

  // ------------------------------------------------------------------ input

  bindInput() {
    const canvas = this.scene.renderer.domElement;
    let drag = null;

    // Where the reticle is on the screen, which is where the hover card goes. It is
    // asked for every frame, and getBoundingClientRect is a forced layout - sixty a
    // second, for a number that only moves when the canvas does. So it is measured
    // when the canvas is resized, which is the only thing that moves it: the window,
    // or the details panel taking a third of the width.
    const measure = () => { this.canvasRect = canvas.getBoundingClientRect(); };
    measure();
    new ResizeObserver(measure).observe(canvas);
    window.addEventListener('resize', measure);

    window.addEventListener('keydown', e => {
      if (!this.owns(e) || e.target.closest('input, select, textarea, dialog') || e.ctrlKey || e.metaKey || e.altKey) return;
      // This listener is registered before the map's; stopping here keeps the map from
      // acting on the same key (V would leave walk mode and re-enter it at once).
      const mine = () => { e.preventDefault(); e.stopImmediatePropagation(); };
      // Implements: REQ-WALK-046
      if (this.frozen) {
        // Held, the walker is not playing and the page is. So the page keeps its keys
        // and only the few that put the street back are taken here - where swallowing
        // the rest meant that once the pointer had been let go, a button in the
        // toolbar could not be worked by keyboard at all.
        if (e.repeat) return;
        // ... and not even those from a control somebody has tabbed to, where Enter
        // and Space belong to the control. The Walk button is the exception: it keeps
        // the focus from the click that began the walk, so a walker is very often
        // standing on it without having chosen to.
        if (e.target !== document.body
          && e.target.closest('button:not(#walk), a[href], summary, [role="option"]')) return;
        switch (e.code) {
          case 'KeyV': case 'KeyM': mine(); this.exit(); break;
          case 'Escape':
          case 'Enter': mine(); this.setFrozen(false); this.lockPointer(); break;
        }
        return;
      }
      mine();
      if (e.repeat) return;
      this.keys.add(e.code);
      if (BIGGER.has(e.key)) this.setRadius(this.radius * 1.25);
      else if (SMALLER.has(e.key)) this.setRadius(this.radius / 1.25);
      // While the wheel is up it has first refusal on everything: it answers the keys
      // that pick from it and passes on the ones that walk, so a walker can keep
      // moving through a change of hands.
      if (this.wheelKey(e.code)) return;
      // The digits pick a tool outright, the way a shooter's number keys do, counting
      // along the row: 1 to 3 for the carried tools and 4 to 0 for the hunt's. A
      // carried tool's key pressed for the one already in hand puts it down, so it is
      // never a no-op.
      // Implements: REQ-TOOL-033, REQ-TOOL-054
      const digit = toolForKey(e.code);
      if (digit) {
        if (digit !== this.primary.id) this.setTool(digit);
        return;
      }
      switch (e.code) {
        // Two triggers for the off hand, because one hand's tool wants a button of its
        // own and the mouse's spare one is already the scope. C is free except while
        // flying, where it is how you go down.
        // Implements: REQ-TOOL-029, REQ-TOOL-030
        case 'KeyF': this.useSecondary(); break;
        case 'KeyC': if (!this.p.fly) this.useSecondary(); break;
        // Changing hands, all three of them within reach of the hand that is already
        // on W, A, S and D: the hunt's ring, the carried ring, and the wheel.
        // Implements: REQ-TOOL-056, REQ-TOOL-057, REQ-TOOL-058
        case 'KeyE': this.nextPrimary(); break;
        case 'KeyQ': this.nextCarried(); break;
        case 'KeyR': this.openWheel(); break;
        case 'KeyH': this.setHandsOff(!this.handsOff); break;
        case 'Escape':
          // Browsers usually swallow the Esc that frees the pointer; if not, free it first.
          if (document.pointerLockElement) document.exitPointerLock();
          else this.exit();
          break;
        // Implements: REQ-HUNT-006
        case 'Enter': this.hooks.onInspect(this.aimed()); break;
        // V is the toggle the map also answers to; M says where it goes, for anyone
        // who reaches for the map by name rather than remembering which way V points.
        case 'KeyV': case 'KeyM': this.exit(); break;
      }
    });
    window.addEventListener('keyup', e => {
      this.keys.delete(e.code);
      // R let go having pointed at something is the whole gesture; let go having
      // pointed at nothing, it leaves the wheel up.
      if (e.code === 'KeyR' && this.active) this.releaseWheel();
    });
    window.addEventListener('blur', () => { this.keys.clear(); this.firing = false; this.closeWheel(false); });

    // The pointer is locked at the reticle while walking (lockPointer): the mouse
    // looks around, the left button fires, holding the right one looks through the
    // scope. Once freed (Esc), or where locking is refused, a left drag looks around
    // and a left click locks it again.
    //
    // Buttons are taken from mouse events, not pointer events: pressing a second
    // button while one is held fires pointermove rather than pointerdown, so firing
    // while scoped would never arrive.
    let fresh = false; // the first movement after locking can carry a bogus jump
    // Implements: REQ-WALK-012, REQ-WALK-014, REQ-WALK-015, REQ-WALK-045
    canvas.addEventListener('mousedown', e => {
      if (!this.active || this.dying !== null) return;
      // Frozen means something else has the pointer - a panel, the backpack, a menu,
      // the toolbar over the street. The one thing a click on the map then means is
      // "walk on", which mouseup below answers; it must not also scope, fire, or take
      // hold of the trigger.
      //
      // It does have to be written down here, though. mouseup listens on the window,
      // because a button released off the edge of the canvas still has to be released;
      // `drag` is the only thing that tells it a click began on the map rather than on
      // a menu across the page, and while this returned before setting it, a click on
      // the map did nothing at all - which is what the HUD and the help had been
      // promising would walk on again.
      if (this.frozen) {
        if (e.button === 0) drag = { x: e.clientX, y: e.clientY, moved: false };
        return;
      }
      // The wheel has the mouse while it is up: the left button takes what it is
      // pointing at and the right one backs out, because a click is what a hand on the
      // mouse reaches for and neither the scope nor the tool is any use mid-change.
      // Implements: REQ-TOOL-060, REQ-TOOL-061
      if (this.wheel.open) {
        e.preventDefault();
        if (e.button === 0 || e.button === 1) this.closeWheel(true);
        else if (e.button === 2) this.closeWheel(false);
        return;
      }
      if (e.button === 2) {
        this.setScoped(true);
        return;
      }
      // The middle button is the off hand, the way F is: a second tool wants a second
      // trigger, and the right one is already the scope.
      // Implements: REQ-TOOL-029
      if (e.button === 1 && document.pointerLockElement === canvas) {
        e.preventDefault();
        this.useSecondary();
        return;
      }
      if (e.button !== 0) return;
      if (document.pointerLockElement === canvas) {
        // Held down, a tool with a cadence keeps going (loop). One shot leaves now
        // either way, so a tap is a tap whatever the tool is.
        this.firing = true;
        this.fire();
      } else drag = { x: e.clientX, y: e.clientY, moved: false };
    });
    window.addEventListener('mouseup', e => {
      if (e.button === 2) this.setScoped(false);
      if (e.button === 0) this.firing = false;
      if (this.active && drag && !drag.moved && e.button === 0) {
        // A click that went nowhere: normally that asks for the mouse. Where the
        // mouse is not given, it is the tool instead - or the way back out of
        // reading, which is what asking for the mouse would have done.
        if (!this.noLock) this.lockPointer();
        else if (this.frozen) this.setFrozen(false);
        else this.fire();
      }
      if (e.button === 0) drag = null;
    });
    // Implements: REQ-WALK-011, REQ-WALK-047
    window.addEventListener('pointermove', e => {
      if (!this.active) return;
      // The wheel takes the mouse whether or not the pointer was ever captured. Where
      // the lock is refused the view is turned by dragging, and a wheel that could
      // only be aimed by dragging is a wheel nobody would find - and Esc, which is one
      // of the ways to back out of it, hands the lock back on its way past.
      if (this.wheel.open) { this.aimWheel(e.movementX || 0, e.movementY || 0); return; }
      if (document.pointerLockElement === canvas) {
        if (fresh) { fresh = false; return; }
        // Clamp implausible jumps (some browsers report one after focus changes)
        // instead of dropping them: a fast flick still turns the view.
        const cap = v => Math.max(-MAX_LOOK_STEP, Math.min(MAX_LOOK_STEP, v));
        this.look(cap(e.movementX), cap(e.movementY));
      } else if (drag && e.buttons & 1) {
        drag.moved ||= Math.hypot(e.clientX - drag.x, e.clientY - drag.y) > 4;
        if (drag.moved) this.look(e.movementX, e.movementY);
      }
    });
    window.addEventListener('blur', () => this.setScoped(false));
    // Implements: REQ-WALK-013
    canvas.addEventListener('wheel', e => {
      if (!this.active || this.frozen) return;
      e.preventDefault();
      this.fov = clamp(this.fov * Math.exp(e.deltaY * 0.001), MIN_FOV, MAX_FOV); // zoom
    }, { passive: false });
    document.addEventListener('pointerlockerror', () => this.active && this.refused(null));
    // Implements: REQ-WALK-044
    document.addEventListener('pointerlockchange', () => {
      fresh = document.pointerLockElement === canvas;
      if (fresh) this.lockFails = 0; // it can be had here; earlier refusals were passing
      if (fresh && !this.active) document.exitPointerLock(); // never keep the map's cursor hidden
      // Where the pointer can be captured at all, having it is what walking is, and
      // this is the one place that decides. The mouse gone somewhere else - Esc, a
      // switch to another tab, a reach for the toolbar over the street - holds the
      // walker: one left running behind a dropdown keeps walking on whatever key was
      // down when the pointer went, spends their wind, and can drown or walk off a
      // roof while somebody is reading a menu. The mouse back on the street starts
      // them again.
      //
      // Both halves, because half of it does not work. Holding on the way out without
      // letting go on the way back in leaves a walker with the pointer captured and
      // held still anyway, which is every control taken away at once - and it happens
      // on nothing rarer than a click, since letting go of the reading closes the
      // panel, the backpack and the photographs, and each of those asks for the
      // pointer again on its way out. Reading the lock rather than counting the asks
      // is also what makes that harmless.
      //
      // Not where the lock was refused in the first place: there the view is turned by
      // dragging and the walker never has the pointer to lose, so this would hold them
      // still for good.
      if (this.active && !this.noLock) this.setFrozen(!fresh);
      if (this.active) this.drawHud();
    });
  }

  // Implements: REQ-WALK-011, REQ-WALK-012
  look(dx, dy) {
    if (this.frozen) return;
    const k = LOOK * this.scene.walkCamera.fov / FOV; // steadier through the scope
    this.p.yaw -= dx * k;
    this.p.pitch = clamp(this.p.pitch - dy * k, -1.5, 1.5);
  }

  // The planet can grow until the map looks flat, not beyond.
  // Implements: REQ-WALK-009
  maxRadius() {
    const l = this.limits;
    if (!l) return MAX_R;
    return clamp(3 * Math.hypot(l.maxX - l.minX, l.maxZ - l.minZ), 60, MAX_R);
  }

  setRadius(r) {
    this.radius = clamp(r, MIN_R, this.maxRadius());
    this.scene.setRadius(this.radius);
    this.setFog();
    this.drawHud();
  }

  // Fog fades what lies near the horizon; a larger planet, or a higher flight, shows
  // farther.
  // Implements: REQ-WALK-017
  setFog() {
    const far = Math.max(60, this.radius * 2.5) + 6 * Math.max(0, this.p.feet);
    Object.assign(this.scene.scene.fog, { near: far * 0.3, far });
  }

  // ------------------------------------------------------------------ simulation

  // Implements: REQ-WALK-042
  loop() {
    this.frame = requestAnimationFrame(() => {
      if (!this.active) return;
      const now = performance.now();
      const dt = Math.min(0.05, (now - this.last) / 1000);
      this.last = now;
      // Dying is watched rather than played: the walker stops steering and the red
      // deepens instead, until it takes them back to the map.
      if (this.dying !== null) this.fade(dt);
      else this.step(dt);
      if (this.showing) this.study(dt);
      this.autoFire(now);
      if (this.p.fly) this.setFog();
      this.zoom(dt);
      this.updateDarts(dt);
      // The walker's eye, for the catches that draw a bug in towards them - and the
      // hoop of the net, for the one catch that carries a bug somewhere else.
      const hoop = this.primary.catchAs === 'net' ? this.muzzle(this.viewmodel, HOOP_AT) : null;
      this.bugs?.update(dt, now, this.eye(EYE_AT), hoop);
      this.douse(dt);
      this.drawRadar(now, dt);
      this.health.draw(now); // the wash a hit leaves has to come off by itself
      this.drawFuel();
      this.poseTool(dt, now);
      this.scene.setWalker(this.p.x, this.p.feet, this.p.z, EYE + this.ride(dt), this.p.yaw, this.p.pitch);
      if (!this.still) this.updateAim();
      this.scene.renderNow();
      this.hooks.onRender();
      this.loop();
    });
  }

  /**
   * A held trigger, for the tools that have a cadence: the nail gun and the
   * extinguisher keep going while the button is down, which is what separates a hose
   * from the single aimed shot everything else takes. fire() lands the first one, so
   * this only ever adds the ones after it.
   *
   * Implements: REQ-TOOL-043
   */
  autoFire(now) {
    const every = this.primary.auto;
    if (!every || !this.firing || this.still || this.dying !== null) return;
    if (now - this.firedAt < every * 1000) return;
    this.fire(); // which is what sets the clock for the next one
  }

  /**
   * How far the eye is riding above where it would otherwise be. A jet holds the walker
   * up on something that breathes and water swells under a pair of floats, so the view
   * moves even when the walker's feet do not - and does not move at all when those feet
   * are on the ground, whatever they are doing.
   *
   * It is added to the eye height and nowhere else: what can be reached, what the
   * crosshair is on and where a shot leaves from are all measured from the feet, so
   * none of them wander with it.
   *
   * How far it rides is eased rather than switched, and the swell is advanced by this
   * frame's turn rather than read off the clock. Switched, stepping ashore would end
   * the swell wherever it had got to and drop the view by that much in one frame; read
   * off the clock, a change of rate would multiply the whole of the elapsed time and
   * jump the phase by however many radians that came to.
   *
   * Implements: REQ-WALK-036, REQ-WALK-042, REQ-WALK-043
   */
  ride(dt) {
    const how = this.p.fly ? RIDE.fly : this.onWater() ? RIDE.float : null;
    this.rideLift += ((how ? how.lift : 0) - this.rideLift) * Math.min(1, dt * 4);
    this.ridePhase = (this.ridePhase + dt * (how?.rate ?? RIDE.float.rate) * Math.PI) % (2 * Math.PI);
    return Math.sin(this.ridePhase) * this.rideLift;
  }

  // Eases the field of view towards the scope's or the normal one.
  // Implements: REQ-WALK-012, REQ-WALK-042
  zoom(dt) {
    const cam = this.scene.walkCamera, target = this.scoped ? Math.min(SCOPE_FOV, this.fov) : this.fov;
    if (Math.abs(cam.fov - target) < 0.05) return;
    cam.fov += (target - cam.fov) * Math.min(1, dt * 14);
    cam.updateProjectionMatrix();
  }

  // Implements: REQ-WALK-004, REQ-WALK-006, REQ-WALK-008, REQ-WALK-026
  step(dt) {
    if (this.still) return;
    const k = this.keys, p = this.p;
    // A line in a wall pulls the walker along it, past walls and gravity both, and
    // nothing else moves them until it lets go. Jump cuts it.
    if (this.pull) {
      if (k.has('Space')) this.cutLine('Line cut');
      else {
        this.sinking = false; // a line out of the water is a way out of it
        return this.reel(dt);
      }
    }
    const turn = (k.has('ArrowLeft') ? 1 : 0) - (k.has('ArrowRight') ? 1 : 0);
    p.yaw += turn * TURN * dt;
    const fwd = (k.has('KeyW') || k.has('ArrowUp') ? 1 : 0) - (k.has('KeyS') || k.has('ArrowDown') ? 1 : 0);
    const side = (k.has('KeyD') ? 1 : 0) - (k.has('KeyA') ? 1 : 0);
    // Sprinting is the legs' work, so it is the legs that pay for it; a jet carries
    // the walker on its own tank and asks nothing of them.
    const wants = k.has('ShiftLeft') || k.has('ShiftRight');
    const run = wants && (p.fly || this.wind.ready);
    this.wind.breathe(dt, run && !p.fly && (fwd !== 0 || side !== 0));
    // A burst on the jet backpack runs down whether or not it is being used to go
    // anywhere, so opening the throttle is a decision rather than a switch.
    this.burst = Math.max(0, this.burst - dt);
    const speed = (p.fly ? (run ? FLY * 2.5 : FLY) : run ? RUN : WALK)
      * (this.burst > 0 ? BURST_SPEED : 1);
    // On foot, W and S move level; flying, they move where the view points (look
    // down and press W to dive), and Space and C add straight up and down.
    const lift = p.fly ? (k.has('Space') ? 1 : 0) - (k.has('KeyC') ? 1 : 0) : 0;
    const level = p.fly ? Math.cos(p.pitch) : 1;
    let mx = -Math.sin(p.yaw) * level * fwd + Math.cos(p.yaw) * side;
    let mz = -Math.cos(p.yaw) * level * fwd - Math.sin(p.yaw) * side;
    let my = p.fly ? Math.sin(p.pitch) * fwd + lift : 0;
    const len = Math.hypot(mx, my, mz);
    if (len > 1) { mx /= len; my /= len; mz /= len; }

    // Axis by axis, so the walker slides along walls. There is one question, and it is
    // the same one everywhere: can they get up onto that? The bay is not a wall around
    // the map, it is ground half a unit lower than the shore - so it can be walked
    // into, waded about in and, with a shore to hand, climbed out of, exactly as a
    // sunken yard could be. What it does to somebody standing in it is the water's
    // business (drowns) rather than the movement's.
    const wet = !p.fly && this.height(p.x, p.z, p.feet) <= WATER;
    const afloat = !p.fly && this.floating();
    // ... and getting out is the one thing that needs help: the shore stands further
    // above the surface than a step, so anybody down there - on a pair of floats or in
    // it - carries an allowance to climb it. It is the water that gives this and not
    // the skimmers, so wearing them on a street is not a reason to climb higher walls.
    const inWater = p.ground && (wet || this.onWater());
    // Implements: REQ-WALK-040
    const climb = p.feet + (p.ground || p.fly ? STEP : 0.05) + (inWater ? WADE : 0);
    // Getting in has one rule of its own, and it is about the drop rather than the
    // water: a shore is a curb to step off and a bridge is not. Off a deck, and off
    // anything else standing well above the surface, the bay has to be jumped into -
    // walking off an edge into a drop is not a thing anybody means to do, and the
    // railings are there to be gone over rather than through. In the air, in it
    // already, or shod for it, none of this arises.
    const step = !this.onDeck();
    // Implements: REQ-WALK-041
    const ok = h => h <= climb
      && (h > WATER || p.fly || !p.ground || wet || afloat || (step && p.feet - h <= WADE_IN));
    const nx = p.x + mx * speed * dt;
    if (ok(this.height(nx, p.z, p.feet))) p.x = nx;
    const nz = p.z + mz * speed * dt;
    if (ok(this.height(p.x, nz, p.feet))) p.z = nz;
    // How hard the walker is moving, eased: the tool in their hands sways with it.
    const effort = len > 0 ? (p.fly ? 0.3 : run ? 1.5 : 1) : 0;
    this.pace += (effort - this.pace) * Math.min(1, dt * 7);
    // The key list folds away while moving and comes back after a pause.
    // Implements: REQ-WALK-018
    if (len > 0) {
      this.movedAt = performance.now();
      if (!this.hudTimer) this.hudTimer = setTimeout(() => this.hud.classList.add('compact'), 2500);
    } else if (this.hudTimer && performance.now() - this.movedAt > 6000) {
      clearTimeout(this.hudTimer);
      this.hudTimer = 0;
      this.hud.classList.remove('compact');
    }

    const planted = this.scene.props?.userData.obstacles || null;
    if (planted !== this.props) this.indexProps(planted);
    this.clearProps();
    this.confine();

    // What the walker is standing on - or the surface of the water, while the
    // skimmers are out and there is nothing under it.
    const floor = this.height(p.x, p.z, p.feet);
    // Running dry takes effect on the next frame rather than this one, so the walker
    // reads the flash before the ground arrives.
    // Only what the tool is actually doing costs anything: the jet burns while it is
    // holding the walker off the ground, not while they stand on a roof wearing it, and
    // the skimmers only while the water is the only thing under them.
    // Implements: REQ-TOOL-047, REQ-TOOL-049
    this.burn(dt, (p.fly && p.feet > floor + 0.02) || (afloat && floor <= WATER));
    p.fly = this.flying();
    if (p.fly) {
      const ceiling = (this.limits?.maxY ?? 0) + SKY_MARGIN;
      p.feet = Math.max(floor, Math.min(ceiling, p.feet + my * speed * dt));
      p.vy = 0;
      p.ground = p.feet <= floor;
      this.fell = null;
      this.sinking = false; // nothing in the air is drowning
      return;
    }
    // Implements: REQ-WALK-005, REQ-WALK-038
    if (k.has('Space') && p.ground && this.wind.spend(Wind.jumpCost)) p.vy = JUMP;
    // Said once, when it happens: a bar at nought explains why running and jumping
    // stopped working, but only to somebody already looking at it.
    if (this.wind.spent && !this.blown) this.flash('Out of breath');
    this.blown = this.wind.spent;
    p.vy -= GRAVITY * dt;
    p.feet += p.vy * dt;
    // Where the fall started, so how far it was can be measured when it stops. A jump
    // counts from the top of its arc, which is what makes jumping off a roof cost the
    // roof's height and not a hand's breadth more.
    if (p.vy < 0) this.fell = Math.max(this.fell ?? p.feet, p.feet);
    p.ground = p.feet <= floor;
    if (p.ground) {
      if (this.fell !== null) this.land(floor);
      p.feet = floor;
      p.vy = 0;
    }
    this.bites();
    this.scorches();
    this.drowns(dt, floor, afloat);
    // ... and, for a walker none of the three has touched lately, time putting them
    // back together. Last, so that anything which has just landed this turn holds it
    // off rather than being half undone by it in the same frame.
    this.health.mend(dt);
  }

  /**
   * The water, for a walker with nothing holding them up. It takes a couple of seconds,
   * which is long enough to wade ashore from the shallows and nowhere near long enough
   * to cross the bay - so stowing the skimmers out over the water is the end of it,
   * which is the whole reason to look where you are going before you do.
   *
   * Implements: REQ-WALK-031
   */
  drowns(dt, floor, afloat) {
    const p = this.p;
    if (p.fly || afloat || floor > WATER || p.feet > WATER + 0.02) {
      this.sinking = false;
      return;
    }
    if (!this.sinking) {
      this.sinking = true;
      this.flash('In the water - get to a shore');
    }
    if (this.health.hurt(DROWN * dt)) this.die('The water');
  }

  // Implements: REQ-TOOL-023, REQ-TOOL-025, REQ-TOOL-049, REQ-TOOL-050
  /** Whether what is in the off hand is doing its work: it has to have something left
   * in it, and not have run out since it was taken out. */
  working(what) {
    const tool = this.secondary;
    return !!tool?.[what] && !this.dry.has(tool.id) && this.tank(tool) > 0;
  }

  /** Whether the thing in the off hand is flying the walker right now. */
  flying() { return this.working('flies'); }

  /** ... and whether it is holding them up on the water. */
  floating() { return this.working('floats'); }

  /**
   * Whether what the walker is standing on is a bridge deck. A deck is the one floor on
   * the map with open water beside it at about its own height, so it is the one place
   * where "step down into the bay" has to mean something other than what it means on a
   * shore.
   *
   * Implements: REQ-WALK-041
   */
  onDeck() {
    const p = this.p;
    if (!p.ground || p.fly) return false;
    for (const at of this.spans.get(cellKey(p.x, p.z)) || NO_CELL) {
      const deck = at(p.x, p.z);
      if (Number.isFinite(deck) && deck > WATER && Math.abs(deck - p.feet) <= 0.03) return true;
    }
    return false;
  }

  /** Whether the walker is standing on the water itself, rather than merely shod for it. */
  onWater() {
    const p = this.p;
    return !p.fly && p.ground && p.feet <= WATER + 0.02 && this.floating();
  }

  /**
   * The gauge for whatever is being carried, beside the health bar. It is hidden when
   * the off hand is empty or holding something that never runs out, so the row says
   * nothing rather than saying "full" about a grapple line.
   *
   * Implements: REQ-TOOL-048
   */
  drawFuel() {
    const box = this.fuelBox ||= this.hud.querySelector('.w-fuel');
    if (!box) return;
    const tool = this.secondary?.fuel ? this.secondary : null;
    box.hidden = !tool;
    if (!tool) return;
    const share = this.tank(tool);
    const shown = Math.round(share * 100);
    const spent = this.dry.has(tool.id);
    if (shown === this.fuelShown && spent === this.fuelSpent) return;
    this.fuelShown = shown;
    this.fuelSpent = spent;
    (this.fuelFill ||= box.querySelector('.w-fuel-fill')).style.width = `${shown}%`;
    box.dataset.state = spent ? 'empty' : share > 0.25 ? 'well' : share > 0 ? 'low' : 'empty';
    box.title = spent
      ? `${tool.label} has run out: it is ${shown}% filled, and works again once you put it away and take it out`
      : `${tool.label}: ${shown}% left, and it fills again while it is not in use`;
  }

  /** How full a carried tool's tank is, as a share of it; 1 for one with no tank. */
  tank(tool) {
    if (!tool?.fuel) return 1;
    const at = this.tanks.get(tool.id);
    return at === undefined ? 1 : at;
  }

  /**
   * The tanks, over one frame. Whatever is in the off hand and doing its work burns;
   * everything else fills. A tool that runs dry stops working where it stands, which
   * for the jet is a fall and for the skimmers is the water - so it is said out loud
   * before it happens rather than after.
   *
   * Implements: REQ-TOOL-047, REQ-TOOL-049, REQ-TOOL-050
   */
  burn(dt, using) {
    for (const id of SECONDARY_IDS) {
      const tool = toolFor(id);
      if (!tool.fuel) continue;
      const was = this.tank(tool);
      const spending = tool === this.secondary && using;
      const now = clamp(was + (spending ? -dt / tool.fuel.full : dt / tool.fuel.fills), 0, 1);
      this.tanks.set(id, now);
      if (!spending) continue;
      if (now === 0 && was > 0) {
        // Dead until it is taken out again, rather than coming back the moment a drop
        // has trickled in: an empty jet that keeps catching is worse than one that has
        // plainly stopped.
        this.dry.add(id);
        this.flash(`${tool.label} out - put it away and take it out again once it has filled`);
      } else if (now <= 0.25 && was > 0.25) this.flash(`${tool.label} running low`);
    }
  }

  /**
   * The end of a fall: what it was worth, and whether it was the end of the walk.
   *
   * Implements: REQ-WALK-027
   */
  land(floor) {
    const drop = this.fell - floor;
    this.fell = null;
    const damage = this.health.fall(drop);
    if (!damage) return;
    if (this.health.dead) this.die(`A fall of ${Math.round(drop * 3.3)} storeys`);
    else this.flash(`That drop cost ${damage} - watch the roofs`);
  }

  /**
   * Anything close enough to bite, biting. One bug at a time and no faster than
   * BITE_EVERY, so a swarm is dangerous by being hard to get out of rather than by
   * taking the walker apart in a second - and it is the nearest one, so what bit is
   * what the crosshair is most likely already on.
   *
   * Implements: REQ-WALK-028
   */
  bites() {
    const now = performance.now();
    if (!this.bugs || now - this.bitAt < BITE_EVERY * 1000) return;
    const at = this.biteAt ||= new THREE.Vector3();
    at.set(this.p.x, this.p.feet + EYE * 0.55, this.p.z);
    const bug = this.bugs.at(at, BITE_REACH);
    if (!bug) return;
    this.bitAt = now;
    const damage = this.health.bite(bug.f.severity);
    if (this.health.dead) this.die(`${bug.f.severity}: ${bug.f.title}`);
    else this.flash(`Bitten - ${bug.f.severity}: ${bug.f.title} (-${damage}). Catch it or get clear`);
  }

  /**
   * The walk is over. The screen goes red and holds for a moment before the map comes
   * back, because a cut straight to the map reads as a bug rather than as dying - and
   * the walker stops answering to anything in the meantime, so the last second is
   * watched rather than played.
   *
   * It ends the way leaving on foot does, back to the map standing where you fell,
   * because the backpack is what a session is for and nothing in it is lost. Coming
   * back in is coming back at full health (enter).
   *
   * Implements: REQ-WALK-030, REQ-WALK-033
   */
  die(cause) {
    if (this.dying !== null) return;
    this.dying = 0;
    this.keys.clear();
    this.firing = false;
    this.setScoped(false);
    this.hud.classList.add('dead');
    this.flash(`${cause} finished you. Back to the map; walk in again to start over`);
  }

  /**
   * The red, deepening; at the end of it the walker is back on the map.
   *
   * Implements: REQ-WALK-033
   */
  fade(dt) {
    this.dying += dt;
    this.hud.style.setProperty('--dead', Math.min(1, this.dying / (DYING * 0.4)).toFixed(3));
    if (this.dying >= DYING) this.exit();
  }

  /** The block the walker stands on, to keep them by it across a relayout (reanchor). */
  anchorFor() {
    let box = null;
    for (const b of this.cellAt(this.p.x, this.p.z)) {
      if (Math.abs(this.p.x - b.x) <= b.w / 2 && Math.abs(this.p.z - b.z) <= b.d / 2
        && b.y <= this.p.feet + 0.01 && (!box || b.y > box.y)) box = b;
    }
    return box && { node: box.node, x: box.x, z: box.z, w: box.w, d: box.d };
  }

  /**
   * After a relayout, puts the walker back where they were relative to the anchor's
   * new box `b`: outside it, at the same distance from the same edge; over it, at
   * the same share of its size. So a depth change, a filter or a live update rebuilds
   * the city around the walker instead of teleporting them.
   *
   * Implements: REQ-WALK-025
   */
  reanchor(a, b) {
    const p = this.p, was = p.feet;
    const map = (v, c, s, c2, s2) => {
      const d = v - c, half = s / 2;
      return Math.abs(d) > half ? c2 + Math.sign(d) * (s2 / 2 + Math.abs(d) - half) : c2 + d * (s2 / Math.max(1e-6, s));
    };
    p.x = map(p.x, a.x, a.w, b.x, b.w);
    p.z = map(p.z, a.z, a.d, b.z, b.d);
    this.confine();
    this.makeRoom(was);
    const floor = this.height(p.x, p.z);
    p.feet = p.fly ? Math.max(p.feet, floor) : floor;
    p.vy = 0;
  }

  // Steps aside when a building now stands where the walker was put: the nearest
  // spot no higher than the level they were on.
  // Implements: REQ-WALK-025
  makeRoom(level) {
    const p = this.p, fits = (x, z) => this.height(x, z) <= level + STEP;
    if (fits(p.x, p.z)) return;
    for (let r = 0.3; r <= 6; r += 0.3) {
      for (let i = 0; i < 16; i++) {
        const a = i / 16 * Math.PI * 2;
        const x = p.x + Math.cos(a) * r, z = p.z + Math.sin(a) * r;
        if (fits(x, z)) { p.x = x; p.z = z; return; }
      }
    }
  }

  /**
   * The props the current layout planted, in the same kind of grid as the boxes.
   * MapScene rebuilds them on a relayout and on a style change, and the list is a new
   * array each time, so noticing that is a reference test rather than a subscription.
   */
  indexProps(list) {
    this.props = list;
    this.propGrid = new Map();
    for (const o of list || []) {
      const k = cellKey(o.x, o.z);
      const cell = this.propGrid.get(k);
      if (cell) cell.push(o); else this.propGrid.set(k, [o]);
    }
  }

  /**
   * Pushes the walker off anything standing on the map they have walked into: a trunk,
   * a lamp post, a crystal. Boxes are cleared by never stepping into them, axis by
   * axis, which is what makes a wall slide; a trunk is a circle, and pushing out of a
   * circle along its own radius slides around it without any of that bookkeeping.
   *
   * Twice over, because stepping off one tree can be a step into the next.
   */
  clearProps() {
    const p = this.p;
    if (p.fly || !this.props?.length) return;
    const nearby = this.nearby ||= []; // reused: this runs twice every frame
    for (let pass = 0; pass < 2; pass++) {
      let moved = false;
      for (const o of this.propsNear(p.x, p.z, nearby)) {
        if (Math.abs(o.y - p.feet) > PROP_REACH) continue;
        const dx = p.x - o.x, dz = p.z - o.z;
        const want = o.r + BODY;
        const d2 = dx * dx + dz * dz;
        if (d2 >= want * want) continue;
        const d = Math.sqrt(d2);
        // Dead centre: push along the way they came rather than picking an axis.
        const [ux, uz] = d > 1e-4 ? [dx / d, dz / d] : [Math.sin(p.yaw), Math.cos(p.yaw)];
        const x = o.x + ux * want, z = o.z + uz * want;
        // Never push someone through a wall or up onto a roof to get them off a tree,
        // and never push them off what they are standing on into the water - a
        // bridge is narrow and a push across it would go over the side. Standing in
        // the trunk is the lesser of all of those.
        const to = this.height(x, z);
        if (to > p.feet + STEP || (to <= WATER && p.feet > WATER)) continue;
        p.x = x;
        p.z = z;
        moved = true;
      }
      if (!moved) return;
    }
  }

  /** The props whose cell the point is in, or next to. */
  propsNear(x, z, out) {
    out.length = 0;
    const i = Math.floor(x / CELL), j = Math.floor(z / CELL);
    for (let di = -1; di <= 1; di++) {
      for (let dj = -1; dj <= 1; dj++) {
        const cell = this.propGrid.get(cellOf(i + di, j + dj));
        if (cell) for (const o of cell) out.push(o);
      }
    }
    return out;
  }

  /**
   * Keeps the walker over the map or the water just off its shores.
   *
   * Implements: REQ-WALK-007
   */
  confine() {
    const l = this.limits, p = this.p;
    if (!l) return;
    p.x = clamp(p.x, l.minX - SHORE_MARGIN, l.maxX + SHORE_MARGIN);
    p.z = clamp(p.z, l.minZ - SHORE_MARGIN, l.maxZ + SHORE_MARGIN);
  }

  /**
   * Top of the solid column under a body at (x, z): the highest box, ramp or bridge
   * deck it overlaps.
   *
   * `from` is where the body already is, and a bridge deck more than a step above that
   * is a bridge the body is under rather than one it is on. Without it, walking the
   * water on skimmers put the walker on top of every deck they passed beneath, which
   * is the one place on the map where there is somewhere to be underneath.
   *
   * Implements: REQ-WALK-002, REQ-WALK-032, REQ-WALK-035, REQ-CITY-018, REQ-CITY-028, REQ-PERF-008
   */
  height(x, z, from = Infinity) {
    let top = WATER;
    for (let i = 0; i < PROBES.length; i += 2) {
      const px = x + PROBES[i], pz = z + PROBES[i + 1];
      for (const b of this.cellAt(px, pz)) {
        if (Math.abs(px - b.x) <= b.w / 2 && Math.abs(pz - b.z) <= b.d / 2) {
          top = Math.max(top, b.y + b.h);
        }
      }
      for (const at of this.decks.get(cellKey(px, pz)) || NO_CELL) top = Math.max(top, at(px, pz));
    }
    // A bridge carries the walker, not the corners of them. It is asked once, at
    // their middle, against a deck already narrowed by their own radius: asking at
    // the corners meant one corner on the deck was enough to stand on, so a body
    // could be walked out through the railing and left hanging over the water. On
    // the shores the deck overlaps there is land underneath, so stepping off at
    // either end is as free as it ever was.
    for (const at of this.spans.get(cellKey(x, z)) || NO_CELL) {
      const deck = at(x, z);
      if (deck <= from + STEP) top = Math.max(top, deck);
    }
    return top;
  }

  /**
   * The boxes indexed in the cell holding (x, z) - candidates, not answers: a cell is
   * larger than a box, so the caller tests the footprint.
   *
   * The test stays with the caller rather than here in a generator that yields only
   * what matches: the ray the crosshair marches calls this a few hundred times a
   * frame, and an iterator object per call is that many allocations to throw away.
   * Implements: REQ-PERF-008
   */
  cellAt(x, z) {
    return this.grid.get(cellKey(x, z)) || NO_CELL;
  }

  /** The box containing a flat-map point, or null. */
  boxAt(v) {
    for (const b of this.cellAt(v.x, v.z)) {
      if (Math.abs(v.x - b.x) <= b.w / 2 && Math.abs(v.z - b.z) <= b.d / 2
        && v.y >= b.y - 0.02 && v.y <= b.y + b.h + 0.02) return b;
    }
    return null;
  }

  aimed() {
    return this.aim.i >= 0 ? this.boxes[this.aim.i] : null;
  }

  /** Where the walker's eye is, into `out`, which saves one vector a frame. */
  eye(out) {
    return out.set(this.p.x, this.p.feet + EYE, this.p.z);
  }

  /**
   * Whether a box is the shore or the block the walker stands in.
   *
   * Implements: REQ-WALK-016
   */
  underfoot(b) {
    const p = this.p;
    return b.kind === 'land' || b.kind === 'terrace' && Math.abs(p.x - b.x) <= b.w / 2 && Math.abs(p.z - b.z) <= b.d / 2;
  }

  // Marches the crosshair ray through the bent view, mapping each sample back to the
  // flat map, until it enters a box or the water.
  // Implements: REQ-TOOL-022
  updateAim() {
    const cam = this.scene.walkCamera;
    const dir = new THREE.Vector3(0, 0, -1).applyQuaternion(cam.quaternion);
    const v = new THREE.Vector3();
    const tool = this.primary;
    // Both answers are the same at every step of the march, so they are asked once.
    const catches = hits(tool, 'bugs') && this.bugs ? this.bugs : null;
    const tags = hits(tool, 'buildings');
    let hit = null, point = null, bug = null;
    for (let t = 0.2; t < REACH; t += 0.04 + t * 0.008) {
      v.copy(cam.position).addScaledVector(dir, t);
      this.scene.unbend(v);
      if (v.y < WATER) break;
      // A bug walks in front of the building it belongs to, so it is tested first:
      // otherwise the wall behind it would always win. A tool that is no use against
      // bugs looks straight through them - and one that is no use against buildings
      // still stops at the wall, because the wall is still in the way. A secondary
      // tool is no use against either: what it marks is only where it would take you.
      bug = catches ? catches.at(v, BUG_AIM(t)) : null;
      if (bug) { point = v.clone(); break; }
      hit = this.boxAt(v);
      if (hit) { point = v.clone(); break; }
    }
    // The ground underfoot and the shore are scenery, not targets.
    const on = hit && !this.underfoot(hit) ? hit : null;
    let i = !bug && on && tags ? on.i : -1;
    // A tool with a reach is swung, not thrown: past it there is nothing to be done
    // about what the crosshair is on, which the HUD says rather than going blank.
    const far = tool.reach != null && point != null
      && cam.position.distanceTo(point) > tool.reach && (bug || i >= 0 || (tool.douses && on));
    if (far) { bug = null; i = -1; }
    // `box` is what the crosshair is on whether or not this tool can do anything to
    // it; `i` is what this tool would tag. They were the same until the extinguisher
    // needed to put a building's fire out without also tagging the module, which is
    // not what an extinguisher is for.
    this.aim = { i, point, bug, far, box: far ? null : on };
    this.showTarget(bug, far);
    const r = this.canvasRect;
    this.hooks.onAim(i, r.left + r.width / 2, r.top + r.height / 2);
  }

  // ------------------------------------------------------------------ darts

  /**
   * Where what the tool throws leaves it, in flat-map coordinates: the rod's tip, the
   * net's hoop, the wand's ring, the launcher's muzzle (tools.js marks the spot).
   * Null before the hand has loaded, and the walker's eye then stands in for it.
   *
   * The viewmodel hangs off the walk camera, which is at the walker's eye in those
   * same coordinates, so this needs no conversion - only an up-to-date matrix, since
   * a shot is fired between frames.
   *
   * Implements: REQ-TOOL-038
   */
  muzzle(vm = this.viewmodel, out = new THREE.Vector3()) {
    const m = vm?.getObjectByName('muzzle');
    if (!m) return null;
    m.updateWorldMatrix(true, false);
    return m.getWorldPosition(out);
  }

  // Using the primary tool on an aimed box sends whatever it throws along a shallow
  // arc to the aimed point, and it always arrives; used on nothing it flies ahead
  // under its own gravity and drag until it hits something or falls into the water. A
  // tool that throws nothing reaches what it is pointed at the moment it is used - as
  // far as it reaches, which for the camera is any distance and for the net is arm's
  // length.
  // Implements: REQ-HUNT-037, REQ-HUNT-038, REQ-HUNT-044, REQ-TOOL-029
  fire() {
    if (this.still || this.dying !== null) return;
    // A photograph is up: the click that would have taken another one puts this one
    // away instead, which is the obvious thing to do with a picture held in front of
    // your face and saves waiting out the rest of the timer.
    if (this.showing) return this.endShow();
    const tool = this.primary;
    this.firedAt = performance.now();
    this.swing = 0; // the hand moves whether or not anything flies
    const target = this.aimed(), bug = this.aim.bug;
    // A copy: a shot that scatters moves where it is going, and where it is going is
    // the crosshair's own point until the next frame recomputes it.
    const to = bug ? bug.pos.clone() : this.aim.point?.clone() || null;

    if (!tool.projectile) {
      // A camera keeps every frame it is used on, because pressing the shutter is what
      // a camera is for and anything less leaves its ordinary use doing nothing that
      // can be seen - every one but the use that asks to read a module it has already
      // tagged, which is the second click on a building and wants the details rather
      // than another picture of the same wall. What was in the frame goes with the
      // picture, so it is more than a number in a list: the bug that was under the
      // reticle, or the building behind it.
      // A bug in front of a tagged wall is still a bug: the rule is about the second
      // click on a building, not about everything standing in front of one.
      const again = !bug && !!target && this.tagged.has(target.node.id);
      const keeping = tool.keeps && !again;
      // The shutter goes off when a picture is taken and not when one is not.
      if (tool.flash && keeping) this.screenFlash();
      if (keeping) {
        this.hooks.onPhoto?.(bug ? `${bug.f.severity}: ${bug.f.title}` : target?.node.name || '');
      }
      if (bug) this.bugs.catch(bug, tool.catchAs);
      else if (target) this.tag(target);
      else if (this.aim.far) this.flash(`Out of reach: the ${tool.label.toLowerCase()} has to be walked up to`);
      return;
    }
    const shot = this.shotFrom(tool, this.viewmodel);
    const flight = shot.flight;
    if (bug || target) {
      // A tool that scatters does not land where it was aimed: the nail goes wide by
      // a share of how far it has to travel, which is nothing across a room and the
      // width of a window at the end of its reach.
      const dist = shot.start.distanceTo(to);
      if (flight.spread) scatter(to, flight.spread * dist);
      Object.assign(shot, { to, target, bug, T: Math.max(0.12, dist / flight.speed), arc: (0.05 + dist * 0.03) * flight.arc });
    } else {
      this.loose(shot);
    }
  }

  /**
   * The extinguisher, held on a burning building.
   *
   * Dousing is held rather than fired, which is the whole difference between this tool
   * and the rest of the bag. Every other primary tool is a gesture with a result: one
   * cast, one shot, one photograph. Fire does not answer to a gesture - it answers to
   * standing there and keeping the cone on it - so this is paid in seconds and not in
   * clicks, and the tool's own cadence is only what makes the foam keep coming.
   *
   * Putting out the building an advisory names puts out everything that fire lit,
   * wherever it has reached. That is not a mercy: it is what upgrading the dependency
   * actually does.
   */
  douse(dt) {
    if (!this.fires || !this.primary.douses || !this.firing || this.still) return;
    const box = this.aim.box;
    if (!box) return;
    const what = this.fires.douse(box.node.id, dt);
    if (what === 'out') this.flash(`Out - ${box.node.name} and everything that fire reached`);
    else if (what === 'cooled') this.flash(`${box.node.name} is out; the fire is still going elsewhere`);
  }

  /**
   * What a tool throws, leaving the muzzle of the hand that threw it, with its line
   * behind it if it trails one. Where it goes from there is the caller's business: the
   * crosshair's target for the hand the crosshair belongs to, and straight ahead for
   * the other one.
   *
   * Implements: REQ-TOOL-004, REQ-TOOL-038, REQ-TOOL-040
   */
  shotFrom(tool, vm) {
    const p = this.p;
    const start = this.muzzle(vm) || new THREE.Vector3(p.x, p.feet + EYE - 0.08, p.z);
    const mesh = tool.projectile(this.scene);
    mesh.position.copy(start);
    this.scene.scene.add(mesh);
    const shot = { mesh, t: 0, tool, hand: vm, start, flight: tool.flight || DEFAULT_FLIGHT };
    if (tool.line) { // a cast trails its line back to the hand it left
      const geo = new THREE.BufferGeometry().setFromPoints([start.clone(), start.clone()]);
      // Bendable like everything else out there: a line across a street on a small
      // planet is long enough for the curve to show.
      shot.line = new THREE.Line(geo, this.scene.bendable(new THREE.LineBasicMaterial({ color: tool.line })));
      shot.line.frustumCulled = false;
      this.scene.scene.add(shot.line);
    }
    this.darts.push(shot);
    return shot;
  }

  // Implements: REQ-TOOL-038
  /** Sends a shot off along the view, to fly on under its own physics. */
  loose(shot) {
    const p = this.p;
    const dir = new THREE.Vector3(-Math.sin(p.yaw) * Math.cos(p.pitch), Math.sin(p.pitch) + 0.04, -Math.cos(p.yaw) * Math.cos(p.pitch));
    if (shot.flight.spread) scatter(dir, shot.flight.spread);
    shot.vel = dir.normalize().multiplyScalar(shot.flight.speed);
    // Where it left from, so how far it has carried can be measured against the tool's
    // reach - and against the length of a line, for the ones that pay one out.
    shot.from = shot.start.clone();
  }

  /**
   * The off hand. What it carries is never aimed - the crosshair belongs to the hunt -
   * so a line fired from it goes where the walker is looking and bites whatever it
   * reaches, and the two that carry rather than throw do their one thing where they
   * stand.
   *
   * Implements: REQ-TOOL-024, REQ-TOOL-029, REQ-TOOL-034
   */
  useSecondary() {
    if (this.still || this.dying !== null) return;
    const tool = this.secondary;
    if (!tool) {
      this.flash('Nothing in your off hand - Q or 1, 2, 3 picks one up, or hold R for the wheel');
      return;
    }
    this.offSwing = 0;
    if (tool.fuel && (this.dry.has(tool.id) || this.tank(tool) <= 0)) {
      this.flash(`${tool.label} is spent - put it away and take it out again once it has filled`);
      return;
    }
    if (tool.flies) {
      // The throttle, wide open for a moment: a burst of speed rather than a shot.
      this.burst = BURST;
      this.flash('Thrusters');
      return;
    }
    if (tool.floats) {
      this.flash(this.height(this.p.x, this.p.z) <= WATER
        ? 'Riding the water' : 'The skimmers want water under them');
      return;
    }
    if (!tool.projectile) return;
    const shot = this.shotFrom(tool, this.offhand);
    // A line is aimed, even though the crosshair is not its own: the wall it is meant
    // to pull the walker up is the wall they are looking at, and a hook that landed
    // near it instead would be a tool nobody could use. Everything else the off hand
    // throws simply goes where the view points.
    const seen = tool.reel ? this.lookingAt(tool.reach ?? REACH) : null;
    if (!seen) {
      this.loose(shot);
      return;
    }
    const dist = shot.start.distanceTo(seen.point);
    Object.assign(shot, {
      to: seen.point, target: seen.box,
      T: Math.max(0.12, dist / shot.flight.speed),
      arc: (0.05 + dist * 0.03) * shot.flight.arc,
    });
  }

  /**
   * The box the walker is looking at and where the ray met it, for a tool that is not
   * the one the crosshair belongs to. The crosshair is the primary tool's, so the off
   * hand asks for itself - one march of the ray, once, at the moment it is used.
   *
   * Implements: REQ-TOOL-034
   */
  lookingAt(reach) {
    const cam = this.scene.walkCamera;
    const dir = new THREE.Vector3(0, 0, -1).applyQuaternion(cam.quaternion);
    const v = new THREE.Vector3();
    for (let t = 0.2; t < reach; t += 0.04 + t * 0.008) {
      v.copy(cam.position).addScaledVector(dir, t);
      this.scene.unbend(v);
      if (v.y < WATER) break;
      const box = this.boxAt(v);
      if (box) return { box, point: v.clone() };
    }
    return null;
  }

  // ---------------------------------------------------------------- the grapple

  /**
   * A line has stuck, and it pulls: the walker goes along it to what it caught.
   *
   * The grapple sets them on top of it, which is how a facade is got up and a roof
   * arrived on - and, aimed off that roof at anything lower, how they get down again.
   * The rod anchors where its hook landed instead, so a cast at the tenth floor
   * brings the walker to that wall rather than standing them on the roof.
   *
   * Returns whether the line was taken up. It may not be - too far to pull from, or
   * already there - and the caller has to know, because a hook that is not holding
   * anything has to come off the map with the rest of the shot.
   */
  hook(at, box, shot) {
    const line = shot.tool.reel;
    const to = at.clone();
    if (line.onto && box && box.kind !== 'land' && box.kind !== 'terrace') {
      // Onto it rather than against it: the top of the box, a step in from the face
      // so the landing is on the roof and not on its edge.
      to.y = box.y + box.h + 0.02;
      to.x += clamp(box.x - to.x, -ROOF_IN, ROOF_IN);
      to.z += clamp(box.z - to.z, -ROOF_IN, ROOF_IN);
    } else if (!box || box.kind === 'land' || box.kind === 'terrace') {
      to.y = this.height(to.x, to.z);
    }
    const away = Math.hypot(to.x - this.p.x, to.y - this.p.feet, to.z - this.p.z);
    // A hook that lands where the walker already is pulls them nowhere: looking
    // straight down from a roof catches that roof. Say so instead of paying out a
    // line and reeling in nothing - from up here, the way down is over the edge.
    if (away < Math.max(NO_PULL, line.stop)) {
      if (line.onto) this.flash('Nothing to be pulled to - aim past the edge');
      return false;
    }
    // And a line only so long: past that the cast still lands, it simply does not
    // drag the walker the width of the map to where it landed.
    if (away > line.max) {
      this.flash('Too far for the line');
      return false;
    }
    const up = to.y > this.p.feet;
    this.cutLine();
    this.pull = { to, t: 0, line, mesh: shot.mesh, rope: shot.line, hand: shot.hand };
    this.p.fly = this.flying(); // only one thing is carried, so a line is not a jet
    this.p.vy = 0;
    this.flash(up ? 'Line away - going up' : 'Line away - going down');
    this.drawHud();
    return true;
  }

  /** Reels the walker along the line, and lets go at the end of it. */
  reel(dt) {
    const p = this.p, to = this.pull.to;
    const dx = to.x - p.x, dy = to.y - p.feet, dz = to.z - p.z;
    const d = Math.hypot(dx, dy, dz);
    this.pull.t += dt;
    if (d < this.pull.line.stop || this.pull.t > GRAPPLE_TIME) {
      this.cutLine();
      // Let go standing on what was arrived at, rather than falling back off it.
      p.feet = Math.max(p.feet, this.height(p.x, p.z));
      p.vy = 0;
      p.ground = true;
      return;
    }
    const step = Math.min(d, this.pull.line.speed * dt);
    p.x += (dx / d) * step;
    p.feet += (dy / d) * step;
    p.z += (dz / d) * step;
    p.vy = 0;
    p.ground = false;
    // The hook stays where it bit, and the line follows the hand to it.
    if (this.pull.mesh) this.pull.mesh.position.copy(to);
    if (this.pull.rope) {
      const tip = this.muzzle(this.pull.hand) || new THREE.Vector3(p.x, p.feet + EYE - 0.05, p.z);
      this.pull.rope.geometry.setFromPoints([tip, to.clone()]);
    }
  }

  /** Lets go of whatever the line is holding, and takes the line off the map. */
  cutLine(say) {
    if (!this.pull) return;
    if (this.pull.mesh) this.scene.scene.remove(this.pull.mesh);
    if (this.pull.rope) this.scene.scene.remove(this.pull.rope);
    this.pull = null;
    if (say) this.flash(say);
  }

  // ------------------------------------------------------------------ the tracker

  /**
   * A top-down sweep centred on the walker and turning with them: every bug still on
   * the streets as a dot in its severity's color, every module already tagged as a
   * ring, and anything beyond the sweep's range pinned to its rim as an arrow - the
   * point of the thing being to say which way to walk. Under it, how far the nearest
   * bug is and what it is carrying.
   */
  /**
   * Standing in a fire.
   *
   * It takes the hottest one the walker is inside, because standing where two blazes
   * meet is not twice as survivable as standing in one, and taking from each in turn
   * would make it so.
   *
   * There is no getting bitten back here and nothing to swing at: the extinguisher
   * will put the fire out, but not from inside it and not quickly enough to matter, so
   * what this asks of a walker is to leave. Which is why it says so, and says how -
   * the flash names the tool, because a walker who has just found out that roofs are
   * dangerous is not in a frame of mind to go looking for it.
   */
  scorches() {
    const now = performance.now();
    if (!this.fires?.burning || this.dying !== null) return;
    if (now - this.burnAt < BURN_EVERY * 1000) return;
    let worst = null;
    for (const f of this.burning()) {
      if (!inBlaze(this.p.x, this.p.feet, this.p.z, f)) continue;
      if (!worst || f.heat > worst.heat) worst = f;
    }
    if (!worst) return;
    this.burnAt = now;
    const damage = Math.max(1, Math.round(BURN * worst.heat));
    this.health.hurt(damage);
    if (this.health.dead) this.die(`The fire on ${worst.node.name}`);
    else this.flash(`Burning - ${worst.node.name} (-${damage}). Get clear, or put it out with the extinguisher`);
  }

  /**
   * Where the fires are, in the flat map's own coordinates, for the sweep.
   *
   * Asked of the boxes rather than of fires.js, because what fires.js holds is node
   * ids and the sweep needs somewhere to put a dot: a fire on a package island, or on
   * a file inside a directory the map has collapsed, is burning at whatever the layout
   * is showing for it. Boxes are the only thing that knows that.
   */
  burning() {
    if (!this.fires?.burning) return [];
    const out = [];
    for (const b of this.boxes) {
      if (b.kind === 'land' || !b.node) continue;
      const heat = this.fires.heatOf(b.node.id);
      // The roof they stand on and the footprint they cover come too: the sweep wants
      // only x and z, but what decides whether a walker is standing in one needs all
      // of it (inBlaze).
      if (heat) out.push({ x: b.x, z: b.z, y: b.y + b.h, w: b.w, d: b.d, heat, node: b.node });
    }
    return out;
  }

  // Implements: REQ-HUNT-021, REQ-HUNT-022, REQ-HUNT-023, REQ-HUNT-024, REQ-HUNT-025, REQ-HUNT-026
  drawRadar(now, dt) {
    const box = this.hud.querySelector('.w-radar');
    // A map whose every advisory is reachable has no bugs walking it at all, and the
    // sweep is worth more there than anywhere: everything on it is on fire.
    const alight = this.burning();
    if (!this.bugs?.bugs.length && !alight.length) {
      box.hidden = true;
      return;
    }
    box.hidden = false;
    const canvas = box.querySelector('canvas');

    // The range fits whatever is still out there, so the sweep is never all centre
    // dot or all rim arrows. Once one is close, though, holding the whole map is the
    // wrong thing to hold: the range pulls in to the neighborhood and the dial grows
    // to meet it, which is the difference between knowing a bug is somewhere ahead
    // and seeing which side of the building it is on.
    const { x: px, z: pz, yaw } = this.p;
    const live = this.bugs?.bugs.filter(b => !b.caught) || [];
    let far = RADAR_MIN, near = Infinity;
    for (const at of [...live.map(b => b.pos), ...alight]) {
      const d = Math.hypot(at.x - px, at.z - pz);
      far = Math.max(far, d);
      near = Math.min(near, d);
    }
    const closing = near < RADAR_NEAR ? 1 - near / RADAR_NEAR : 0;
    this.radarNear = near; // what the sweep thinks it is closing on, for the tests

    // Both the range and the dial ease towards where they are going, and both ease
    // by elapsed time rather than by redraw, so the approach looks the same whatever
    // the frame rate is and whatever the throttle below decides.
    const want = closing
      ? clamp(Math.max(near * 2.4, RADAR_MIN), RADAR_MIN, RADAR_NEAR * 2.4)
      : clamp(far * 1.2, RADAR_MIN, RADAR_MAX);
    this.radarRange = ease(this.radarRange, want, RADAR_RANGE_TAU, dt);
    this.radarZoom = ease(this.radarZoom, 1 + closing * RADAR_GROW, RADAR_ZOOM_TAU, dt);
    // The dial grows by transform rather than by resizing the canvas: a scale is
    // sub-pixel and costs the compositor alone, where a resize rounded to whole
    // pixels grew in visible steps, threw the drawing away and reallocated the
    // bitmap on the way. The bitmap is therefore made once, at the size the dial
    // reaches when it is fully grown, so growing into it stays sharp.
    canvas.style.transform = `scale(${this.radarZoom.toFixed(3)})`;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const pixels = Math.round(RADAR_SIZE * (1 + RADAR_GROW) * dpr);
    if (canvas.width !== pixels) canvas.width = canvas.height = pixels;

    // The contents turn as slowly as a walker does, so they are repainted a dozen
    // times a second; the scale above follows every frame.
    if (now - (this.radarAt || 0) < RADAR_MS) return;
    this.radarAt = now;

    const cs = getComputedStyle(this.hud);
    const v = name => cs.getPropertyValue(name).trim();
    // Drawn in CSS pixels of the unscaled dial, whatever the bitmap behind it is.
    const size = RADAR_SIZE;
    const g = canvas.getContext('2d');
    const c = size / 2, R = c - 7;
    const unit = pixels / size;
    g.setTransform(unit, 0, 0, unit, 0, 0);
    g.clearRect(0, 0, size, size);
    const k = R / this.radarRange;

    // The sweep itself: a disc, range rings, and the wedge the walker is looking into.
    g.save();
    g.beginPath();
    g.arc(c, c, R, 0, Math.PI * 2);
    g.fillStyle = v('--surface');
    g.globalAlpha = 0.84;
    g.fill();
    g.globalAlpha = 1;
    g.clip();
    // The wedge is what the walker can actually see: the horizontal field of view,
    // which is the vertical one widened by the aspect ratio.
    const cam = this.scene.walkCamera;
    const half = Math.atan(Math.tan((cam.fov * Math.PI / 180) / 2) * (cam.aspect || 1.6));
    g.beginPath();
    g.moveTo(c, c);
    g.arc(c, c, R, -Math.PI / 2 - half, -Math.PI / 2 + half);
    g.closePath();
    g.fillStyle = v('--grid');
    g.globalAlpha = 0.75;
    g.fill();
    g.globalAlpha = 1;
    g.restore();
    g.strokeStyle = v('--border');
    g.lineWidth = 1;
    for (const r of [R, R * 0.66, R * 0.33]) {
      g.beginPath();
      g.arc(c, c, r, 0, Math.PI * 2);
      g.stroke();
    }

    // Everything is placed in the walker's frame: forward is up.
    const sin = Math.sin(yaw), cos = Math.cos(yaw);
    const place = (x, z) => {
      const dx = x - px, dz = z - pz;
      const u = dx * cos - dz * sin, f = -dx * sin - dz * cos;
      const d = Math.hypot(u, f);
      const inside = d * k <= R - 4;
      const scale = inside ? k : (R - 4) / Math.max(d, 1e-6);
      return { x: c + u * scale, y: c - f * scale, d, inside, angle: Math.atan2(u, f) };
    };

    // The severity colors, for the bugs and for the rings around what is tagged.
    const colors = this.bugs?.colors || {};

    // North, so the sweep can be read against the map it came from.
    const n = place(px, pz - this.radarRange * 4);
    g.fillStyle = v('--muted');
    g.font = '600 9px system-ui, sans-serif';
    g.textAlign = 'center';
    g.textBaseline = 'middle';
    g.fillText('N', n.x, n.y);

    // Modules already tagged: where the hunt has been, each ring in the color of the
    // worst thing found on it, the way its beacon is.
    for (const b of this.boxes) {
      if (b.kind === 'land' || !this.tagged.has(b.node.id)) continue;
      const q = place(b.x, b.z);
      if (!q.inside) continue;
      g.strokeStyle = colors[this.severityOf(b.node)] || v('--muted');
      g.beginPath();
      g.arc(q.x, q.y, 2.6, 0, Math.PI * 2);
      g.stroke();
    }

    // The bugs, worst drawn last so a critical one is never hidden under a nit.
    live.sort((a, b) => severityRank(a.f.severity) - severityRank(b.f.severity));
    let nearest = null;
    for (const bug of live) {
      const m = bug.pos;
      const q = place(m.x, m.z);
      if (!nearest || q.d < nearest.d) nearest = { d: q.d, bug };
      g.fillStyle = colors[bug.f.severity] || v('--muted');
      if (q.inside) {
        g.beginPath();
        g.arc(q.x, q.y, 3, 0, Math.PI * 2);
        g.fill();
      } else {
        // Out of range: an arrow on the rim, pointing the way.
        g.save();
        g.translate(q.x, q.y);
        g.rotate(q.angle);
        g.beginPath();
        g.moveTo(0, -4);
        g.lineTo(3, 3);
        g.lineTo(-3, 3);
        g.closePath();
        g.fill();
        g.restore();
      }
    }

    // The fires, last of all and over everything else on the sweep.
    //
    // They pulse, and nothing else on the dial does. A bug is a still dot and a tagged
    // module a still ring, because neither is going anywhere: a bug walks its lap and
    // waits to be caught, and a module stays tagged. A fire is the one mark here that
    // is getting worse while it is being looked at, and the one worth turning round
    // for - so it is the one that moves. What pulses is a ring thrown off the dot and
    // fading as it widens, which is a thing spreading, drawn small.
    for (const f of alight) {
      const q = place(f.x, f.z);
      const beat = ((now % RADAR_PULSE) / RADAR_PULSE + f.x * 0.11 + f.z * 0.07) % 1;
      g.fillStyle = FIRE_DOT;
      g.strokeStyle = FIRE_DOT;
      if (q.inside) {
        // The ring first, so the dot it comes off stays solid over it.
        g.save();
        g.globalAlpha = (1 - beat) * 0.7 * (0.4 + f.heat * 0.6);
        g.lineWidth = 1.5;
        g.beginPath();
        g.arc(q.x, q.y, 3 + beat * 7, 0, Math.PI * 2);
        g.stroke();
        g.restore();
        g.beginPath();
        g.arc(q.x, q.y, 2.6 + f.heat * 1.4, 0, Math.PI * 2);
        g.fill();
      } else {
        // Out of range, a fire gets the same rim arrow a bug does, and the same pulse
        // with it: a district alight across the map is worth walking towards, and
        // saying so is most of what the sweep is for.
        g.save();
        g.translate(q.x, q.y);
        g.rotate(q.angle);
        g.globalAlpha = 0.55 + (1 - beat) * 0.45;
        g.beginPath();
        g.moveTo(0, -5);
        g.lineTo(3.4, 3.4);
        g.lineTo(-3.4, 3.4);
        g.closePath();
        g.fill();
        g.restore();
      }
    }

    // How far the sweep reaches, so a dot's distance can be read off it.
    g.fillStyle = v('--muted');
    g.font = '9px system-ui, sans-serif';
    g.textAlign = 'right';
    g.textBaseline = 'bottom';
    g.fillText(`${Math.round(this.radarRange)}`, size - 2, size - 1);

    // The walker, facing up.
    g.fillStyle = v('--text');
    g.beginPath();
    g.moveTo(c, c - 5);
    g.lineTo(c + 3.6, c + 4);
    g.lineTo(c - 3.6, c + 4);
    g.closePath();
    g.fill();

    // Close in, the nearest bug is marked as well as drawn: a ring around it, so the
    // one being walked up to is not one dot among several.
    if (nearest && closing > 0.25) {
      const q = place(nearest.bug.pos.x, nearest.bug.pos.z);
      if (q.inside) {
        g.strokeStyle = colors[nearest.bug.f.severity] || v('--text');
        g.lineWidth = 1.5;
        g.beginPath();
        g.arc(q.x, q.y, 6.5, 0, Math.PI * 2);
        g.stroke();
      }
    }

    const label = this.hud.querySelector('.w-nearest');
    const { caught, total } = this.bugs?.counts || { caught: 0, total: 0 };
    // What is alight comes first, whatever else the sweep is showing: it is the only
    // thing on there that gets worse for being left.
    const fire = alight.map(f => ({ f, d: Math.hypot(f.x - px, f.z - pz) }))
      .sort((a, b) => a.d - b.d)[0];
    label.textContent = fire
      ? `${alight.length} alight · nearest ${Math.round(fire.d)} away · ${fire.f.node.name}`
      : nearest
        ? `nearest ${Math.round(nearest.d)} away · ${nearest.bug.f.severity}: ${nearest.bug.f.title}`
        : `all ${total} bugs caught`;
    if (!fire && !nearest && !caught) label.textContent = '';
  }

  // Names what the crosshair is on while it is a bug, so it is clear what would be
  // caught before the tool is used - or says that it is out of the tool's reach,
  // which is the one case where the crosshair is on something and nothing happens.
  // A swung tool has to be walked up to; a thrown one would simply fall short, and
  // saying which it is saves the walker guessing why the shot did nothing.
  // Implements: REQ-HUNT-014
  showTarget(bug, far = false) {
    const el = this.hud.querySelector('.w-target');
    this.hud.classList.toggle('far', !!far);
    if (far) {
      const name = this.primary.label.toLowerCase();
      this.targeted = null;
      el.hidden = false;
      el.textContent = isMelee(this.primary)
        ? `Out of reach - walk closer to use the ${name}`
        : `Too far - the ${name} will fall short`;
      return;
    }
    if (!bug) {
      el.hidden = true;
      el.textContent = '';
      this.targeted = null;
      return;
    }
    if (this.targeted === bug) return;
    this.targeted = bug;
    el.hidden = false;
    el.replaceChildren(
      Object.assign(document.createElement('i'), { className: `sev-dot sev-${bug.f.severity}` }),
      document.createTextNode(`${bug.f.severity}: ${bug.f.title}`));
  }

  // A photograph has no flight to watch, so the screen says it happened.
  screenFlash() {
    const el = this.hud.querySelector('.crosshair');
    el.classList.remove('flash');
    void el.offsetWidth; // restart the animation
    el.classList.add('flash');
  }

  // Implements: REQ-HUNT-001, REQ-HUNT-017, REQ-HUNT-018, REQ-TOOL-004, REQ-TOOL-027
  updateDarts(dt) {
    const done = [], prev = new THREE.Vector3(), dir = new THREE.Vector3();
    for (const dart of this.darts) {
      dart.t += dt;
      const m = dart.mesh;
      prev.copy(m.position);
      if (dart.bug || dart.target) {
        // A bug walks on while the cast is in the air, so the shot follows it.
        if (dart.bug && !dart.bug.caught) dart.to.copy(dart.bug.pos);
        const u = Math.min(1, dart.t / dart.T);
        m.position.lerpVectors(dart.start, dart.to, u);
        m.position.y += dart.arc * 4 * u * (1 - u);
        if (u >= 1) {
          done.push(dart);
          // A rod both lands the cast and hauls on it; a grapple only hauls.
          if (!dart.tool.climbs) {
            if (dart.bug) this.bugs.catch(dart.bug, dart.tool.catchAs);
            else if (dart.target) this.tag(dart.target);
          }
          // A line hauls on a wall, not on a beetle: what the rod caught comes back
          // on the line, and the walker stays where they are.
          if (dart.tool.reel && !dart.bug && this.hook(m.position, dart.target, dart)) dart.kept = true;
        }
      } else {
        // A miss flies on under the tool's own physics: a dart drops like a dart, a
        // bubble slows to a crawl and then climbs.
        const { gravity, drag, track } = dart.flight || DEFAULT_FLIGHT;
        dart.vel.y -= gravity * dt;
        if (drag) dart.vel.multiplyScalar(Math.max(0, 1 - drag * dt));
        // A tracking dart earns the name on a miss: its fins pull it round towards
        // whatever wall lies ahead of it, so a shot lobbed over a block still finds
        // one. Nothing else here steers, which is the whole of the difference between
        // it and a nail.
        if (track) this.steer(dart, track * dt);
        m.position.addScaledVector(dart.vel, dt);
        // Anything thrown catches a bug it passes through, aimed at or not - if it is
        // the kind of thing that catches bugs at all.
        const bug = hits(dart.tool, 'bugs') ? this.bugs?.at(m.position) : null;
        if (bug) this.bugs.catch(bug, dart.tool.catchAs);
        const hit = this.boxAt(m.position);
        if (hit && !dart.tool.climbs && hits(dart.tool, 'buildings')
          && hit.kind !== 'land' && hit.kind !== 'terrace') this.tag(hit);
        // A grapple bites anything solid, the ground included: that is what makes a
        // shot off a roof a way down rather than a wasted line. A rod does not - a
        // cast that falls short lands on the pavement, and a line that hauls the
        // walker a step across their own street is not worth having.
        const ground = hit && (hit.kind === 'land' || hit.kind === 'terrace');
        if (hit && dart.tool.reel && (dart.tool.climbs || !ground)
          && this.hook(m.position, hit, dart)) dart.kept = true;
        // A shot that hits nothing still has a range: what a tool reaches is what it
        // throws that far, and a nail that sails on over the next six blocks made
        // the reticle's own "too far" a lie. A line is shorter still - fired into
        // the sky or out over the water a hook finds nothing to stop it, and without
        // this it would be six seconds of a rope across the view, going nowhere.
        const gone = dart.from ? m.position.distanceTo(dart.from) : 0;
        // A tool that pays out a line ends where the line does, and says so; for
        // everything else the end is the tool's reach.
        const rope = dart.tool.reel && dart.from && gone > dart.tool.reel.max;
        const spent = !dart.tool.reel && dart.from && gone > (dart.tool.reach ?? REACH);
        if (rope) this.flash('The line ran out');
        if (bug || hit || rope || spent || m.position.y < WATER || dart.t > 6) done.push(dart);
      }
      // A dart points along its flight; a hoop spins, a bubble wobbles, a bobber
      // just bobs along.
      if (m.userData.aim && dir.subVectors(m.position, prev).lengthSq() > 1e-10) {
        m.quaternion.setFromUnitVectors(FORWARD, dir.normalize());
      }
      if (m.userData.spin) m.rotation.z += m.userData.spin * dt;
      if (m.userData.wobble) m.scale.set(1 + Math.sin(dart.t * 9) * 0.07, 1 - Math.sin(dart.t * 9) * 0.07, 1);
      // A cloud opens out as it goes: what left the horn as a gout is a fog by the
      // time it is across the street, which is why the extinguisher is forgiving up
      // close and no use at all past that.
      if (m.userData.swell) m.scale.setScalar(1 + dart.t * m.userData.swell);
      if (dart.line) { // keep the line between the hand it left and what was cast
        const tip = this.muzzle(dart.hand) || new THREE.Vector3(this.p.x, this.p.feet + EYE - 0.05, this.p.z);
        dart.line.geometry.setFromPoints([tip, m.position.clone()]);
      }
    }
    for (const dart of done) {
      // A line that bit keeps its hook and its rope: they are what the walker is
      // being pulled along, and cutLine is what takes them off the map.
      if (!dart.kept) {
        this.scene.scene.remove(dart.mesh);
        if (dart.line) this.scene.scene.remove(dart.line);
      }
      this.darts.splice(this.darts.indexOf(dart), 1);
    }
  }

  /**
   * Turns a shot in flight towards the wall it has picked, by at most `by` radians.
   *
   * It picks one on the way out of the muzzle and holds it: a dart that chose again
   * every frame would swing from building to building as it passed them, and it would
   * cost a sweep of the layout a frame to do it. Nothing is picked twice, and a shot
   * that leaves with nothing ahead of it stays a shot that misses.
   *
   * Implements: REQ-TOOL-027, REQ-TOOL-041
   */
  steer(dart, by) {
    const at = dart.mesh.position;
    const going = (this.aimAt ||= new THREE.Vector3()).copy(dart.vel).normalize();
    if (dart.lock === undefined) dart.lock = this.wallAhead(at, going);
    if (!dart.lock) return;
    const b = dart.lock;
    const want = (this.aimTo ||= new THREE.Vector3())
      .set(b.x - at.x, b.y + b.h / 2 - at.y, b.z - at.z);
    if (want.lengthSq() < 1e-6) return;
    const speed = dart.vel.length();
    dart.vel.copy(going.lerp(want.normalize(), Math.min(1, by)).normalize()).multiplyScalar(speed);
  }

  /**
   * The nearest box a dart could tag that lies within TRACK_AHEAD of where it is
   * going and TRACK_REACH of where it is. The ground and the blocks are not it: a dart
   * that steered into the street would never reach anything.
   */
  wallAhead(at, going) {
    let best = null, nearest = TRACK_REACH;
    for (const b of this.boxes) {
      if (b.kind === 'land' || b.kind === 'terrace') continue;
      const dx = b.x - at.x, dy = b.y + b.h / 2 - at.y, dz = b.z - at.z;
      const d = Math.hypot(dx, dy, dz);
      if (d >= nearest || d < 0.2) continue;
      if ((dx * going.x + dy * going.y + dz * going.z) / d < TRACK_AHEAD) continue;
      nearest = d;
      best = b;
    }
    return best;
  }

  // The hunt takes two shots: the first dart tags the module, a second one into the
  // same building asks what it is - the details, without letting go of the trigger.
  // Implements: REQ-HUNT-001, REQ-HUNT-002, REQ-HUNT-005
  tag(box) {
    if (this.tagged.has(box.node.id)) {
      this.hooks.onInspect(box);
      return;
    }
    this.tagged.add(box.node.id);
    this.drawBeacons();
    this.drawHud();
    this.hooks.onHit(box, this.tagged.size);
  }

  /**
   * The worst thing the scanners said about a module, or null where they said nothing.
   * The map is what knows; the walker only asks, so that walk mode works the same with
   * findings turned off as with them on.
   *
   * Implements: REQ-HUNT-004
   */
  severityOf(node) {
    return this.hooks.severityOf?.(node) || null;
  }

  /**
   * A beacon stands over every tagged module that has a box: a diamond on a thin light
   * beam, so the hunt's trophies are visible across the city - in the color of the
   * worst finding on it, so what the beam says is not only that the module was tagged
   * but how bad what is in it is. A module the scanners had nothing to say about keeps
   * the plain beacon, which is what a clean module looks like from across the map.
   *
   * Implements: REQ-HUNT-003, REQ-HUNT-004
   */
  drawBeacons() {
    this.beacons.clear();
    if (!this.tagged.size) return;
    beaconParts ||= [
      new THREE.OctahedronGeometry(0.16).scale(1, 1.6, 1).translate(0, 1.75, 0),
      new THREE.CylinderGeometry(0.018, 0.018, 1.5, 6).translate(0, 0.75, 0),
    ];
    const colors = severityColors();
    for (const b of this.boxes) {
      if (b.kind === 'land' || !this.tagged.has(b.node.id)) continue;
      const color = colors[this.severityOf(b.node)] || BEACON;
      for (const geo of beaconParts) {
        const m = new THREE.Mesh(geo, this.beaconMaterial(color));
        m.position.set(b.x, b.y + b.h, b.z);
        m.frustumCulled = false;
        this.beacons.add(m);
      }
    }
  }

  // One material per color, however many beacons there are: seven at the very most,
  // and they are rebuilt every time something is tagged.
  beaconMaterial(color) {
    const beams = this.beams ||= new Map();
    let m = beams.get(color);
    if (!m) {
      m = this.scene.bendable(new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.85 }));
      beams.set(color, m);
    }
    return m;
  }

  // Implements: REQ-HUNT-002, REQ-HUNT-019
  drawHud() {
    const locked = document.pointerLockElement === this.scene.renderer.domElement;
    // Walking is what walk mode is; saying so is a chip that never changes. Flying
    // and reading are worth a word, and get one.
    const mode = this.frozen ? 'reading'
      : this.showing ? 'looking'
        : this.p.fly ? 'flying'
          : this.sinking ? 'sinking'
            : this.onWater() ? 'afloat' : '';
    const modeChip = this.hud.querySelector('.w-mode');
    modeChip.hidden = !mode;
    modeChip.textContent = mode;
    this.hud.querySelector('.w-tagged').textContent = this.tagged.size;
    this.hud.querySelector('.w-radius').textContent = Math.round(this.radius);
    const bugs = this.bugs?.counts;
    const counter = this.hud.querySelector('.w-bugs');
    counter.hidden = !bugs?.total;
    if (bugs?.total) {
      this.hud.querySelector('.w-caught').textContent = bugs.caught;
      this.hud.querySelector('.w-total').textContent = bugs.total;
    }

    // One line, and only when it has something to say that the slots and the
    // reticle do not: how to get the mouse back, or what reading means.
    this.hud.querySelector('.w-hint').textContent = this.frozen
      ? 'Holding still · click the map to walk on'
      : locked ? ''
        : this.noLock ? 'Drag to look · click: use the tool'
          : 'Click the map to capture the mouse, or drag to look';
  }
}

const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

// Implements: REQ-TOOL-027, REQ-TOOL-042
/** Knocks a point or a direction off course by up to `by`, evenly in all directions. */
export function scatter(v, by) {
  v.x += (Math.random() * 2 - 1) * by;
  v.y += (Math.random() * 2 - 1) * by;
  v.z += (Math.random() * 2 - 1) * by;
}

/**
 * Exponential easing towards a value: `tau` is how long it takes to close most of
 * the gap, in seconds, whatever dt happens to be. An undefined current value starts
 * where it is going, so nothing animates in from zero on the first frame.
 */
const ease = (cur, want, tau, dt) =>
  cur === undefined ? want : cur + (want - cur) * (1 - Math.exp(-Math.max(0, dt) / tau));

// The walker's footprint, sampled at its centre and four corners (height). Flat
// pairs, so walking it allocates nothing.
const PROBES = [0, 0, BODY, BODY, BODY, -BODY, -BODY, BODY, -BODY, -BODY];

// The tracker draws the worst bugs last, so a critical one is never hidden under a nit.
const SEVERITY_ORDER = ['unknown', 'info', 'low', 'medium', 'high', 'critical'];
const severityRank = s => SEVERITY_ORDER.indexOf(s);

const FORWARD = new THREE.Vector3(0, 0, 1); // the dart geometry's nose
const EYE_AT = new THREE.Vector3();        // where the walker is, handed to the bugs
const HOOP_AT = new THREE.Vector3();       // ... and where the net's hoop is, likewise
// What a beacon is over a module the scanners had nothing to say about; anything they
// did have something to say about wears the color of the worst of it instead.
const BEACON = '#ff8a1f';
let beaconParts = null; // the diamond and its beam, shared by every beacon

// A tracking dart: a dark shaft, a glowing tip and two crossed fins, nose along +z.
// Its materials bend like the map's.
