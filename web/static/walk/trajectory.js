// Where a shot would go, drawn before it is taken: a thin line of light from the muzzle
// along the path the shot would fly, and a marker where it would land. Mixed into Walker
// (walk.js), like shots.js, whose decisions it repeats.
//
// It is worked out as the shot itself would be (shots.js fire): aimed at a bug or at a
// building in reach, along the path the launcher would solve for it in the wind
// blowing now (ballistics.js aim); at anything else, along the flight the shot would
// make from the muzzle under its own physics, until it strikes something, falls into
// the water or reaches the end of the tool's reach. A tracking dart that misses steers
// for a wall on the way, which a line drawn now cannot know: it shows where the dart
// would land unsteered.
//
// Implements: REQ-TOOL-082

import * as THREE from '../vendor/three.module.min.js';
import { aim, fly } from './ballistics.js';
import { EYE, WATER, REACH, faceOf } from './walkbase.js';

const MOST = 160;          // points along a drawn path at most
const SPACING = 0.25;      // units between them
const WIDTH = 0.0019;      // the line's width, as a share of its distance: about two pixels
const HALO = 3.4;          // how much wider the dark edge under it is, so it reads on a pale sky too
const HIDDEN = 0.28;       // how strongly the part behind a building shows, of the rest
const STEP = 1 / 60;       // seconds between points of a free flight
const LONGEST = 5;         // seconds of free flight drawn at most
const UP = new THREE.Vector3(0, 0, 1); // a ring's own normal (RingGeometry lies in xy)
const Y = new THREE.Vector3(0, 1, 0);

export const trajectory = {
  /** Draws, moves or hides the guide for this frame. */
  drawPath() {
    const tool = this.primary;
    const guide = this.guide ||= makeGuide(this.scene);
    const wanted = this.active && tool?.projectile && !tool.douses && !this.handsOff && !this.still && !this.frozen
      && !this.showing && !this.arrival && this.dying === null && !this.wheel?.open && !this.pull;
    if (!wanted) { guide.group.visible = false; return; }
    const flight = tool.flight;
    const start = this.muzzle(this.viewmodel, guide.start)
      || guide.start.set(this.p.x, this.p.feet + EYE - 0.08, this.p.z);
    const air = this.airNow || guide.calm;
    const target = this.aimed(), bug = this.aim.bug;
    let path = null, landed = null, normal = null;
    if (bug || target) {
      const to = bug ? bug.position : this.aim.point;
      path = to && aim(start, to, flight, air)?.path;
      if (path) {
        landed = to;
        normal = target ? faceOf(target, to) : null;
      }
    }
    if (!path) ({ path, landed, normal } = this.freeFlight(start, tool, air, guide));
    guide.draw(path, landed, normal, this.scene.walkCamera.position, performance.now() / 1000);
  },

  /** The path a shot let go along the view would fly, and where and on what it would stop. */
  freeFlight(start, tool, air, guide) {
    const p = this.p, flight = tool.flight;
    const pos = guide.pos.copy(start);
    const vel = guide.vel.set(-Math.sin(p.yaw) * Math.cos(p.pitch), Math.sin(p.pitch) + 0.04, -Math.cos(p.yaw) * Math.cos(p.pitch))
      .normalize().multiplyScalar(flight.speed);
    const reach = tool.reach ?? REACH;
    const path = guide.free;
    path.length = 0;
    path.push(pos.clone());
    for (let t = 0; t < LONGEST; t += STEP) {
      const before = guide.before.copy(pos);
      fly(pos, vel, flight, air, STEP);
      const hit = this.boxAt(pos);
      if (hit) {
        // A fast shot goes a unit a step, so where it is found is well inside the
        // wall: halved back to the face, between the step before and this one.
        const outside = before, inside = pos.clone();
        for (let k = 0; k < 8; k++) {
          const half = outside.clone().lerp(inside, 0.5);
          if (this.boxAt(half)) inside.copy(half); else outside.copy(half);
        }
        path.push(outside.clone());
        return { path, landed: outside.clone(), normal: faceOf(hit, outside) };
      }
      path.push(pos.clone());
      if (pos.y < WATER) return { path, landed: pos.clone(), normal: new THREE.Vector3(0, 1, 0) };
      if (pos.distanceTo(start) > reach) return { path, landed: null, normal: null };
    }
    return { path, landed: null, normal: null };
  },
};

/**
 * The guide, in the walk scene: the line, a ribbon turned to face the eye and as wide
 * on screen near as far, brightening out of the muzzle and fading towards the end of
 * a shot that lands nowhere; and the marker, a ring with a dot in it, lying on what
 * the shot would strike and breathing slowly so it reads as a mark rather than a part
 * of the city. Both are drawn twice: fully where they are in view, and faintly where
 * a building is in front of them, so the path is always there to be read and still
 * says what is in the way. Bendable like everything out there.
 */
