// Where a shot would go, drawn before it is taken: a thin line of light from the muzzle
// along the path the shot would fly, and a marker where it would land. Mixed into Walker
// (walk.js), like shots.js, whose flight it repeats.
//
// It is not worked out alongside the shot but by it: a stand-in shot is launched as the
// real one would be now (shots.js launch: along the view, carried by the walker's own
// motion) and flown down the same steps (flightStep: the same physics, in the same wind,
// steering as a tracking dart steers), until it touches a bug or something solid, falls
// into the water or reaches the end of its reach. So the line is the path, and where
// the marker is is where the shot lands - unless what it lands on moves meanwhile, or
// the wind turns. A nail's scatter is the one thing left out, being chance.
//
// The hunting hand is aimed with it, and nothing else: the crosshair is taken away for a
// tool that throws (style.css .guided), since a shot that drops does not land where the
// middle of the view is. Where it would land on nothing - out over the water, past its
// reach - nothing is drawn.
//
// Implements: REQ-TOOL-082

import * as THREE from '../vendor/three.module.min.js';
import { EYE, WATER, REACH, faceOf } from './walkbase.js';
import { FLIGHT_STEP } from './shots.js';

const MOST = 160;          // points along a drawn path at most
const SPACING = 0.25;      // units between them
const WIDTH = 0.0012;      // the line's width, as a share of its distance: about a pixel
const HALO = 2.6;          // how much wider the faint dark edge under it is, so it reads on a pale sky too
const HIDDEN = 0.3;        // how strongly the part behind a building shows, of the rest
const STRENGTH = 0.6;      // the line at its strongest: a guide over the view, not a part of it
const FADE_IN = 3;         // units from the muzzle it takes to come up to that: the near end is the
                           // largest on screen, and the least use
const LONGEST = 5;         // seconds of flight looked ahead at most
const UP = new THREE.Vector3(0, 0, 1); // a ring's own normal (RingGeometry lies in xy)
const Y = new THREE.Vector3(0, 1, 0);
const END = '#ffd98a';     // the line's color where it lands, and the marker's

/** Whether a tool is aimed by its guide rather than its crosshair: one that throws, but not the extinguisher's spray. */
export const guided = tool => !!tool?.projectile && !tool.douses;

export const trajectory = {
  /**
   * Flies a stand-in for the shot the hunting hand would throw now, and returns
   * { path, point, normal, bug, box } - where it would stop and on what - or null when
   * it would come down on nothing. Kept as this.prediction for the frame.
   */
  predict() {
    const tool = this.primary, guide = this.guide ||= makeGuide(this.scene);
    const start = this.muzzle(this.viewmodel, guide.start)
      || guide.start.set(this.p.x, this.p.feet + EYE - 0.08, this.p.z);
    const shot = guide.shot;
    shot.tool = tool;
    shot.flight = tool.flight;
    shot.mesh.position.copy(start);
    this.launch(tool.flight, shot.vel);
    shot.lock = undefined;
    const reach = tool.reel ? tool.reel.max : (tool.reach ?? REACH);
    const path = guide.path;
    path.length = 0;
    path.push(start.clone());
    const at = shot.mesh.position, before = guide.before;
    for (let t = 0; t < LONGEST; t += FLIGHT_STEP) {
      before.copy(at);
      const { bug, box } = this.flightStep(shot, FLIGHT_STEP);
      if (bug) {
        path.push(bug.position.clone());
        return { path, point: bug.position.clone(), normal: null, bug, box: null };
      }
      if (box) {
        // Where it struck is found inside the box, up to a step deep; the face is
        // halfway back, found by halving, so the line ends on the surface.
        const outside = before.clone(), inside = at.clone();
        for (let k = 0; k < 10; k++) {
          const half = outside.clone().lerp(inside, 0.5);
          if (this.boxAt(half)) inside.copy(half); else outside.copy(half);
        }
        path.push(outside);
        return { path, point: outside.clone(), normal: faceOf(box, outside), bug: null, box };
      }
      path.push(at.clone());
      if (at.y < WATER || at.distanceTo(start) > reach) return null;
    }
    return null;
  },

  /** Draws, moves or hides the guide for this frame, from this.prediction (updateAim). */
  drawPath() {
    const guide = this.guide ||= makeGuide(this.scene);
    const wanted = this.active && guided(this.primary) && !this.handsOff && !this.still && !this.frozen
      && !this.showing && !this.arrival && this.dying === null && !this.wheel?.open && !this.pull;
    const at = wanted && this.prediction;
    if (!at) { guide.group.visible = false; return; }
    const eye = this.scene.walkCamera.position;
    // A marker on a bug faces the eye; on a surface it lies on it.
    const normal = at.normal || guide.toEye.subVectors(eye, at.point).normalize();
    guide.draw(at.path, at.point, normal, eye);
  },
};

/**
 * The guide, in the walk scene: the line, a ribbon a pixel or so wide turned to face
 * the eye, half see-through, coming up out of nothing over the first few units from the
 * muzzle; and the marker, a ring with a dot in it, lying on what the shot would strike.
 * It is there to be glanced at, not looked at: the view is the city. Both are drawn
 * twice: as they are where they are in view, and fainter where a building is in front
 * of them, so the path is always there to be read and still says what is in the way.
 * Bendable like everything out there.
 */
function makeGuide(scene) {
  const group = new THREE.Group();
  group.visible = false;
  // Two ribbons: the faint dark edge, and the light core on it.
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
  // In the line's own color where it ends, so the line runs into it rather than up to it.
  const markerParts = [
    [new THREE.RingGeometry(0.72, 1.1, 48), '#06131f', 0.25], // its faint dark edge
    [new THREE.RingGeometry(0.82, 1, 48), END, 1],
    [new THREE.CircleGeometry(0.2, 24), END, 1],
  ];
  // Over the city and under it: the same geometry, once depth-tested and once not.
  const pass = (seen, opacity, order) => {
    const look = { transparent: true, depthWrite: false, depthTest: seen, side: THREE.DoubleSide };
    const lines = [edge, core].map(r => new THREE.Mesh(r.geometry, scene.bendable(new THREE.MeshBasicMaterial({ ...look, vertexColors: true, opacity }))));
    const rings = markerParts.map(([g, color, alpha]) => new THREE.Mesh(g, scene.bendable(new THREE.MeshBasicMaterial({ ...look, color, opacity: opacity * alpha * 0.75 }))));
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
  const NEAR = new THREE.Color('#f2fbff'), FAR = new THREE.Color(END), DARK = new THREE.Color('#06131f'), c = new THREE.Color();
  return {
    group, shown, start: new THREE.Vector3(), before: new THREE.Vector3(), toEye: new THREE.Vector3(), path: [],
    // The stand-in shot the path is flown with: what flightStep needs of a shot.
    shot: { mesh: { position: new THREE.Vector3() }, vel: new THREE.Vector3(), tool: null, flight: null, lock: undefined },
    /** Lays the line along `path` and the marker at `landed`, facing out along `normal`, as seen from `eye`. */
    draw(path, landed, normal, eye) {
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
        // In from nothing at the muzzle, and full from there to the marker it runs into.
        const alpha = STRENGTH * Math.min(1, i * SPACING / FADE_IN) ** 2;
        c.copy(NEAR).lerp(FAR, u);
        for (const [r, k, rgb, a] of [[edge, HALO, DARK, alpha * 0.25], [core, 1, c, alpha]]) {
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
        // As large on screen near as far.
        marker.scale.setScalar(Math.max(0.05, distance * 0.017));
      }
      group.visible = true;
    },
  };
}
