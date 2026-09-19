// Walk mode: explore the map on foot, in first person, on a small planet. The walker
// lives on the flat map (layout coordinates) — collisions, heights and newspapers are
// all computed there — and MapScene bends what is drawn around the walker's feet.
//
// Paperboy rules: throw newspapers at buildings to select them.

import * as THREE from './vendor/three.module.min.js';

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
// How far the walker may leave the map: over the water beyond the outermost shore,
// and above its tallest building when flying.
const SHORE_MARGIN = 3, SKY_MARGIN = 12;

// Keys the walker owns while active, by KeyboardEvent.code; the map's own shortcuts
// for these letters are suspended.
const KEYS = new Set([
  'KeyW', 'KeyA', 'KeyS', 'KeyD', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight',
  'Space', 'ShiftLeft', 'ShiftRight', 'KeyC', 'KeyF', 'KeyE', 'KeyQ', 'Enter',
  'BracketLeft', 'BracketRight', 'Escape', 'KeyV',
]);

export class Walker {
  /**
   * hooks: {
   *   onAim(i, x, y)   box index under the crosshair (-1: none) and its client position
   *   onHit(box)       a newspaper landed on a box
   *   onSelect(box)    Enter: select the aimed box without throwing
   *   onToggle(box)    E / right click: expand or collapse the aimed box
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
    this.papers = [];
    this.delivered = 0;
    this.aim = { i: -1, point: null };
    this.p = { x: 0, z: 0, feet: 0, vy: 0, yaw: 0, pitch: 0, ground: true, fly: false };
    this.radius = 40;
    this.bindInput();
  }

  /** Keys that belong to walk mode while it is active. */
  owns(e) {
    return this.active && KEYS.has(e.code);
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
    for (const b of boxes) {
      const x0 = Math.floor((b.x - b.w / 2) / CELL), x1 = Math.floor((b.x + b.w / 2) / CELL);
      const z0 = Math.floor((b.z - b.d / 2) / CELL), z1 = Math.floor((b.z + b.d / 2) / CELL);
      for (let x = x0; x <= x1; x++) for (let z = z0; z <= z1; z++) {
        const k = x + ',' + z;
        if (!this.grid.has(k)) this.grid.set(k, []);
        this.grid.get(k).push(b);
      }
    }
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
    this.setFog();
    if (box) this.teleport(box);
    else Object.assign(this.p, { x: block.x, z: block.z + block.d / 2 - 0.15, yaw: 0, pitch: -0.15 });
    Object.assign(this.p, { vy: 0, fly: false });
    this.p.feet = this.height(this.p.x, this.p.z);
    this.last = performance.now();
    this.drawHud();
    this.loop();
  }

  exit() {
    if (!this.active) return;
    this.active = false;
    this.keys.clear();
    for (const paper of this.papers) this.scene.scene.remove(paper.mesh);
    this.papers = [];
    if (document.pointerLockElement) document.exitPointerLock();
    cancelAnimationFrame(this.frame);
    this.hud.hidden = true;
    this.scene.setWalking(false);
    this.hooks.onAim(-1);
    this.hooks.onExit();
  }

