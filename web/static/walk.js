// Walk mode: explore the map on foot, in first person, on a small planet. The walker
// lives on the flat map (layout coordinates) - collisions, heights and darts are all
// computed there - and MapScene bends what is drawn around the walker's feet.
//
// The dependency hunt: fire tracking darts at buildings. A hit tags the module: it is
// selected, so its dependency trails light up, and a beacon marks it for the rest
// of the session.

import * as THREE from './vendor/three.module.min.js';
import { rampsFor, rampHeight, bridgesFor, bridgeHeight } from './city.js';

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
const LOOK = 0.0022;        // radians per pixel of mouse movement
const TURN = 2.2;           // radians per second with the arrow keys
const MIN_R = 6, MAX_R = 2000;
const MAX_LOOK_STEP = 250;  // pixels; larger pointer movements are glitches, not looks
// Degrees: the default view, the wheel's zoom range, and the view through the scope
// (right button).
const FOV = 70, MIN_FOV = 30, MAX_FOV = 90, SCOPE_FOV = 22;
// How far the walker may leave the map: over the water beyond the outermost shore,
// and above its tallest building when flying.
const SHORE_MARGIN = 3, SKY_MARGIN = 12;

// Keys the walker owns while active, by KeyboardEvent.code; the map's own shortcuts
// for these letters are suspended. E and Q do nothing here: on the map they rotate
// the view and expand things, which would only reshuffle the city around a walker.
const KEYS = new Set([
  'KeyW', 'KeyA', 'KeyS', 'KeyD', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight',
  'Space', 'ShiftLeft', 'ShiftRight', 'KeyC', 'KeyF', 'KeyE', 'KeyQ', 'Enter',
  'Escape', 'KeyV',
]);

// Planet curvature, by the character typed rather than the key's place on the board:
// on a German keyboard the key at BracketRight types '+', which would otherwise make
// '+' curve the planet while '-' still changed the map's depth.
const BIGGER = new Set(['+', '=', ']']), SMALLER = new Set(['-', '_', '[']);

