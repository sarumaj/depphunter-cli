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
// a wall, a jet to fly on, floats to cross the bay with. Which one is in hand is what
// decides whether the walker flies or the water holds them up, so putting one away is
// how you come down or get wet.
//
// The other quarry is real: every finding a scanner reported walks the streets as a
// bug (bugs.js), and catching one reads out what was said about it. They bite back,
// and a roof is a long way down, so the walker has a condition to keep (health.js).

import * as THREE from './vendor/three.module.min.js';
import { rampsFor, rampHeight, bridgesFor, bridgeHeight, bridgeBounds } from './city.js';
import { Health } from './health.js';
import { TOOL_IDS, DEFAULT_TOOL, toolFor, idleTool, restTool, viewLights, hits, marks, isMelee } from './tools.js';

// A building is one unit wide and its storeys 0.3 high (city.js): the walker is
// about a storey and a half tall.
const EYE = 0.45;           // eye height above the feet
const WALK = 2.6, RUN = 7, FLY = 10; // units per second
const JUMP = 4.4, GRAVITY = 13;
const STEP = 0.5;           // highest ledge walked up without jumping
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
const RADAR_MIN = 12, RADAR_MAX = 400, RADAR_MS = 85, RADAR_SIZE = 150;
// Closing in: inside this, the sweep stops trying to hold the whole map and draws the
// neighborhood instead, growing by up to RADAR_GROW as it does. The last few steps
// to a bug are the ones worth seeing, and they are the ones a whole-map sweep loses.
// 14 rather than something roomier because a city block is a few units across: any
// wider and a map with bugs on every street is zoomed in the whole time, which is
// the same as not zooming at all.
const RADAR_NEAR = 14, RADAR_GROW = 0.55;
// How long the range and the dial take to settle, in seconds. Eased by elapsed time
// rather than per redraw, so the growth is the same on a slow frame as on a fast one.
const RADAR_RANGE_TAU = 0.5, RADAR_ZOOM_TAU = 0.55;
// Degrees: the default view, the wheel's zoom range, and the view through the scope
// (right button).
const FOV = 70, MIN_FOV = 30, MAX_FOV = 90, SCOPE_FOV = 22;
// How far in the held tool sits, as a fraction of where it is modelled. See showTool.
const VIEW_NEAR = 0.5;
const SWING = 0.45;         // seconds a tool takes to swing and settle
// How far the walker may leave the map: over the water beyond the outermost shore,
// and above its tallest building when flying.
const SHORE_MARGIN = 3, SKY_MARGIN = 12;
// A burst on the jet backpack: how long it lasts and how much faster it goes.
const BURST = 0.9, BURST_SPEED = 3;
// Being bitten: how near a bug has to be to reach the walker, and how often it can.
// The reach is a stride, so standing in the middle of a lap is what does it rather
// than walking past one; a bug on a wall three storeys up cannot reach anybody.
const BITE_REACH = 0.75, BITE_EVERY = 1.1;
// How far a dart looks for a wall to steer towards, and how nearly ahead of itself it
// will accept one, as a cosine: about forty degrees either side, which is wide enough
// to save a lobbed shot and narrow enough that a dart cannot turn round.
const TRACK_REACH = 30, TRACK_AHEAD = 0.75;

// Keys the walker owns while active, by KeyboardEvent.code; the map's own shortcuts
// for these letters are suspended. A key that is not here never reaches walk mode -
// the map keeps it - so this has to list every one the handler below acts on. E and Q
// are held and do nothing: on the map they rotate the view and expand things, which
// would only reshuffle the city around a walker.
const KEYS = new Set([
  'KeyW', 'KeyA', 'KeyS', 'KeyD', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight',
  'Space', 'ShiftLeft', 'ShiftRight', 'KeyC', 'KeyE', 'KeyQ', 'Enter',
  'Escape', 'KeyV', 'KeyM', 'KeyT', 'KeyH',
  // The tool slots: 1 to 9, and 0 for the tenth, the way a shooter numbers them.
  'Digit0', ...Array.from({ length: 9 }, (_, i) => `Digit${i + 1}`),
]);

