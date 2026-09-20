// Bugs on the streets. Every finding a scanner reported walks a lap around the
// building it belongs to, colored by how serious it is; catching one with whatever
// tool is in your hands opens what was said about it.
//
// The bugs live on the flat map, like the walker: MapScene bends what is drawn, so a
// bug's position here is a layout coordinate and nothing more.

import * as THREE from './vendor/three.module.min.js';
import { mergeGeometries } from './vendor/BufferGeometryUtils.js';
import { rankOf, severityColors } from './findings.js';

const MAX_BUGS = 140;     // a large repository reports thousands; the worst ones walk
const LANE = 0.3;         // how far outside a building's footprint its bugs patrol
const SPEED = 0.42;       // units per second along the lap - a walking pace, catchable
const CATCH = 0.42;       // how close a shot has to pass
const CELL = 4;           // spatial grid for finding the bug under a ray sample
// The crosshair widens its search with distance (walk.js), so a bug is indexed into
// every cell within this much of its lap - and no search may look farther.
const MARGIN = 1.5;
const BODY = 0.1;         // half the length of a bug's body
const LEGS = 6;

// Every bug is drawn from the same two geometries, as two instanced meshes: the
// shell (body, head and the split down it) and the legs, which rock on their own.
// A hundred and forty beetles were a thousand draw calls as separate meshes, which
// is most of a frame in walk mode; they are two now, whatever the count.
//
// The shell carries its shading in its vertex colors - white over the wing cases,
// dark over the head and the seam - and the severity color is set per instance, so
// one mesh draws every color. Nothing in this scene is lit, so that split has to be
// painted: without it a beetle at arm's length is a colored blob.
const SHELL = '#151515'; // the legs, which are the same dark whatever the severity
const DARK = 0.05;       // how dark the head and the seam are, in linear light
let parts = null;

export class Bugs {
  /** hooks: { onCatch(finding, node) } */
  constructor(scene, hooks = {}) {
    this.scene = scene;
    this.hooks = hooks;
    this.group = new THREE.Group();
    this.group.visible = false;
    this.bugs = [];
    this.grid = new Map();   // cell -> bugs whose lap passes through it
    this.caught = new Set(); // finding ids, so a relayout does not revive them
    this.colors = {};
    this.shell = null;      // the instanced bodies
    this.legs = null;       // ... and the instanced legs, which rock on their own
    this.dirty = false;     // the instance colors need writing again
    scene.scene.add(this.group);
  }

  /** The total on the map and how many have been caught, for the HUD. */
  get counts() {
    return { total: this.bugs.length, caught: this.bugs.filter(b => b.caught).length };
  }

  show(on) { this.group.visible = on; }

  /**
   * The findings already caught, which stay caught. The backpack is what remembers
   * them - across a relayout, a depth change and a reload - so it says which they are
   * and this follows.
   */
  keepCaught(ids) {
    this.caught = new Set(ids);
  }

  /**
   * place puts a bug on the streets for every finding that belongs to a building on
   * the map. index is what findings.js built; boxes are the current layout's.
   */
  place(index, boxes) {
    this.drop();
    this.bugs = [];
    this.grid.clear();
    if (!index || !boxes?.length) return;
    this.colors = severityColors();

    const byNode = new Map();
    for (const b of boxes) {
      if (b.kind !== 'land' && b.node) byNode.set(b.node.id, b);
    }
    // The findings arrive worst first; a map that cannot show them all shows those.
    const wanted = [...index.all]
      .sort((a, b) => rankOf(b.severity) - rankOf(a.severity))
      .slice(0, MAX_BUGS);

    const perBox = new Map();
    for (const f of wanted) {
      const node = index.place(f);
      const box = boxFor(byNode, node);
      if (!box) continue;
      const n = perBox.get(box) || 0;
      perBox.set(box, n + 1);
      const bug = this.spawn(f, node, box, n);
      this.bugs.push(bug);
      this.index(bug);
    }
    if (this.bugs.length) this.build();
  }

  // A bug never leaves its lap, so one indexing at spawn is enough: the crosshair
  // tests a handful of candidates per ray sample instead of every bug on the map.
  index(bug) {
    const { x0, x1, z0, z1 } = bug.lap.bounds;
    for (let x = Math.floor((x0 - MARGIN) / CELL); x <= Math.floor((x1 + MARGIN) / CELL); x++) {
      for (let z = Math.floor((z0 - MARGIN) / CELL); z <= Math.floor((z1 + MARGIN) / CELL); z++) {
        const k = x + ',' + z;
        const cell = this.grid.get(k);
        if (cell) cell.push(bug); else this.grid.set(k, [bug]);
      }
    }
  }