export class Walker {
  /**
   * hooks: {
   *   onAim(i, x, y)   box index under the crosshair (-1: none) and its client position
   *   onHit(box, n)    a dart tagged a box; n: modules tagged so far
   *   onInspect(box)   Enter: show the box's details (null: nothing aimed at)
   *   onExit()         the walker left walk mode (V, Esc)
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
    this.aim = { i: -1, point: null };
    this.p = { x: 0, z: 0, feet: 0, vy: 0, yaw: 0, pitch: 0, ground: true, fly: false };
    this.radius = 40;
    this.fov = FOV;
    this.bindInput();
  }

  /** Keys that belong to walk mode while it is active. */
  owns(e) {
    return this.active && (KEYS.has(e.code) || BIGGER.has(e.key) || SMALLER.has(e.key));
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
    // Darts in flight aim at boxes of the old layout: let them go.
    for (const dart of this.darts) this.scene.scene.remove(dart.mesh);
    this.darts = [];
    for (const b of boxes) {
      const x0 = Math.floor((b.x - b.w / 2) / CELL), x1 = Math.floor((b.x + b.w / 2) / CELL);
      const z0 = Math.floor((b.z - b.d / 2) / CELL), z1 = Math.floor((b.z + b.d / 2) / CELL);
      for (let x = x0; x <= x1; x++) for (let z = z0; z <= z1; z++) {
        const k = x + ',' + z;
        if (!this.grid.has(k)) this.grid.set(k, []);
        this.grid.get(k).push(b);
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

  /** Starts walking in front of `box`, or on the south road of `block`. bounds sizes the planet. */
  enter(box, block, bounds) {
    const diag = Math.hypot(bounds.maxX - bounds.minX, bounds.maxZ - bounds.minZ);
    this.radius = clamp(diag * 0.6, 15, Math.min(600, this.maxRadius()));
    this.active = true;
    this.hud.hidden = false;
    this.scene.setWalking(true, this.radius);
    this.scene.scene.add(this.beacons);
    this.setFog();
    if (box) this.teleport(box);
    else Object.assign(this.p, { x: block.x, z: block.z + block.d / 2 - 0.15, yaw: 0, pitch: -0.15 });
    Object.assign(this.p, { vy: 0, fly: false });
    this.p.feet = this.height(this.p.x, this.p.z);
    this.last = performance.now();
    this.scoped = false;
    this.fov = FOV;
    this.drawHud();
    this.loop();
    // Like a first-person shooter: the pointer is captured at the reticle at once (V
    // and the Walk button are user gestures, which browsers require for this).
    this.lockPointer();
  }

  exit() {
    if (!this.active) return;
    this.active = false;
    this.keys.clear();
    for (const dart of this.darts) this.scene.scene.remove(dart.mesh);
    this.darts = [];
    this.scene.scene.remove(this.beacons);
    if (document.pointerLockElement) document.exitPointerLock();
    cancelAnimationFrame(this.frame);
    this.hud.hidden = true;
    this.hud.classList.remove('compact');
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

  // Locking is asynchronous: a lock asked for just before leaving would be granted
  // afterwards and hide the cursor over the map, with nothing listening to it.
  lockPointer() {
    if (!this.active) return;
    const done = this.scene.renderer.domElement.requestPointerLock?.();
    done?.then?.(() => this.active || document.exitPointerLock(), () => {});
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

    window.addEventListener('keydown', e => {
      if (!this.owns(e) || e.target.closest('input, select, textarea, dialog') || e.ctrlKey || e.metaKey || e.altKey) return;
      // This listener is registered before the map's; stopping here keeps the map from
      // acting on the same key (V would leave walk mode and re-enter it at once).
      e.preventDefault();
      e.stopImmediatePropagation();
      if (e.repeat) return;
      this.keys.add(e.code);
      if (BIGGER.has(e.key)) this.setRadius(this.radius * 1.25);
      else if (SMALLER.has(e.key)) this.setRadius(this.radius / 1.25);
      switch (e.code) {
        case 'KeyF': this.p.fly = !this.p.fly; this.p.vy = 0; this.drawHud(); break;
        case 'Escape':
          // Browsers usually swallow the Esc that frees the pointer; if not, free it first.
          if (document.pointerLockElement) document.exitPointerLock();
          else this.exit();
          break;
        case 'Enter': this.hooks.onInspect(this.aimed()); break;
        case 'KeyV': this.exit(); break;
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
      if (this.active && drag && !drag.moved && e.button === 0) this.lockPointer();
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
      if (!this.active) return;
      e.preventDefault();
      this.fov = clamp(this.fov * Math.exp(e.deltaY * 0.001), MIN_FOV, MAX_FOV); // zoom
    }, { passive: false });
    document.addEventListener('pointerlockchange', () => {
      fresh = document.pointerLockElement === canvas;
      if (fresh && !this.active) document.exitPointerLock(); // never keep the map's cursor hidden
      if (this.active) this.drawHud();
    });
  }

  look(dx, dy) {
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
      this.scene.setWalker(this.p.x, this.p.feet, this.p.z, EYE, this.p.yaw, this.p.pitch);
      this.updateAim();
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
    const k = this.keys, p = this.p;
    const turn = (k.has('ArrowLeft') ? 1 : 0) - (k.has('ArrowRight') ? 1 : 0);
    p.yaw += turn * TURN * dt;
    const fwd = (k.has('KeyW') || k.has('ArrowUp') ? 1 : 0) - (k.has('KeyS') || k.has('ArrowDown') ? 1 : 0);
    const side = (k.has('KeyD') ? 1 : 0) - (k.has('KeyA') ? 1 : 0);
    const run = k.has('ShiftLeft') || k.has('ShiftRight');
    const speed = p.fly ? (run ? FLY * 2.5 : FLY) : run ? RUN : WALK;
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
    // are already in it, say after landing there).
    const climb = p.feet + (p.ground || p.fly ? STEP : 0.05);
    const wet = !p.fly && this.height(p.x, p.z) <= WATER;
    const ok = h => h <= climb && (p.fly || wet || h > WATER);
    const nx = p.x + mx * speed * dt;
    if (ok(this.height(nx, p.z))) p.x = nx;
    const nz = p.z + mz * speed * dt;
    if (ok(this.height(p.x, nz))) p.z = nz;
    // The key list folds away while moving and comes back after a pause.
    if (len > 0) {
      this.movedAt = performance.now();
      if (!this.hudTimer) this.hudTimer = setTimeout(() => this.hud.classList.add('compact'), 2500);
    } else if (this.hudTimer && performance.now() - this.movedAt > 6000) {
      clearTimeout(this.hudTimer);
      this.hudTimer = 0;
      this.hud.classList.remove('compact');
    }

    this.confine();

    const floor = this.height(p.x, p.z);
    if (p.fly) {
      const ceiling = (this.limits?.maxY ?? 0) + SKY_MARGIN;
      p.feet = Math.max(floor, Math.min(ceiling, p.feet + my * speed * dt));
      p.vy = 0;
      p.ground = p.feet <= floor;
      return;
    }
    if (k.has('Space') && p.ground) p.vy = JUMP;
    p.vy -= GRAVITY * dt;
    p.feet += p.vy * dt;
    p.ground = p.feet <= floor;
    if (p.ground) {
      p.feet = floor;
      p.vy = 0;
    }
  }

  /** The block the walker stands on, to keep them by it across a relayout (reanchor). */
  anchorFor() {
    let box = null;
    for (const b of this.at(this.p.x, this.p.z)) if (b.y <= this.p.feet + 0.01 && (!box || b.y > box.y)) box = b;
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
    for (const [dx, dz] of [[0, 0], [BODY, BODY], [BODY, -BODY], [-BODY, BODY], [-BODY, -BODY]]) {
      for (const b of this.at(x + dx, z + dz)) top = Math.max(top, b.y + b.h);
      for (const r of this.ramps) top = Math.max(top, rampHeight(r, x + dx, z + dz));
      for (const r of this.bridges) top = Math.max(top, bridgeHeight(r, x + dx, z + dz));
    }
    return top;
  }

  /** Boxes whose footprint contains (x, z). */
  *at(x, z) {
    for (const b of this.grid.get(Math.floor(x / CELL) + ',' + Math.floor(z / CELL)) || []) {
      if (Math.abs(x - b.x) <= b.w / 2 && Math.abs(z - b.z) <= b.d / 2) yield b;
    }
  }

  /** The box containing a flat-map point, or null. */
  boxAt(v) {
    for (const b of this.at(v.x, v.z)) if (v.y >= b.y - 0.02 && v.y <= b.y + b.h + 0.02) return b;
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
    let hit = null, point = null;
    for (let t = 0.2; t < REACH; t += 0.04 + t * 0.008) {
      v.copy(cam.position).addScaledVector(dir, t);
      this.scene.unbend(v);
      if (v.y < WATER) break;
      hit = this.boxAt(v);
      if (hit) { point = v.clone(); break; }
    }
    // The ground underfoot and the shore are scenery, not targets.
    const i = hit && !this.underfoot(hit) ? hit.i : -1;
    this.aim = { i, point };
    const r = this.scene.renderer.domElement.getBoundingClientRect();
    this.hooks.onAim(i, r.left + r.width / 2, r.top + r.height / 2);
  }

  // ------------------------------------------------------------------ darts

  // A dart fired at an aimed box flies a shallow arc to the aimed point and always
  // hits; fired at nothing it flies ahead under gravity until it hits something or
  // falls into the water.
  fire() {
    const p = this.p;
    const start = new THREE.Vector3(p.x, p.feet + EYE - 0.08, p.z);
    const mesh = dartMesh(this.scene);
    mesh.position.copy(start);
    this.scene.scene.add(mesh);
    const target = this.aimed(), to = this.aim.point;
    if (target) {
      const dist = start.distanceTo(to);
      this.darts.push({ mesh, start, to, target, t: 0, T: Math.max(0.12, dist / DART_SPEED), arc: 0.05 + dist * 0.03 });
    } else {
      const dir = new THREE.Vector3(-Math.sin(p.yaw) * Math.cos(p.pitch), Math.sin(p.pitch) + 0.04, -Math.cos(p.yaw) * Math.cos(p.pitch));
      this.darts.push({ mesh, vel: dir.multiplyScalar(DART_SPEED), t: 0 });
    }
  }

  updateDarts(dt) {
    const done = [], prev = new THREE.Vector3(), dir = new THREE.Vector3();
    for (const dart of this.darts) {
      dart.t += dt;
      const m = dart.mesh;
      prev.copy(m.position);
      if (dart.target) {
        const u = Math.min(1, dart.t / dart.T);
        m.position.lerpVectors(dart.start, dart.to, u);
        m.position.y += dart.arc * 4 * u * (1 - u);
        if (u >= 1) {
          done.push(dart);
          this.tag(dart.target);
        }
      } else {
        dart.vel.y -= 6 * dt;
        m.position.addScaledVector(dart.vel, dt);
        const hit = this.boxAt(m.position);
        if (hit && hit.kind !== 'land' && hit.kind !== 'terrace') this.tag(hit);
        if (hit || m.position.y < WATER || dart.t > 4) done.push(dart);
      }
      // Point along the flight path.
      if (dir.subVectors(m.position, prev).lengthSq() > 1e-10) m.quaternion.setFromUnitVectors(FORWARD, dir.normalize());
    }
    for (const dart of done) {
      this.scene.scene.remove(dart.mesh);
      this.darts.splice(this.darts.indexOf(dart), 1);
    }
  }

  tag(box) {
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
    this.hud.querySelector('.w-mode').textContent = this.p.fly ? 'flying' : 'walking';
    this.hud.querySelector('.w-tagged').textContent = this.tagged.size;
    this.hud.querySelector('.w-radius').textContent = Math.round(this.radius);
    this.hud.querySelector('.w-hint').textContent = locked
      ? 'Click: fire a tracking dart · hold right: scope · V: back to the map · Esc: free the mouse · ?: all controls'
      : 'Click the map to capture the mouse, or drag to look · V / Esc: back to the map · ?: all controls';
  }
}

const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

const DART_SPEED = 30;
const FORWARD = new THREE.Vector3(0, 0, 1); // the dart geometry's nose
const BEACON = '#ff8a1f';
let beaconParts = null;

// A tracking dart: a dark shaft, a glowing tip and two crossed fins, nose along +z.
// Its materials bend like the map's.
let dart = null;
function dartMesh(scene) {
  dart ||= [
    [new THREE.CylinderGeometry(0.008, 0.008, 0.26, 6).rotateX(Math.PI / 2), '#2b2d31'],
    [new THREE.ConeGeometry(0.018, 0.06, 8).rotateX(Math.PI / 2).translate(0, 0, 0.16), BEACON],
    [new THREE.BoxGeometry(0.07, 0.003, 0.06).translate(0, 0, -0.11), '#d8dadf'],
    [new THREE.BoxGeometry(0.003, 0.07, 0.06).translate(0, 0, -0.11), '#d8dadf'],
  ].map(([geo, color]) => [geo, scene.bendable(new THREE.MeshBasicMaterial({ color }))]);
  const g = new THREE.Group();
  for (const [geo, mat] of dart) {
    const m = new THREE.Mesh(geo, mat);
    m.frustumCulled = false;
    g.add(m);
  }
  return g;
}
