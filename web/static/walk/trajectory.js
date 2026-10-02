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
import { EYE, WATER, REACH, faceOf, holds } from './walkbase.js';
import { aim } from './ballistics.js';
import { FLIGHT_STEP } from './shots.js';
import { hits } from './tools.js';

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
// The tolerance a bug is locked on to within: this far off the path, plus this share of
// the distance out, in units - a few pixels on the screen at any distance.
const ASSIST = 0.45, ASSIST_GROW = 0.035;
// The off hand's line: the colors of a hook that would hold and one that would glance
// off; how far under a roof's edge a hook is drawn up to; and how far off where the
// walker is looking that edge may be, in radians, and still be taken.
const HOLDS = '#8ff0c4', GLANCES = '#ff8f78', EDGE_IN = 0.3, SNAP = 0.22;
// A roof the view passes over is taken within this angle under it, or this many units
// near by; past the line's reach a wall is looked for this many reaches out, to say so.
const CONE = 0.07, CONE_MIN = 0.25, FAR_LOOK = 1.6;

/** Whether a tool is aimed by its guide rather than its crosshair: one that throws, but not the extinguisher's spray. */
export const guided = tool => !!tool?.projectile && !tool.douses;

export const trajectory = {
  /**
   * Flies a stand-in for the shot the hunting hand would throw now, and returns
   * { path, point, normal, bug, box } - where it would stop and on what - or null when
   * it would come down on nothing. Kept as this.prediction for the frame.
   */
  predict() {
    const tool = this.primary, guide = this.guide ||= makeGuide(this.scene, END);
    const start = this.muzzle(this.viewmodel, guide.start)
      || guide.start.set(this.p.x, this.p.feet + EYE - 0.08, this.p.z);
    // Flown straight first. A bug the path passes near enough is locked on to - the
    // shot will home on it - and the path flown again, homing, to see that it gets
    // there: a lock that would still miss, round a corner or behind a wall, is none.
    // Implements: REQ-TOOL-083
    const straight = this.flyStandIn(tool, start, null);
    if (straight?.bug) return { ...straight, lock: straight.bug };
    const near = hits(tool, 'bugs') ? this.nearestToPath(straight?.path || guide.path, start, tool) : null;
    if (near) {
      const homed = this.flyStandIn(tool, start, near);
      if (homed?.bug === near) return { ...homed, lock: near };
    }
    return straight && { ...straight, path: straight.path.slice(), lock: null };
  },

  /**
   * Flies a stand-in shot of `tool` from `start` as it would be launched now, homing on
   * `homing` (a bug) if given: { path, point, normal, bug, box }, or null when it would
   * come down on nothing.
   */
  flyStandIn(tool, start, homing) {
    const guide = this.guide;
    const shot = guide.shot;
    shot.tool = tool;
    shot.flight = tool.flight;
    shot.mesh.position.copy(start);
    this.launch(tool.flight, shot.vel);
    shot.lock = undefined;
    shot.homing = homing;
    const reach = tool.reel ? tool.reel.max : (tool.reach ?? REACH);
    const path = homing ? guide.homed : guide.path;
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

  /**
   * The bug a shot along `path` would pass nearest, if it passes within the tolerance
   * that far out - ASSIST plus ASSIST_GROW of the distance from `start`, as a share of
   * a unit: a bug a few pixels off the line across the street is taken, one a body's
   * width off it is not - or null.
   */
  nearestToPath(path, start, tool) {
    const bugs = this.bugs?.bugs;
    if (!bugs?.length || path.length < 2) return null;
    const reach = tool.reach ?? REACH;
    let best = null, bestMiss = Infinity;
    const closest = new THREE.Vector3(), toBug = new THREE.Vector3(), along = new THREE.Vector3();
    for (const bug of bugs) {
      if (bug.caught) continue;
      const b = bug.position;
      if (b.distanceTo(start) > reach + 1) continue;
      for (let i = 1; i < path.length; i++) {
        // How near the bug the segment from the point before to this one passes.
        const a = path[i - 1], q = path[i], seg = along.subVectors(q, a);
        const u = Math.max(0, Math.min(1, toBug.subVectors(b, a).dot(seg) / Math.max(1e-9, seg.lengthSq())));
        closest.copy(a).addScaledVector(seg, u);
        const miss = closest.distanceTo(b) / (ASSIST + ASSIST_GROW * closest.distanceTo(start));
        if (miss < 1 && miss < bestMiss) { bestMiss = miss; best = bug; }
      }
    }
    return best;
  },

  /** Draws, moves or hides the guide for this frame, from this.prediction (updateAim). */
  drawPath() {
    const guide = this.guide ||= makeGuide(this.scene);
    const wanted = this.active && guided(this.primary) && !this.handsOff && !this.still && !this.frozen
      && !this.showing && !this.arrival && this.dying === null && !this.wheel?.open && !this.pull;
    const at = wanted && this.prediction;
    this.drawLock(at?.lock || null);
    if (!at) { guide.group.visible = false; return; }
    const eye = this.scene.walkCamera.position;
    // A marker on a bug faces the eye; on a surface it lies on it.
    const normal = at.normal || guide.toEye.subVectors(eye, at.point).normalize();
    guide.draw(at.path, at.point, normal, eye);
  },

  /**
   * Brackets the bug a shot is locked on to, on the screen: four corners round it, as
   * large near as they need to be to frame it and never smaller than a thumbnail far
   * off - so what the next shot will catch is plain before it is taken.
   *
   * Implements: REQ-TOOL-083
   */
  drawLock(bug) {
    const el = this.lockEl ||= this.hud?.querySelector('.w-lock');
    if (!el) return;
    const where = bug && this.onScreen(bug.position);
    if (!where) { el.hidden = true; return; }
    const distance = this.scene.walkCamera.position.distanceTo(this.scene.bend(bug.position.clone()));
    const size = Math.max(22, Math.min(80, 300 / Math.max(0.5, distance)));
    el.hidden = false;
    el.style.left = `${(where.x * 100).toFixed(2)}%`;
    el.style.top = `${(where.y * 100).toFixed(2)}%`;
    el.style.setProperty('--size', `${size.toFixed(0)}px`);
  },

  /**
   * Where a line from the off hand would bite, and whether it would hold: the wall the
   * walker is looking at, as the line is aimed (walk.js useSecondary), and the path the
   * hook would fly there. Where a grapple gun's claw would find nothing to close on, the
   * aim is helped on to a roof's edge that does hold: up the wall it would bite too low
   * on, or down to one the view passes just over - in reach, in plain view and near
   * where the walker is looking. Returns { start, point, box, normal, holds, snapped,
   * far, path, flown }: `far` for a wall seen past the line's reach, drawn but not
   * fired at; null with nothing in sight.
   *
   * Implements: REQ-TOOL-083, REQ-TOOL-084
   */
  planLine(tool = this.secondary) {
    if (!tool?.reel) return null;
    const reach = tool.reach ?? REACH;
    const guide = this.lineGuide ||= makeGuide(this.scene, HOLDS);
    const start = this.muzzle(this.offhand, new THREE.Vector3())
      || new THREE.Vector3(this.p.x, this.p.feet + EYE - 0.08, this.p.z);
    const grips = tool.reel.grip !== undefined;
    const seen = this.lookingAt(grips ? reach * FAR_LOOK : reach);
    const passed = grips ? this.edgeUnderView(start, reach, seen) : null;
    let point = seen?.point, box = seen?.box, snapped = false, far = false;
    let hold = !!seen && holds(tool, box, point);
    if (seen && start.distanceTo(point) > reach) {
      if (passed) ({ point, box } = passed);
      else far = true;
      hold = !far;
      snapped = !far;
    } else if (seen && !hold && grips && box.kind !== 'land' && box.kind !== 'terrace') {
      const edge = this.roofEdge(box, point, start, reach) || passed?.point;
      if (edge) {
        box = edge === passed?.point ? passed.box : box;
        point = edge;
        snapped = hold = true;
      }
    } else if (!seen && passed) {
      ({ point, box } = passed);
      snapped = hold = true;
    }
    if (!point) return null;
    const flown = far ? null : aim(start, point, tool.flight, this.airNow || guide.calm);
    return { start, point, box, normal: faceOf(box, point), holds: hold, snapped, far, path: flown ? flown.path : [start, point], flown };
  },

  /**
   * The point on the edge of `box`'s roof straight up the face from `point`, EDGE_IN
   * under it, if a hook from `start` reaches it, nothing stands in the way, and it is
   * within SNAP of where the walker is looking - or null.
   */
  roofEdge(box, point, start, reach) {
    const edge = point.clone();
    edge.y = box.y + box.h - EDGE_IN;
    if (start.distanceTo(edge) > reach) return null;
    const eye = new THREE.Vector3(this.p.x, this.p.feet + EYE, this.p.z);
    const looking = point.clone().sub(eye).normalize(), wanted = edge.clone().sub(eye).normalize();
    if (looking.angleTo(wanted) > SNAP) return null;
    return this.inPlainView(box, edge) ? edge : null;
  },

  /**
   * The nearest roof the view passes over within CONE of it, short of `seen` (what it
   * ends on, if anything): { box, point }, the point EDGE_IN under the middle of the
   * roof's near edge where the view crosses it, if a hook from `start` reaches it and
   * nothing stands in the way - or null. A far roof is a thin line on the screen, and
   * a look a degree too high goes over it into the sky.
   */
  edgeUnderView(start, reach, seen) {
    const cam = this.scene.walkCamera;
    if (!cam.quaternion) return null;
    const direction = new THREE.Vector3(0, 0, -1).applyQuaternion(cam.quaternion);
    const eye = new THREE.Vector3(this.p.x, this.p.feet + EYE, this.p.z);
    const v = new THREE.Vector3(), under = new THREE.Vector3();
    const end = seen ? eye.distanceTo(seen.point) : reach * FAR_LOOK;
    for (let t = 0.5; t < end; t += 0.05 + t * 0.01) {
      v.copy(cam.position).addScaledVector(direction, t);
      this.scene.unbend?.(v);
      const drop = Math.max(CONE_MIN, eye.distanceTo(v) * Math.tan(CONE));
      under.set(v.x, v.y - drop, v.z);
      const box = this.boxAt(under);
      if (!box || box.kind === 'land' || box.kind === 'terrace' || box === seen?.box) continue;
      const top = box.y + box.h;
      if (v.y < top) continue;
      const point = nearEdge(box, eye, v);
      point.y = top - EDGE_IN;
      if (start.distanceTo(point) > reach || !this.inPlainView(box, point)) return null;
      return { box, point };
    }
    return null;
  },

  /** Whether the first thing on the way from the eye to `point` is `box`, at the point. */
  inPlainView(box, point) {
    const eye = new THREE.Vector3(this.p.x, this.p.feet + EYE, this.p.z);
    const toward = point.clone().sub(eye).normalize();
    const v = new THREE.Vector3(), d = eye.distanceTo(point);
    for (let t = 0.3; t < d + 0.3; t += 0.05) {
      v.copy(eye).addScaledVector(toward, t);
      const hit = this.boxAt(v);
      if (hit) return hit === box && v.distanceTo(point) < 0.45;
    }
    return false;
  },

  /** Draws, moves or hides the off hand's line guide for this frame, from this.linePlan. */
  drawLine() {
    const tool = this.secondary;
    const wanted = this.active && tool?.reel && !this.handsOff && !this.still && !this.frozen && !this.showing
      && !this.arrival && this.dying === null && !this.wheel?.open && !this.pull
      && !(tool.fuel && (this.dry.has(tool.id) || this.tank(tool) <= 0));
    const plan = wanted ? (this.linePlan = this.planLine(tool)) : (this.linePlan = null);
    const guide = this.lineGuide;
    this.drawGrip(plan?.snapped ? plan.point : null);
    if (!plan) { if (guide) guide.group.visible = false; return; }
    guide.tint(plan.holds ? HOLDS : GLANCES);
    guide.draw(plan.path, plan.point, plan.normal, this.scene.walkCamera.position);
  },

  /** Brackets the roof edge a hook's aim was helped on to, on the screen. */
  drawGrip(point) {
    const el = this.gripEl ||= this.hud?.querySelector('.w-grip');
    if (!el) return;
    const where = point && this.onScreen(point);
    el.hidden = !where;
    if (!where) return;
    el.style.left = `${(where.x * 100).toFixed(2)}%`;
    el.style.top = `${(where.y * 100).toFixed(2)}%`;
  },
};

// Where the horizontal line from `eye` through `v` enters `box`'s footprint, or the
// footprint's nearest point to `v` when it misses.
function nearEdge(box, eye, v) {
  const x0 = box.x - box.w / 2, x1 = box.x + box.w / 2, z0 = box.z - box.d / 2, z1 = box.z + box.d / 2;
  const dx = v.x - eye.x, dz = v.z - eye.z;
  let enter = 0, leave = Infinity;
  for (const [from, d, lo, hi] of [[eye.x, dx, x0, x1], [eye.z, dz, z0, z1]]) {
    if (Math.abs(d) < 1e-9) {
      if (from < lo || from > hi) { enter = Infinity; break; }
      continue;
    }
    const a = (lo - from) / d, b = (hi - from) / d;
    enter = Math.max(enter, Math.min(a, b));
    leave = Math.min(leave, Math.max(a, b));
  }
  if (enter <= leave && Number.isFinite(enter)) return new THREE.Vector3(eye.x + dx * enter, 0, eye.z + dz * enter);
  return new THREE.Vector3(Math.min(x1, Math.max(x0, v.x)), 0, Math.min(z1, Math.max(z0, v.z)));
}

/**
 * The guide, in the walk scene: the line, a ribbon a pixel or so wide turned to face
 * the eye, half see-through, coming up out of nothing over the first few units from the
 * muzzle; and the marker, a ring with a dot in it, lying on what the shot would strike.
 * It is there to be glanced at, not looked at: the view is the city. Both are drawn
 * twice: as they are where they are in view, and fainter where a building is in front
 * of them, so the path is always there to be read and still says what is in the way.
 * Bendable like everything out there. `spacing` is how finely the line follows the path
 * and `fadeIn` how far from its start it comes up to full: a ball's path is a few
 * units long, a shot's dozens.
 */
export function makeGuide(scene, end = END, { spacing = SPACING, fadeIn = FADE_IN } = {}) {
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
    [new THREE.RingGeometry(0.82, 1, 48), end, 1],
    [new THREE.CircleGeometry(0.2, 24), end, 1],
  ];
  const ends = []; // the marker's colored parts, which take the line's end color (tint)
  // Over the city and under it: the same geometry, once depth-tested and once not.
  const pass = (seen, opacity, order) => {
    const look = { transparent: true, depthWrite: false, depthTest: seen, side: THREE.DoubleSide };
    const lines = [edge, core].map(r => new THREE.Mesh(r.geometry, scene.bendable(new THREE.MeshBasicMaterial({ ...look, vertexColors: true, opacity }))));
    const rings = markerParts.map(([g, color, alpha]) => new THREE.Mesh(g, scene.bendable(new THREE.MeshBasicMaterial({ ...look, color, opacity: opacity * alpha * 0.75 }))));
    [...lines, ...rings].forEach((m, k) => { m.frustumCulled = false; m.renderOrder = order + (k % 3); });
    ends.push(rings[1], rings[2]);
    group.add(...lines);
    marker.add(...rings);
    return [...lines, ...rings];
  };
  const shown = [...pass(false, HIDDEN, 40), ...pass(true, 1, 44)];
  group.add(marker);
  scene.scene.add(group);
  const side = new THREE.Vector3(), along = new THREE.Vector3(), toEye = new THREE.Vector3(), q = new THREE.Vector3();
  // The line's color: a cool white, warming a little towards the far end.
  const NEAR = new THREE.Color('#f2fbff'), FAR = new THREE.Color(end), DARK = new THREE.Color('#06131f'), c = new THREE.Color();
  return {
    group, shown, calm: new THREE.Vector3(), start: new THREE.Vector3(), before: new THREE.Vector3(), toEye: new THREE.Vector3(), path: [], homed: [],
    // The stand-in shot the path is flown with: what flightStep needs of a shot.
    shot: { mesh: { position: new THREE.Vector3() }, vel: new THREE.Vector3(), tool: null, flight: null, lock: undefined, homing: null },
    /** Colors the line's far end and the marker `hex`. */
    tint(hex) {
      if (this.tinted === hex) return;
      this.tinted = hex;
      FAR.set(hex);
      for (const m of ends) m.material.color.set(hex);
    },
    /** Lays the line along `path` and the marker at `landed`, facing out along `normal`, as seen from `eye`. */
    draw(path, landed, normal, eye) {
      // Resampled evenly, so the ribbon bends smoothly and its fade is even.
      const points = [path[0]];
      let next = spacing, total = 0;
      for (let i = 1; i < path.length && points.length < MOST; i++) {
        const a = path[i - 1], b = path[i], step = a.distanceTo(b);
        while (next <= total + step && points.length < MOST - 1) {
          points.push(a.clone().lerp(b, (next - total) / step));
          next += spacing;
        }
        total += step;
      }
      if (points.length < MOST && path.length > 1) points.push(path[path.length - 1]);
      const n = points.length, length = Math.max(1e-6, (n - 1) * spacing);
      for (let i = 0; i < n; i++) {
        const p = points[i];
        along.subVectors(points[Math.min(n - 1, i + 1)], points[Math.max(0, i - 1)]);
        toEye.subVectors(eye, p);
        side.crossVectors(along, toEye).normalize().multiplyScalar(WIDTH * toEye.length());
        const u = (i * spacing) / length;
        // In from nothing at the muzzle, and full from there to the marker it runs into.
        const alpha = STRENGTH * Math.min(1, i * spacing / fadeIn) ** 2;
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