  spawn(f, node, box, nth) {
    const lap = lapOf(box);
    const bug = {
      f, node, box, lap,
      // Bugs on the same building start apart and walk at slightly different speeds,
      // so a busy block looks like traffic rather than a marching band.
      u: (nth * 0.37) % 1,
      speed: SPEED * (0.75 + ((nth * 0.19) % 0.5)),
      phase: nth * 1.7,
      caught: this.caught.has(f.id),
      // Where it is on the flat map: what the crosshair and everything thrown at it
      // measure against, kept here rather than read back off a shared mesh.
      pos: new THREE.Vector3(),
      heading: 0,
      rock: 0,
    };
    this.moveTo(bug, 0);
    return bug;
  }

  /**
   * The two instanced meshes, sized for the bugs that are out there. They are rebuilt
   * with the layout rather than resized, because a layout is where the count changes.
   */
  build() {
    parts ||= geometry();
    const shell = new THREE.InstancedMesh(parts.shell,
      this.scene.bendable(new THREE.MeshBasicMaterial({ vertexColors: true })), this.bugs.length);
    const legs = new THREE.InstancedMesh(parts.legs,
      this.scene.bendable(new THREE.MeshBasicMaterial({ color: SHELL })), this.bugs.length);
    for (const mesh of [shell, legs]) {
      mesh.frustumCulled = false;
      mesh.count = 0;
      this.group.add(mesh);
    }
    this.shell = shell;
    this.legs = legs;
    this.dirty = true;
  }

  update(dt, now) {
    if (!this.shell) return;
    const m = new THREE.Matrix4(), r = new THREE.Matrix4(), q = new THREE.Quaternion();
    const p = new THREE.Vector3(), one = new THREE.Vector3(1, 1, 1);
    const c = new THREE.Color();
    let n = 0;
    for (const bug of this.bugs) {
      if (bug.caught) continue;
      bug.u = (bug.u + (bug.speed * dt) / bug.lap.length) % 1;
      this.moveTo(bug, now);
      m.compose(p.copy(bug.pos), q.setFromAxisAngle(UP, bug.heading), one);
      this.shell.setMatrixAt(n, m);
      // The legs scurry: the whole set rocks, which reads as six legs at this size.
      this.legs.setMatrixAt(n, r.multiplyMatrices(m, rockAbout(bug.rock)));
      if (this.dirty) this.shell.setColorAt(n, c.set(this.colors[bug.f.severity] || this.colors.unknown));
      n++;
    }
    this.shell.count = this.legs.count = n;
    this.shell.instanceMatrix.needsUpdate = true;
    this.legs.instanceMatrix.needsUpdate = true;
    if (this.dirty && this.shell.instanceColor) this.shell.instanceColor.needsUpdate = true;
    this.dirty = false;
  }

  moveTo(bug, now) {
    const { x, z, heading } = pointAt(bug.lap, bug.u);
    const t = now / 1000 + bug.phase;
    bug.pos.set(x, bug.lap.y + 0.035 + Math.sin(t * 9) * 0.006, z);
    bug.heading = heading;
    bug.rock = Math.sin(t * 16) * 0.35;
  }

  /**
   * at is the bug nearest to a point on the flat map, within radius; used both by the
   * crosshair and by whatever the tool threw.
   */
  at(v, radius = CATCH) {
    const cell = this.grid.get(Math.floor(v.x / CELL) + ',' + Math.floor(v.z / CELL));
    if (!cell) return null;
    const r = Math.min(radius, MARGIN); // past this a bug may sit in the next cell
    let best = null, bestSq = r * r;
    for (const bug of cell) {
      if (bug.caught) continue;
      const m = bug.pos;
      const dSq = (m.x - v.x) ** 2 + (m.y - v.y) ** 2 + (m.z - v.z) ** 2;
      if (dSq < bestSq) { best = bug; bestSq = dSq; }
    }
    return best;
  }