function makeGuide(scene) {
  const group = new THREE.Group();
  group.visible = false;
  // Two ribbons: the dark edge, wide and soft, and the bright core on it.
  const ribbon = () => {
    const positions = new Float32Array(MOST * 2 * 3), colors = new Float32Array(MOST * 2 * 4);
    const index = [];
    for (let i = 0; i + 1 < MOST; i++) {
      const a = i * 2;
      index.push(a, a + 1, a + 2, a + 1, a + 3, a + 2);
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute('color', new THREE.BufferAttribute(colors, 4));
    geometry.setIndex(index);
    return { positions, colors, geometry };
  };
  const edge = ribbon(), core = ribbon();
  const marker = new THREE.Group();
  const markerParts = [
    [new THREE.RingGeometry(0.7, 1.15, 48), '#06131f', 0.38], // its dark edge
    [new THREE.RingGeometry(0.82, 1, 48), '#ffffff', 1],
    [new THREE.CircleGeometry(0.17, 20), '#ffffff', 1],
  ];
  // Over the city and under it: the same geometry, once depth-tested and once not.
  const pass = (seen, opacity, order) => {
    const look = { transparent: true, depthWrite: false, depthTest: seen, side: THREE.DoubleSide };
    const lines = [edge, core].map(r => new THREE.Mesh(r.geometry, scene.bendable(new THREE.MeshBasicMaterial({ ...look, vertexColors: true, opacity }))));
    const rings = markerParts.map(([g, color, alpha]) => new THREE.Mesh(g, scene.bendable(new THREE.MeshBasicMaterial({ ...look, color, opacity: opacity * alpha }))));
    [...lines, ...rings].forEach((m, k) => { m.frustumCulled = false; m.renderOrder = order + (k % 3); });
    group.add(...lines);
    marker.add(...rings);
    return [...lines, ...rings];
  };
  const shown = [...pass(false, HIDDEN, 40), ...pass(true, 1, 44)];
  group.add(marker);
  scene.scene.add(group);
  const side = new THREE.Vector3(), along = new THREE.Vector3(), toEye = new THREE.Vector3(), q = new THREE.Vector3();
  // The line's color: a cool white, warming a little towards the far end.
  const NEAR = new THREE.Color('#f2fbff'), FAR = new THREE.Color('#ffe2a8'), DARK = new THREE.Color('#06131f'), c = new THREE.Color();
  return {
    group, shown, start: new THREE.Vector3(), pos: new THREE.Vector3(), vel: new THREE.Vector3(), before: new THREE.Vector3(),
    calm: new THREE.Vector3(), free: [],
    /** Lays the line along `path` and the marker at `landed`, facing out along `normal`, as seen from `eye` at `time`. */
    draw(path, landed, normal, eye, time) {
      // Resampled evenly, so the ribbon bends smoothly and its fade is even.
      const points = [path[0]];
      let next = SPACING, total = 0;
      for (let i = 1; i < path.length && points.length < MOST; i++) {
        const a = path[i - 1], b = path[i], step = a.distanceTo(b);
        while (next <= total + step && points.length < MOST - 1) {
          points.push(a.clone().lerp(b, (next - total) / step));
          next += SPACING;
        }
        total += step;
      }
      if (points.length < MOST && path.length > 1) points.push(path[path.length - 1]);
      const n = points.length, length = Math.max(1e-6, (n - 1) * SPACING);
      for (let i = 0; i < n; i++) {
        const p = points[i];
        along.subVectors(points[Math.min(n - 1, i + 1)], points[Math.max(0, i - 1)]);
        toEye.subVectors(eye, p);
        side.crossVectors(along, toEye).normalize().multiplyScalar(WIDTH * toEye.length());
        const u = (i * SPACING) / length;
        // In from nothing at the muzzle; out to nothing at the end of a shot that lands
        // nowhere, and still clear where one strikes, so the marker is what it meets.
        const alpha = Math.min(1, i * SPACING / 0.6) * (landed ? 1 : Math.min(1, (1 - u) * 4));
        c.copy(NEAR).lerp(FAR, u * 0.8);
        for (const [r, k, rgb, a] of [[edge, HALO, DARK, alpha * 0.32], [core, 1, c, alpha * 0.95]]) {
          r.positions.set([p.x - side.x * k, p.y - side.y * k, p.z - side.z * k, p.x + side.x * k, p.y + side.y * k, p.z + side.z * k], i * 6);
          r.colors.set([rgb.r, rgb.g, rgb.b, a, rgb.r, rgb.g, rgb.b, a], i * 8);
        }
      }
      for (const r of [edge, core]) {
        r.geometry.attributes.position.needsUpdate = true;
        r.geometry.attributes.color.needsUpdate = true;
        r.geometry.setDrawRange(0, Math.max(0, n - 1) * 6);
      }
      marker.visible = !!landed;
      if (landed) {
        const distance = q.subVectors(eye, landed).length();
        marker.position.copy(landed);
        if (normal) marker.position.addScaledVector(normal, 0.015);
        marker.quaternion.setFromUnitVectors(UP, normal || Y);
        // As large on screen near as far, and breathing a little.
        marker.scale.setScalar(Math.max(0.07, distance * 0.026) * (1 + 0.08 * Math.sin(time * 4)));
      }
      group.visible = true;
    },
  };
}