  /** Stands the walker beside `box`, on its lowest free side, looking at it. */
  teleport(box) {
    const p = this.p, gap = 1.0;
    const spots = [[0, box.d / 2 + gap], [0, -box.d / 2 - gap], [box.w / 2 + gap, 0], [-box.w / 2 - gap, 0]]
      .map(([dx, dz]) => ({ x: box.x + dx, z: box.z + dz }))
      .map(s => ({ ...s, h: this.height(s.x, s.z) }))
      .sort((a, b) => a.h - b.h);
    const s = spots[0];
    Object.assign(p, { x: s.x, z: s.z, feet: s.h, vy: 0 });
    p.yaw = Math.atan2(-(box.x - s.x), -(box.z - s.z));
    const dist = Math.hypot(box.x - s.x, box.z - s.z);
    p.pitch = clamp(Math.atan2(box.y + box.h / 2 - (s.h + EYE), dist), -0.6, 0.9);
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
      switch (e.code) {
        case 'KeyF': this.p.fly = !this.p.fly; this.p.vy = 0; this.drawHud(); break;
        case 'KeyE': if (this.aimed()) this.hooks.onToggle(this.aimed()); break;
        case 'Enter': if (this.aimed()) this.hooks.onSelect(this.aimed()); break;
        case 'KeyQ': this.throwPaper(); break;
        case 'BracketLeft': this.setRadius(this.radius / 1.25); break;
        case 'BracketRight': this.setRadius(this.radius * 1.25); break;
        case 'Escape':
          // Browsers usually swallow the Esc that frees the pointer; if not, free it first.
          if (document.pointerLockElement) document.exitPointerLock();
          else this.exit();
          break;
        case 'KeyV': this.exit(); break;
      }
    });
    window.addEventListener('keyup', e => this.keys.delete(e.code));
    window.addEventListener('blur', () => this.keys.clear());

    // With the pointer locked, the mouse looks around and clicks throw; without it
    // (before the first click, or where locking is refused) dragging looks around.
    canvas.addEventListener('pointerdown', e => {
      if (!this.active) return;
      if (document.pointerLockElement === canvas) {
        if (e.button === 0) this.throwPaper();
        else if (e.button === 2 && this.aimed()) this.hooks.onToggle(this.aimed());
        return;
      }
      drag = { x: e.clientX, y: e.clientY, moved: false };
    });
    window.addEventListener('pointermove', e => {
      if (!this.active) return;
      if (document.pointerLockElement === canvas) this.look(e.movementX, e.movementY);
      else if (drag && e.buttons) {
        drag.moved ||= Math.hypot(e.clientX - drag.x, e.clientY - drag.y) > 4;
        if (drag.moved) this.look(e.movementX, e.movementY);
      }
    });
    window.addEventListener('pointerup', () => {
      if (this.active && drag && !drag.moved) canvas.requestPointerLock?.()?.catch?.(() => {});
      drag = null;
    });
    canvas.addEventListener('wheel', e => {
      if (!this.active) return;
      e.preventDefault();
      this.setRadius(this.radius * Math.exp(e.deltaY * 0.001));
    }, { passive: false });
    document.addEventListener('pointerlockchange', () => this.active && this.drawHud());
  }

  look(dx, dy) {
    this.p.yaw -= dx * LOOK;
    this.p.pitch = clamp(this.p.pitch - dy * LOOK, -1.5, 1.5);
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

  // Fog fades what lies near the horizon; a larger planet shows farther.
  setFog() {
    const far = Math.max(60, this.radius * 2.5);
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
      this.updatePapers(dt);
      this.scene.setWalker(this.p.x, this.p.feet, this.p.z, EYE, this.p.yaw, this.p.pitch);
      this.updateAim();
      this.scene.renderNow();
      this.hooks.onRender();
      this.loop();
    });
  }

  step(dt) {
    const k = this.keys, p = this.p;
    const turn = (k.has('ArrowLeft') ? 1 : 0) - (k.has('ArrowRight') ? 1 : 0);
    p.yaw += turn * TURN * dt;
    const fwd = (k.has('KeyW') || k.has('ArrowUp') ? 1 : 0) - (k.has('KeyS') || k.has('ArrowDown') ? 1 : 0);
    const side = (k.has('KeyD') ? 1 : 0) - (k.has('KeyA') ? 1 : 0);
    const run = k.has('ShiftLeft') || k.has('ShiftRight');
    const speed = p.fly ? (run ? FLY * 2.5 : FLY) : run ? RUN : WALK;
    let mx = -Math.sin(p.yaw) * fwd + Math.cos(p.yaw) * side;
    let mz = -Math.cos(p.yaw) * fwd - Math.sin(p.yaw) * side;
    const len = Math.hypot(mx, mz);
    if (len > 1) { mx /= len; mz /= len; }

    // Axis by axis, so the walker slides along walls.
    const climb = p.feet + (p.ground || p.fly ? STEP : 0.05);
    const nx = p.x + mx * speed * dt;
    if (this.height(nx, p.z) <= climb) p.x = nx;
    const nz = p.z + mz * speed * dt;
    if (this.height(p.x, nz) <= climb) p.z = nz;

    this.confine();

    const floor = this.height(p.x, p.z);
    if (p.fly) {
      const up = (k.has('Space') ? 1 : 0) - (k.has('KeyC') ? 1 : 0);
      const ceiling = (this.limits?.maxY ?? 0) + SKY_MARGIN;
      p.feet = Math.max(floor, Math.min(ceiling, p.feet + up * speed * 0.6 * dt));
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

  /** Keeps the walker over the map or the water just off its shores. */
  confine() {
    const l = this.limits, p = this.p;
    if (!l) return;
    p.x = clamp(p.x, l.minX - SHORE_MARGIN, l.maxX + SHORE_MARGIN);
    p.z = clamp(p.z, l.minZ - SHORE_MARGIN, l.maxZ + SHORE_MARGIN);
  }

  /** Top of the solid column under a body at (x, z): the highest box it overlaps. */
  height(x, z) {
    let top = WATER;
    for (const [dx, dz] of [[0, 0], [BODY, BODY], [BODY, -BODY], [-BODY, BODY], [-BODY, -BODY]]) {
      for (const b of this.at(x + dx, z + dz)) top = Math.max(top, b.y + b.h);
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
    const i = hit ? hit.i : -1;
    this.aim = { i, point };
    const r = this.scene.renderer.domElement.getBoundingClientRect();
    this.hooks.onAim(i, r.left + r.width / 2, r.top + r.height / 2);
  }

  // ------------------------------------------------------------------ newspapers

  // A paper thrown at an aimed box follows an arc to the aimed point and always lands;
  // thrown at nothing it flies ahead under gravity until it hits something or sinks.
  throwPaper() {
    const p = this.p;
    const start = new THREE.Vector3(p.x, p.feet + EYE - 0.1, p.z);
    const mesh = paperMesh(this.scene);
    mesh.position.copy(start);
    this.scene.scene.add(mesh);
    const target = this.aimed(), to = this.aim.point;
    if (target) {
      const dist = start.distanceTo(to);
      this.papers.push({ mesh, start, to, target, t: 0, T: Math.max(0.25, dist / 18), arc: 0.4 + dist * 0.12 });
    } else {
      const dir = new THREE.Vector3(-Math.sin(p.yaw) * Math.cos(p.pitch), Math.sin(p.pitch) + 0.15, -Math.cos(p.yaw) * Math.cos(p.pitch));
      this.papers.push({ mesh, vel: dir.multiplyScalar(18), t: 0 });
    }
  }

  updatePapers(dt) {
    const done = [];
    for (const paper of this.papers) {
      paper.t += dt;
      const m = paper.mesh;
      m.rotation.x += dt * 14;
      m.rotation.y += dt * 5;
      if (paper.target) {
        const u = Math.min(1, paper.t / paper.T);
        m.position.lerpVectors(paper.start, paper.to, u);
        m.position.y += paper.arc * 4 * u * (1 - u);
        if (u >= 1) {
          done.push(paper);
          this.deliver(paper.target);
        }
        continue;
      }
      paper.vel.y -= 9 * dt;
      m.position.addScaledVector(paper.vel, dt);
      const hit = this.boxAt(m.position);
      if (hit && hit.kind !== 'land' && hit.kind !== 'terrace') this.deliver(hit);
      if (hit || m.position.y < WATER || paper.t > 6) done.push(paper);
    }
    for (const paper of done) {
      this.scene.scene.remove(paper.mesh);
      this.papers.splice(this.papers.indexOf(paper), 1);
    }
  }

  deliver(box) {
    this.delivered++;
    this.drawHud();
    this.hooks.onHit(box);
  }

  drawHud() {
    const locked = !!document.pointerLockElement;
    this.hud.querySelector('.w-mode').textContent = this.p.fly ? 'flying' : 'walking';
    this.hud.querySelector('.w-papers').textContent = this.delivered;
    this.hud.querySelector('.w-radius').textContent = Math.round(this.radius);
    this.hud.querySelector('.w-hint').textContent = locked
      ? 'Click: throw a newspaper · Esc: free the mouse'
      : 'Click the map to look with the mouse, or drag · V / Esc: back to the map';
  }
}

const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

// A rolled newspaper: a pale roll with a band. Its materials bend like the map's.
let paper = null;
function paperMesh(scene) {
  paper ||= [
    [new THREE.CylinderGeometry(0.07, 0.07, 0.34, 10), '#efece2'],
    [new THREE.CylinderGeometry(0.075, 0.075, 0.07, 10), '#c0392b'],
  ].map(([geo, color]) => [geo.rotateZ(Math.PI / 2), scene.bendable(new THREE.MeshBasicMaterial({ color }))]);
  const g = new THREE.Group();
  for (const [geo, mat] of paper) {
    const m = new THREE.Mesh(geo, mat);
    m.frustumCulled = false;
    g.add(m);
  }
  return g;
}
