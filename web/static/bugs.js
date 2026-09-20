// Bugs on the streets. Every finding a scanner reported walks a lap around the
// building it belongs to, colored by how serious it is; catching one with whatever
// tool is in your hands opens what was said about it.
//
// The bugs live on the flat map, like the walker: MapScene bends what is drawn, so a
// bug's position here is a layout coordinate and nothing more.

import * as THREE from './vendor/three.module.min.js';
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

// Geometry is shared by every bug; only the shell's color and the transform differ.
const SHELL = '#151515'; // head, legs and the split down the shell
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
    this.materials = new Map();
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
    this.group.clear();
    this.bugs = [];
    this.grid.clear();
    for (const m of this.materials.values()) m.dispose();
    this.materials.clear();
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
      mesh: null,
    };
    if (!bug.caught) {
      bug.mesh = this.mesh(f.severity);
      this.group.add(bug.mesh);
      this.moveTo(bug, 0);
    }
    return bug;
  }

  mesh(severity) {
    parts ||= geometry();
    const g = new THREE.Group();
    g.add(new THREE.Mesh(parts.body, this.material(this.colors[severity] || this.colors.unknown)));
    g.add(new THREE.Mesh(parts.head, this.material(SHELL)));
    // Nothing in this scene is lit, so the shell's split is drawn rather than shaded:
    // without it a beetle at arm's length is a colored blob.
    g.add(new THREE.Mesh(parts.seam, this.material(SHELL)));
    const legs = new THREE.Group();
    for (let i = 0; i < LEGS; i++) {
      const leg = new THREE.Mesh(parts.leg, this.material(SHELL));
      const side = i % 2 ? 1 : -1;
      leg.position.set(side * BODY * 0.55, 0.012, (Math.floor(i / 2) - 1) * BODY * 0.5);
      leg.rotation.z = side * 0.9;
      legs.add(leg);
    }
    g.add(legs);
    g.userData.legs = legs;
    g.frustumCulled = false;
    return g;
  }

  // One material per color: six colors for however many bugs there are.
  material(color) {
    let m = this.materials.get(color);
    if (!m) {
      m = this.scene.bendable(new THREE.MeshBasicMaterial({ color }));
      this.materials.set(color, m);
    }
    return m;
  }

  update(dt, now) {
    for (const bug of this.bugs) {
      if (bug.caught) continue;
      bug.u = (bug.u + (bug.speed * dt) / bug.lap.length) % 1;
      this.moveTo(bug, now);
    }
  }

  moveTo(bug, now) {
    const m = bug.mesh;
    if (!m) return;
    const { x, z, heading } = pointAt(bug.lap, bug.u);
    const t = now / 1000 + bug.phase;
    m.position.set(x, bug.lap.y + 0.035 + Math.sin(t * 9) * 0.006, z);
    m.rotation.y = heading;
    // The legs scurry: the whole set rocks, which reads as six legs at this size.
    m.userData.legs.rotation.x = Math.sin(t * 16) * 0.35;
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
      if (bug.caught || !bug.mesh) continue;
      const m = bug.mesh.position;
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
    if (bug.mesh) {
      this.group.remove(bug.mesh);
      bug.mesh = null;
    }
    this.hooks.onCatch?.(bug.f, bug.node);
    return true;
  }

  dispose() {
    this.group.clear();
    this.grid.clear();
    this.scene.scene.remove(this.group);
    for (const m of this.materials.values()) m.dispose();
    this.materials.clear();
    this.bugs = [];
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

// A beetle: a rounded shell, a dark head in front and six legs under it, nose along +z.
function geometry() {
  return {
    body: new THREE.SphereGeometry(BODY, 10, 7).scale(0.62, 0.48, 1).translate(0, BODY * 0.45, 0),
    head: new THREE.SphereGeometry(BODY * 0.42, 8, 6).translate(0, BODY * 0.45, BODY * 0.82),
    leg: new THREE.BoxGeometry(BODY * 0.7, 0.012, 0.012),
    seam: new THREE.BoxGeometry(0.01, 0.02, BODY * 1.4).translate(0, BODY * 0.86, -BODY * 0.12),
  };
}