// Which slot a number key picks: 1..9 in order, and 0 last.
const slotOf = code => (code === 'Digit0' ? 9 : +code.slice(5) - 1);

// Planet curvature, by the character typed rather than the key's place on the board:
// on a German keyboard the key at BracketRight types '+', which would otherwise make
// '+' curve the planet while '-' still changed the map's depth.
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
    this.aim = { i: -1, point: null, bug: null };
    this.bugs = null; // set by setBugs once there are findings to walk the streets
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
    this.tool = toolFor(hooks.tool?.() || DEFAULT_TOOL);
    this.health = new Health(hud);
    this.bitAt = 0;   // when a bug last got a bite in
    this.burst = 0;   // seconds of jet backpack thrust left
    this.fell = null; // the height a fall in progress started from
    this.viewmodel = null;      // the hand and its tool
    this.held = null;           // what holds them in front of the walk camera
    this.props = null;          // the prop obstacle list this grid was built from
    this.propGrid = new Map();  // cell -> the props standing in it
    this.swing = -1;            // seconds into the current gesture, -1 when idle
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

  /** Hands the walker the bugs patrolling the map (bugs.js); null takes them away. */
  setBugs(bugs) {
    this.bugs = bugs;
    bugs?.show(this.active);
    if (this.active) this.drawHud();
  }

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
   * planet.
   */
  enter(box, block, bounds) {
    const diag = Math.hypot(bounds.maxX - bounds.minX, bounds.maxZ - bounds.minZ);
    this.radius = clamp(diag * 0.6, 15, Math.min(600, this.maxRadius()));
    this.active = true;
    this.hud.hidden = false;
    this.scene.setWalking(true, this.radius);
    this.scene.scene.add(this.beacons);
    this.bugs?.show(true);
    this.showTool();
    this.setFog();
    this.p.fly = !!this.tool.flies;
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
    this.burst = 0;
    this.fell = null;
    // The tracker starts where it belongs rather than easing in from wherever it was
    // left the last time walk mode was entered, possibly half a map away.
    this.radarRange = this.radarZoom = undefined;
    this.radarAt = 0;
    this.drawSlots();
    this.drawHud();
    this.loop();
    // Like a first-person shooter: the pointer is captured at the reticle at once (V
    // and the Walk button are user gestures, which browsers require for this).
    this.lockPointer();
  }

  exit() {
    if (!this.active) return;
    // Where they stood, so coming back is coming back rather than starting again.
    this.home = this.stance();
    this.active = false;
    this.keys.clear();
    for (const dart of this.darts) this.scene.scene.remove(dart.mesh);
    this.darts = [];
    this.cutLine();
    this.scene.scene.remove(this.beacons);
    this.bugs?.show(false);
    this.showTarget(null);
    this.hideTool();
    if (document.pointerLockElement) document.exitPointerLock();
    cancelAnimationFrame(this.frame);
    this.hud.hidden = true;
    this.hud.classList.remove('compact');
    this.hud.classList.remove('hit');
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
   */
  setHandsOff(off) {
    this.handsOff = off;
    if (!this.active) return;
    if (off) this.hideTool(); else this.showTool();
    this.flash(off ? 'Empty-handed - H takes the tool out again' : `${this.tool.label} back in hand`);
    this.drawHud();
  }

  showTool() {
    this.hideTool();
    // The reticle is the tool's, not the tool's name: several tools share one, and
    // style.css keys the crosshair off it.
    this.hud.dataset.tool = this.tool.reticle || 'scope';
    if (this.handsOff) return;
    // A camera draws its children only when it is itself part of a scene, and this
    // one belongs to the pass that draws the tool over the world (MapScene.renderNow).
    this.scene.viewScene.add(this.scene.walkCamera);
    // The only lights in the scene, and they travel with the view: everything on the
    // map is drawn with unlit materials, so they reach nothing but what is held.
    this.lights ||= viewLights();
    this.scene.walkCamera.add(this.lights);
    this.viewmodel = this.tool.viewmodel();
    this.viewmodel.userData.restY = this.viewmodel.position.y;
    // Held near the lens rather than out in the street. A viewmodel is drawn in the
    // same pass as the map, so at arm's length it is half a metre off the ground and
    // the pavement is drawn straight through it; brought in and scaled down by the
    // same amount, the picture is identical and nothing can reach it.
    this.held = new THREE.Group();
    this.held.scale.setScalar(VIEW_NEAR);
    this.held.add(this.viewmodel);
    this.scene.walkCamera.add(this.held);
  }

  hideTool() {
    if (this.lights) this.scene.walkCamera.remove(this.lights);
    if (this.held) {
      this.scene.walkCamera.remove(this.held);
      this.held = null;
      this.viewmodel = null;
    }
    this.scene.viewScene.remove(this.scene.walkCamera);
  }

  /** Take another tool out: same hunt, different gesture. */
  setTool(id) {
    this.tool = toolFor(id);
    this.swing = -1;
    // Flight is a thing you are carrying, not a mode you are in: putting the jet
    // backpack away is how you come down.
    this.p.fly = !!this.tool.flies;
    if (!this.p.fly) this.burst = 0;
    if (this.active) {
      this.setFog();
      this.showTool();
      this.drawSlots();
      this.drawHud();
      this.flash(this.tool.hint);
      this.aim = { i: -1, point: null, bug: null, far: false }; // it may want another target
    }
    this.hooks.onTool?.(this.tool.id);
  }

  /**
   * The row of tools along the bottom, built once: a slot per tool, in slot order,
   * carrying the number that picks it. Only the one in hand is named, so the row
   * stays a row of shapes rather than a sentence.
   *
   * The two kinds are kept apart, with a gap and a mark between them: what a primary
   * tool does to the map and what a secondary one does to you are different enough
   * that reaching for the wrong one should look like a mistake before it is made.
   */
  drawSlots() {
    const row = this.hud.querySelector('.w-slots');
    if (row.querySelectorAll('.w-slot').length !== TOOL_IDS.length) {
      const slots = [];
      TOOL_IDS.forEach((id, n) => {
        const tool = toolFor(id);
        if (n && toolFor(TOOL_IDS[n - 1]).kind !== tool.kind) {
          const gap = document.createElement('span');
          gap.className = 'w-slot-gap';
          gap.textContent = 'carried';
          gap.setAttribute('aria-hidden', 'true'); // a listbox's children are its options
          slots.push(gap);
        }
        const el = document.createElement('div');
        el.className = 'w-slot';
        el.dataset.tool = id;
        el.dataset.kind = tool.kind;
        el.setAttribute('role', 'option');
        el.title = `${tool.label} - ${tool.hint}`;
        el.append(
          // Ten slots, so the tenth is the 0 key: what a shooter does, and what the
          // keyboard leaves room for.
          Object.assign(document.createElement('kbd'), { textContent: String((n + 1) % 10) }),
          document.createElement('i'),
          Object.assign(document.createElement('span'), { className: 'w-slot-name', textContent: tool.label }),
        );
        slots.push(el);
      });
      row.replaceChildren(...slots);
    }
    for (const el of row.querySelectorAll('.w-slot')) {
      el.setAttribute('aria-selected', String(el.dataset.tool === this.tool.id));
    }
  }

  nextTool() {
    const ids = TOOL_IDS;
    this.setTool(ids[(ids.indexOf(this.tool.id) + 1) % ids.length]);
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

  // The tool breathes while it waits and swings while it is used; the swing is what
  // makes the gesture legible, so it runs to its end even if the shot lands sooner.
  poseTool(dt, now) {
    const vm = this.viewmodel;
    if (!vm) return;
    // A tool with something live on it gets its frame here. Every other frame is
    // enough for a screen this size, and halves what it costs.
    if (this.tool.live && ((this.frames = (this.frames || 0) + 1) & 1)) {
      this.tool.live(vm, this.scene, this.lens());
    }
    if (this.swing >= 0) {
      this.swing += dt;
      const u = this.swing / SWING;
      if (u >= 1) {
        this.swing = -1;
        this.tool.pose(vm, 1, now); // land the gesture on its own end state
        restTool(vm);
      } else {
        this.tool.pose(vm, u, now);
        return;
      }
    }
    idleTool(vm, now, this.pace);
  }

  /**
   * Hold the view still while something else has the pointer (the details panel), or
   * let it go again. Frozen, the walker does not move, look, aim or fire; the scene
   * keeps rendering, so what is being read about stays on screen.
   */
  setFrozen(on) {
    if (this.frozen === on || (!this.active && on)) return;
    this.frozen = on;
    if (on) {
      this.keys.clear(); // a key held when the panel opened must not walk on
      this.setScoped(false);
      // The aim is not recomputed while frozen, so whatever the crosshair was on
      // would keep its hover card - on top of the panel that is being read.
      this.aim = { i: -1, point: null, bug: null };
      this.showTarget(null);
      this.hooks.onAim(-1);
    }
    this.drawHud();
  }

  // Locking is asynchronous: a lock asked for just before leaving would be granted
  // afterwards and hide the cursor over the map, with nothing listening to it.
  lockPointer() {
    if (!this.active) return;
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
      e.preventDefault();
      e.stopImmediatePropagation();
      if (e.repeat) return;
      if (this.frozen) {
        // Reading: Esc and V still work, Enter goes back to walking, nothing moves.
        switch (e.code) {
          case 'KeyV': case 'KeyM': this.exit(); break;
          case 'Escape':
          case 'Enter': this.setFrozen(false); this.lockPointer(); break;
        }
        return;
      }
      this.keys.add(e.code);
      if (BIGGER.has(e.key)) this.setRadius(this.radius * 1.25);
      else if (SMALLER.has(e.key)) this.setRadius(this.radius / 1.25);
      // 1..9 pick a tool by its slot, the way a shooter does; T still walks the row
      // for anyone who would rather not look down at it.
      if (e.code.startsWith('Digit')) {
        const id = TOOL_IDS[slotOf(e.code)];
        if (id && id !== this.tool.id) this.setTool(id);
        return;
      }
      switch (e.code) {
        case 'KeyT': this.nextTool(); break;
        case 'KeyH': this.setHandsOff(!this.handsOff); break;
        case 'Escape':
          // Browsers usually swallow the Esc that frees the pointer; if not, free it first.
          if (document.pointerLockElement) document.exitPointerLock();
          else this.exit();
          break;
        case 'Enter': this.hooks.onInspect(this.aimed()); break;
        // V is the toggle the map also answers to; M says where it goes, for anyone
        // who reaches for the map by name rather than remembering which way V points.
        case 'KeyV': case 'KeyM': this.exit(); break;
      }
    });
    window.addEventListener('keyup', e => this.keys.delete(e.code));
    window.addEventListener('blur', () => this.keys.clear());

    // The pointer is locked at the reticle while walking (lockPointer): the mouse
    // looks around, the left button fires, holding the right one looks through the
    // scope. Once freed (Esc), or where locking is refused, a left drag looks around
    // and a left click locks it again.
    //
    // Buttons are taken from mouse events, not pointer events: pressing a second
    // button while one is held fires pointermove rather than pointerdown, so firing
    // while scoped would never arrive.
    let fresh = false; // the first movement after locking can carry a bogus jump
    canvas.addEventListener('mousedown', e => {
      if (!this.active) return;
      if (e.button === 2) {
        this.setScoped(true);
        return;
      }
      if (e.button !== 0) return;
      if (document.pointerLockElement === canvas) this.fire();
      else drag = { x: e.clientX, y: e.clientY, moved: false };
    });
    window.addEventListener('mouseup', e => {
      if (e.button === 2) this.setScoped(false);
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
    window.addEventListener('pointermove', e => {
      if (!this.active) return;
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
    canvas.addEventListener('wheel', e => {
      if (!this.active || this.frozen) return;
      e.preventDefault();
      this.fov = clamp(this.fov * Math.exp(e.deltaY * 0.001), MIN_FOV, MAX_FOV); // zoom
    }, { passive: false });
    document.addEventListener('pointerlockerror', () => this.active && this.refused(null));
    document.addEventListener('pointerlockchange', () => {
      fresh = document.pointerLockElement === canvas;
      if (fresh) this.lockFails = 0; // it can be had here; earlier refusals were passing
      if (fresh && !this.active) document.exitPointerLock(); // never keep the map's cursor hidden
      if (this.active) this.drawHud();
    });
  }

  look(dx, dy) {
    if (this.frozen) return;
    const k = LOOK * this.scene.walkCamera.fov / FOV; // steadier through the scope
    this.p.yaw -= dx * k;
    this.p.pitch = clamp(this.p.pitch - dy * k, -1.5, 1.5);
  }

  // The planet can grow until the map looks flat, not beyond.
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
  setFog() {
    const far = Math.max(60, this.radius * 2.5) + 6 * Math.max(0, this.p.feet);
    Object.assign(this.scene.scene.fog, { near: far * 0.3, far });
  }

  // ------------------------------------------------------------------ simulation

  loop() {
    this.frame = requestAnimationFrame(() => {
      if (!this.active) return;
      const now = performance.now();
      const dt = Math.min(0.05, (now - this.last) / 1000);
      this.last = now;
      this.step(dt);
      if (this.p.fly) this.setFog();
      this.zoom(dt);
      this.updateDarts(dt);
      this.bugs?.update(dt, now);
      this.drawRadar(now, dt);
      this.health.draw(now); // the wash a hit leaves has to come off by itself
      this.poseTool(dt, now);
      this.scene.setWalker(this.p.x, this.p.feet, this.p.z, EYE, this.p.yaw, this.p.pitch);
      if (!this.frozen) this.updateAim();
      this.scene.renderNow();
      this.hooks.onRender();
      this.loop();
    });
  }

  // Eases the field of view towards the scope's or the normal one.
  zoom(dt) {
    const cam = this.scene.walkCamera, target = this.scoped ? Math.min(SCOPE_FOV, this.fov) : this.fov;
    if (Math.abs(cam.fov - target) < 0.05) return;
    cam.fov += (target - cam.fov) * Math.min(1, dt * 14);
    cam.updateProjectionMatrix();
  }

  step(dt) {
    if (this.frozen) return;
    const k = this.keys, p = this.p;
    // A line in a wall pulls the walker along it, past walls and gravity both, and
    // nothing else moves them until it lets go. Jump cuts it.
    if (this.pull) {
      if (k.has('Space')) this.cutLine('Line cut');
      else return this.reel(dt);
    }
    const turn = (k.has('ArrowLeft') ? 1 : 0) - (k.has('ArrowRight') ? 1 : 0);
    p.yaw += turn * TURN * dt;
    const fwd = (k.has('KeyW') || k.has('ArrowUp') ? 1 : 0) - (k.has('KeyS') || k.has('ArrowDown') ? 1 : 0);
    const side = (k.has('KeyD') ? 1 : 0) - (k.has('KeyA') ? 1 : 0);
    const run = k.has('ShiftLeft') || k.has('ShiftRight');
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

    // Axis by axis, so the walker slides along walls.
    // On foot, the shore is the end of the world: water stops the walker (unless they
    // are already in it, say after landing there) - or it does not, because the water
    // skimmers are what is in their hands, and then the bay is a street.
    const climb = p.feet + (p.ground || p.fly ? STEP : 0.05);
    const wet = !p.fly && this.height(p.x, p.z) <= WATER;
    const afloat = !p.fly && !!this.tool.floats;
    const ok = h => h <= climb && (p.fly || wet || afloat || h > WATER);
    const nx = p.x + mx * speed * dt;
    if (ok(this.height(nx, p.z))) p.x = nx;
    const nz = p.z + mz * speed * dt;
    if (ok(this.height(p.x, nz))) p.z = nz;
    // How hard the walker is moving, eased: the tool in their hands sways with it.
    const effort = len > 0 ? (p.fly ? 0.3 : run ? 1.5 : 1) : 0;
    this.pace += (effort - this.pace) * Math.min(1, dt * 7);
    // The key list folds away while moving and comes back after a pause.
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
    const floor = this.height(p.x, p.z);
    if (p.fly) {
      const ceiling = (this.limits?.maxY ?? 0) + SKY_MARGIN;
      p.feet = Math.max(floor, Math.min(ceiling, p.feet + my * speed * dt));
      p.vy = 0;
      p.ground = p.feet <= floor;
      this.fell = null;
      return;
    }
    if (k.has('Space') && p.ground) p.vy = JUMP;
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
  }

  /** The end of a fall: what it was worth, and whether it was the end of the walk. */
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
   * The walk is over. It ends the way leaving on foot does - back to the map, standing
   * where you fell - because the backpack is what a session is for and nothing in it
   * is lost. Coming back in is coming back at full health (enter).
   */
  die(cause) {
    this.flash(`${cause} finished you. Back to the map; walk in again to start over`);
    this.exit();
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

  /** Keeps the walker over the map or the water just off its shores. */
  confine() {
    const l = this.limits, p = this.p;
    if (!l) return;
    p.x = clamp(p.x, l.minX - SHORE_MARGIN, l.maxX + SHORE_MARGIN);
    p.z = clamp(p.z, l.minZ - SHORE_MARGIN, l.maxZ + SHORE_MARGIN);
  }

  /**
   * Top of the solid column under a body at (x, z): the highest box, ramp or bridge
   * deck it overlaps.
   */
  height(x, z) {
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
    for (const at of this.spans.get(cellKey(x, z)) || NO_CELL) top = Math.max(top, at(x, z));
    return top;
  }

  /**
   * The boxes indexed in the cell holding (x, z) - candidates, not answers: a cell is
   * larger than a box, so the caller tests the footprint.
   *
   * The test stays with the caller rather than here in a generator that yields only
   * what matches: the ray the crosshair marches calls this a few hundred times a
   * frame, and an iterator object per call is that many allocations to throw away.
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

  /** Whether a box is the shore or the block the walker stands in. */
  underfoot(b) {
    const p = this.p;
    return b.kind === 'land' || b.kind === 'terrace' && Math.abs(p.x - b.x) <= b.w / 2 && Math.abs(p.z - b.z) <= b.d / 2;
  }

  // Marches the crosshair ray through the bent view, mapping each sample back to the
  // flat map, until it enters a box or the water.
  updateAim() {
    const cam = this.scene.walkCamera;
    const dir = new THREE.Vector3(0, 0, -1).applyQuaternion(cam.quaternion);
    const v = new THREE.Vector3();
    const tool = this.tool;
    // Both answers are the same at every step of the march, so they are asked once.
    const catches = hits(tool, 'bugs') && this.bugs ? this.bugs : null;
    const tags = marks(tool, 'buildings');
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
    let i = !bug && hit && tags && !this.underfoot(hit) ? hit.i : -1;
    // A tool with a reach is swung, not thrown: past it there is nothing to be done
    // about what the crosshair is on, which the HUD says rather than going blank.
    const far = tool.reach != null && point != null
      && cam.position.distanceTo(point) > tool.reach && (bug || i >= 0);
    if (far) { bug = null; i = -1; }
    this.aim = { i, point, bug, far };
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
   */
  muzzle() {
    const m = this.viewmodel?.getObjectByName('muzzle');
    if (!m) return null;
    m.updateWorldMatrix(true, false);
    return m.getWorldPosition(new THREE.Vector3());
  }

  // Using the tool on an aimed box sends whatever it throws along a shallow arc to
  // the aimed point, and it always arrives; used on nothing it flies ahead under its
  // own gravity and drag until it hits something or falls into the water. A tool that
  // throws nothing reaches what it is pointed at the moment it is used - as far as it
  // reaches, which for the camera is any distance and for the net is arm's length.
  fire() {
    if (this.frozen) return;
    const p = this.p;
    const tool = this.tool;
    this.swing = 0; // the hand moves whether or not anything flies
    const target = this.aimed(), bug = this.aim.bug;
    // A copy: a shot that scatters moves where it is going, and where it is going is
    // the crosshair's own point until the next frame recomputes it.
    const to = bug ? bug.pos.clone() : this.aim.point?.clone() || null;

    if (!tool.projectile) {
      if (tool.flash) this.screenFlash();
      if (tool.flies) {
        // The throttle, wide open for a moment. It is the one use of a tool that
        // changes the walker rather than the map.
        this.burst = BURST;
        this.flash('Thrusters');
      } else if (tool.floats) {
        this.flash(this.height(p.x, p.z) <= WATER ? 'Riding the water' : 'The skimmers want water under them');
      } else if (bug) this.bugs.catch(bug);
      else if (target) this.tag(target);
      else if (this.aim.far) this.flash(`Out of reach: the ${tool.label.toLowerCase()} has to be walked up to`);
      return;
    }
    const start = this.muzzle() || new THREE.Vector3(p.x, p.feet + EYE - 0.08, p.z);
    const mesh = tool.projectile();
    mesh.position.copy(start);
    this.scene.scene.add(mesh);
    const shot = { mesh, t: 0, tool };
    if (tool.line) { // a cast trails its line back to the rod
      const geo = new THREE.BufferGeometry().setFromPoints([start.clone(), start.clone()]);
      shot.line = new THREE.Line(geo, new THREE.LineBasicMaterial({ color: tool.line }));
      shot.line.frustumCulled = false;
      this.scene.scene.add(shot.line);
    }
    const flight = tool.flight || DEFAULT_FLIGHT;
    shot.flight = flight;
    if (bug || target) {
      // A tool that scatters does not land where it was aimed: the nail goes wide by
      // a share of how far it has to travel, which is nothing across a room and the
      // width of a window at the end of its reach.
      const dist = start.distanceTo(to);
      if (flight.spread) scatter(to, flight.spread * dist);
      Object.assign(shot, { start, to, target, bug, T: Math.max(0.12, dist / flight.speed), arc: (0.05 + dist * 0.03) * flight.arc });
    } else {
      const dir = new THREE.Vector3(-Math.sin(p.yaw) * Math.cos(p.pitch), Math.sin(p.pitch) + 0.04, -Math.cos(p.yaw) * Math.cos(p.pitch));
      if (flight.spread) scatter(dir, flight.spread);
      shot.vel = dir.normalize().multiplyScalar(flight.speed);
      // Where it left from, so how far it has carried can be measured against the
      // tool's reach - and against the length of a line, for the ones that pay one out.
      shot.from = start.clone();
    }
    this.darts.push(shot);
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
    this.pull = { to, t: 0, line, mesh: shot.mesh, rope: shot.line };
    this.p.fly = false;
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
      const tip = this.muzzle() || new THREE.Vector3(p.x, p.feet + EYE - 0.05, p.z);
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
  drawRadar(now, dt) {
    const box = this.hud.querySelector('.w-radar');
    if (!this.bugs?.bugs.length) {
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
    const live = this.bugs.bugs.filter(b => !b.caught);
    let far = RADAR_MIN, near = Infinity;
    for (const bug of live) {
      const d = Math.hypot(bug.pos.x - px, bug.pos.z - pz);
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

    // North, so the sweep can be read against the map it came from.
    const n = place(px, pz - this.radarRange * 4);
    g.fillStyle = v('--muted');
    g.font = '600 9px system-ui, sans-serif';
    g.textAlign = 'center';
    g.textBaseline = 'middle';
    g.fillText('N', n.x, n.y);

    // Modules already tagged: where the hunt has been.
    g.strokeStyle = v('--muted');
    for (const b of this.boxes) {
      if (b.kind === 'land' || !this.tagged.has(b.node.id)) continue;
      const q = place(b.x, b.z);
      if (!q.inside) continue;
      g.beginPath();
      g.arc(q.x, q.y, 2.6, 0, Math.PI * 2);
      g.stroke();
    }

    // The bugs, worst drawn last so a critical one is never hidden under a nit.
    const colors = this.bugs.colors;
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
    const { caught, total } = this.bugs.counts;
    label.textContent = nearest
      ? `nearest ${Math.round(nearest.d)} away · ${nearest.bug.f.severity}: ${nearest.bug.f.title}`
      : `all ${total} bugs caught`;
    if (!nearest && !caught) label.textContent = '';
  }

  // Names what the crosshair is on while it is a bug, so it is clear what would be
  // caught before the tool is used - or says that it is out of the tool's reach,
  // which is the one case where the crosshair is on something and nothing happens.
  // A swung tool has to be walked up to; a thrown one would simply fall short, and
  // saying which it is saves the walker guessing why the shot did nothing.
  showTarget(bug, far = false) {
    const el = this.hud.querySelector('.w-target');
    this.hud.classList.toggle('far', !!far);
    if (far) {
      const name = this.tool.label.toLowerCase();
      this.targeted = null;
      el.hidden = false;
      el.textContent = isMelee(this.tool)
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
            if (dart.bug) this.bugs.catch(dart.bug);
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
        if (bug) this.bugs.catch(bug);
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
      if (dart.line) { // keep the line between the rod's tip and what was cast
        const tip = this.muzzle() || new THREE.Vector3(this.p.x, this.p.feet + EYE - 0.05, this.p.z);
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

  // A beacon stands over every tagged module that has a box: a diamond on a thin light
  // beam, so the hunt's trophies are visible across the city.
  drawBeacons() {
    this.beacons.clear();
    if (!this.tagged.size) return;
    beaconParts ||= [
      [new THREE.OctahedronGeometry(0.16).scale(1, 1.6, 1).translate(0, 1.75, 0), BEACON],
      [new THREE.CylinderGeometry(0.018, 0.018, 1.5, 6).translate(0, 0.75, 0), BEACON],
    ].map(([geo, color]) => [geo, this.scene.bendable(new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.85 }))]);
    for (const b of this.boxes) {
      if (b.kind === 'land' || !this.tagged.has(b.node.id)) continue;
      for (const [geo, mat] of beaconParts) {
        const m = new THREE.Mesh(geo, mat);
        m.position.set(b.x, b.y + b.h, b.z);
        m.frustumCulled = false;
        this.beacons.add(m);
      }
    }
  }

  drawHud() {
    const locked = document.pointerLockElement === this.scene.renderer.domElement;
    // Walking is what walk mode is; saying so is a chip that never changes. Flying
    // and reading are worth a word, and get one.
    const mode = this.frozen ? 'reading'
      : this.p.fly ? 'flying'
        : this.tool.floats && this.height(this.p.x, this.p.z) <= WATER ? 'afloat' : '';
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
      ? 'Reading · Enter or a click: walk on · Esc: close'
      : locked ? ''
        : this.noLock ? 'Drag to look · click: use the tool'
          : 'Click the map to capture the mouse, or drag to look';
  }
}

const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

/** Knocks a point or a direction off course by up to `by`, evenly in all directions. */
function scatter(v, by) {
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
const BEACON = '#ff8a1f';
let beaconParts = null;

// A tracking dart: a dark shaft, a glowing tip and two crossed fins, nose along +z.
// Its materials bend like the map's.