  /** Catch one: it stops walking, and what it was carrying is read out. */
  catch(bug) {
    if (!bug || bug.caught) return false;
    bug.caught = true;
    this.caught.add(bug.f.id);
    this.dirty = true; // the instances close up over the gap it leaves
    this.hooks.onCatch?.(bug.f, bug.node);
    return true;
  }

  dispose() {
    this.drop();
    this.grid.clear();
    this.scene.scene.remove(this.group);
    this.bugs = [];
  }

  /** Lets go of the meshes and their materials; the geometry is shared and stays. */
  drop() {
    for (const mesh of this.group.children) {
      mesh.material.dispose();
      mesh.dispose();
    }
    this.group.clear();
    this.shell = this.legs = null;
  }
}

// boxFor finds the building a finding's node is drawn as. A file inside a collapsed
// directory has no box of its own, so the nearest ancestor that does takes its bug.
// pins.js places its markers by the same rule, so a bug and its pin agree.
export function boxFor(byNode, node) {
  for (let n = node; n; n = n.parentNode) {
    const b = byNode.get(n.id);
    if (b) return b;
  }
  return null;
}

// lapOf is the loop a bug walks: the building's footprint, one lane out, at the height
// its base stands on.
function lapOf(box) {
  const hw = box.w / 2 + LANE, hd = box.d / 2 + LANE;
  const corners = [
    [box.x - hw, box.z - hd], [box.x + hw, box.z - hd],
    [box.x + hw, box.z + hd], [box.x - hw, box.z + hd],
  ];
  const segments = [];
  let length = 0;
  for (let i = 0; i < corners.length; i++) {
    const [x0, z0] = corners[i], [x1, z1] = corners[(i + 1) % corners.length];
    const len = Math.hypot(x1 - x0, z1 - z0);
    segments.push({ x0, z0, x1, z1, len, at: length });
    length += len;
  }
  return {
    segments, length: length || 1, y: box.y,
    bounds: { x0: box.x - hw, x1: box.x + hw, z0: box.z - hd, z1: box.z + hd },
  };
}

// pointAt walks the lap: where the bug is at u in [0,1), and which way it faces.
function pointAt(lap, u) {
  const want = u * lap.length;
  let seg = lap.segments[lap.segments.length - 1];
  for (const s of lap.segments) {
    if (want < s.at + s.len) { seg = s; break; }
  }
  const t = seg.len ? (want - seg.at) / seg.len : 0;
  return {
    x: seg.x0 + (seg.x1 - seg.x0) * t,
    z: seg.z0 + (seg.z1 - seg.z0) * t,
    heading: Math.atan2(seg.x1 - seg.x0, seg.z1 - seg.z0),
  };
}

// A beetle: a rounded shell, a dark head in front and six legs under it, nose along
// +z. The shell is one geometry painted in two tones; the legs are another, so they
// can rock without the rest of it following.
function geometry() {
  const body = paint(new THREE.SphereGeometry(BODY, 10, 7).scale(0.62, 0.48, 1).translate(0, BODY * 0.45, 0), 1);
  const head = paint(new THREE.SphereGeometry(BODY * 0.42, 8, 6).translate(0, BODY * 0.45, BODY * 0.82), DARK);
  const seam = paint(new THREE.BoxGeometry(0.01, 0.02, BODY * 1.4).translate(0, BODY * 0.86, -BODY * 0.12), DARK);
  const legs = [];
  for (let i = 0; i < LEGS; i++) {
    const side = i % 2 ? 1 : -1;
    legs.push(new THREE.BoxGeometry(BODY * 0.7, 0.012, 0.012)
      .rotateZ(side * 0.9)
      .translate(side * BODY * 0.55, 0.012, (Math.floor(i / 2) - 1) * BODY * 0.5));
  }
  return { shell: mergeGeometries([body, head, seam]), legs: mergeGeometries(legs) };
}

// paint gives a geometry a flat vertex color, which the instance color then tints:
// the wing cases take the severity color and the head and seam a dark share of it.
function paint(geo, k) {
  const n = geo.getAttribute('position').count;
  const c = new Float32Array(n * 3).fill(k);
  geo.setAttribute('color', new THREE.BufferAttribute(c, 3));
  return geo;
}

const UP = new THREE.Vector3(0, 1, 0);
const rock = new THREE.Matrix4();

// The legs' rocking, as a matrix to hang off the bug's own.
function rockAbout(angle) {
  return rock.makeRotationX(angle);
}
