// Bugs on the buildings. Every finding a scanner reported walks a lap on the building
// it belongs to, colored by how serious it is; catching one with whatever tool is in
// your hands opens what was said about it.
//
// A lap is one of four: the street around the footprint, a band around the facade at
// some height, a circuit of the roof, or a wider circuit in the air above it. A beetle
// holds to whatever it is standing on, so one on a wall stands out of that wall
// sideways and one under an eave would hang from it - which is what makes a tall
// building look infested rather than decorated around the bottom. All four are the
// same loop of four corners; what changes is the plane it lies in, which way is up on
// it, and - for the one in the air - that nothing holds it there at all.
//
// The bugs live on the flat map, like the walker: MapScene bends what is drawn, so a
// bug's position here is a layout coordinate and nothing more.

import * as THREE from './vendor/three.module.min.js';
import { mergeGeometries } from './vendor/BufferGeometryUtils.js';
import { rankOf, severityColors } from './findings.js';
import { bugParts } from './models.js';

const MAX_BUGS = 140;     // a large repository reports thousands; the worst ones walk
const LANE = 0.3;         // how far outside a building's footprint its bugs patrol
const EAVE = 0.28;        // how far in from the roof's edge its circuit runs
const CLIMBABLE = 0.9;    // a building shorter than this has walls not worth walking
// The ones in the air: how far out they circle, how high above the roof, how much
// they rise and fall on the way round, how much faster they go than a walking bug,
// and how far they lean into the corners.
const AIR_OUT = 1.15, AIR_UP = 0.6, AIR_RISE = 0.4, AIR_SPEED = 2.1, BANK = 0.65;
// How high up the building a flyer holds its circuit, as a share of the height: one
// against the upper windows, one at the eaves, one over the roof. All at one height
// is a ring round the top rather than something flying about a building.
const AIR_AT = [0.55, 0.85, 1];
const BEAT = 34;          // wingbeats a second, which at this size is a blur
const WING = 0.15;        // how long a wing is, along the body
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
// The shell carries its shading in its vertex colors - the wing cases take the
// severity color, the head and the plate behind it stay nearly black - and the
// severity color is set per instance, so one mesh draws every color. Nothing in this
// scene is lit, so both the split and the roundness have to be painted: without them
// a beetle at arm's length is a colored blob.
const SHELL = '#151515'; // the legs, which are the same dark whatever the severity
const DARK = 0.06;       // how dark the head and its plate are, in linear light
let parts = null;        // the two geometries, once built
let fromModel = false;   // ... and whether they came out of bug.glb

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
    this.legs = null;       // ... the instanced legs, which rock on their own
    this.wings = null;      // ... and one pair of wings per bug that is in the air
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
   *
   * It follows while the bugs are out there, too: taking a finding from the panel in
   * the map view is the same catch as netting it in the street, and putting one back
   * puts its bug back on its lap.
   */
  keepCaught(ids) {
    this.caught = new Set(ids);
    for (const bug of this.bugs) bug.caught = this.caught.has(bug.f.id);
    this.dirty = true;
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

    // Which surface a bug gets is dealt from its place in its building's queue, and
    // most buildings have one or two findings - so without an offset per building the
    // first surface in the list would take nearly all of them. Each building starts
    // the deal one further along, which spreads the four kinds over the map instead
    // of over the few buildings with enough findings to get that far.
    const perBox = new Map();
    let nth = 0;
    for (const f of wanted) {
      const node = index.place(f);
      const box = boxFor(byNode, node);
      if (!box) continue;
      let seen = perBox.get(box);
      if (seen === undefined) perBox.set(box, (seen = nth++));
      else perBox.set(box, (seen += 1));
      const bug = this.spawn(f, node, box, seen);
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
    const lap = lapOf(box, surfaceFor(box, nth));
    const flying = lap.kind === 'air';
    const bug = {
      f, node, box, lap, flying,
      // Bugs on the same building start apart and walk at slightly different speeds,
      // so a busy block looks like traffic rather than a marching band.
      u: (nth * 0.37) % 1,
      speed: SPEED * (0.75 + ((nth * 0.19) % 0.5)) * (flying ? AIR_SPEED : 1),
      phase: nth * 1.7,
      caught: this.caught.has(f.id),
      // Which way is up for this bug: the way the surface it walks on faces, leaned
      // over into the corners if it is flying rather than standing on anything.
      up: new THREE.Vector3(0, 1, 0),
      wing: 0, // where in the beat its wings are, for the ones that have any
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
    parts = geometry();
    const shell = new THREE.InstancedMesh(parts.shell,
      this.scene.bendable(new THREE.MeshBasicMaterial({ vertexColors: true })), this.bugs.length);
    const legs = new THREE.InstancedMesh(parts.legs,
      this.scene.bendable(new THREE.MeshBasicMaterial({ color: SHELL })), this.bugs.length);
    // Two instances a flyer, one wing each: the geometry is the left wing, and the
    // right one is the same matrix mirrored across the body. Mirroring turns the
    // triangles inside out, which is what DoubleSide is here for.
    const flyers = this.bugs.filter(b => b.flying).length;
    const wings = new THREE.InstancedMesh(parts.wing,
      this.scene.bendable(new THREE.MeshBasicMaterial({
        color: '#dfe7f0', transparent: true, opacity: 0.4, side: THREE.DoubleSide, depthWrite: false,
      })), Math.max(1, flyers * 2));
    wings.renderOrder = 1; // over the shell it beats against, not fighting it for depth
    for (const mesh of [shell, legs, wings]) {
      mesh.frustumCulled = false;
      mesh.count = 0;
      this.group.add(mesh);
    }
    this.shell = shell;
    this.legs = legs;
    this.wings = wings;
    this.dirty = true;
  }

  update(dt, now) {
    if (!this.shell) return;
    const m = new THREE.Matrix4(), r = new THREE.Matrix4();
    const c = new THREE.Color();
    let n = 0, w = 0;
    for (const bug of this.bugs) {
      if (bug.caught) continue;
      bug.u = (bug.u + (bug.speed * dt) / bug.lap.length) % 1;
      this.moveTo(bug, now);
      orient(m, bug);
      this.shell.setMatrixAt(n, m);
      // The legs scurry: the whole set rocks, which reads as six legs at this size.
      // Tucked back, the same rock is a flyer holding them out of the way.
      this.legs.setMatrixAt(n, r.multiplyMatrices(m, rockAbout(bug.rock)));
      if (bug.flying) {
        // One beat, two wings: the same angle up on the left and down on the right of
        // a body whose x axis is its own, so they meet over its back and part under it.
        const beat = Math.sin(bug.wing) * 0.9 + 0.35;
        this.wings.setMatrixAt(w++, r.multiplyMatrices(m, flap(beat, 1)));
        this.wings.setMatrixAt(w++, r.multiplyMatrices(m, flap(beat, -1)));
      }
      if (this.dirty) this.shell.setColorAt(n, c.set(this.colors[bug.f.severity] || this.colors.unknown));
      n++;
    }
    this.shell.count = this.legs.count = n;
    this.wings.count = w;
    this.shell.instanceMatrix.needsUpdate = true;
    this.legs.instanceMatrix.needsUpdate = true;
    this.wings.instanceMatrix.needsUpdate = true;
    if (this.dirty && this.shell.instanceColor) this.shell.instanceColor.needsUpdate = true;
    this.dirty = false;
  }

  moveTo(bug, now) {
    const at = pointAt(bug.lap, bug.u);
    const t = now / 1000 + bug.phase;
    if (bug.flying) {
      // Nothing to stand on: it holds a height above the roof, rising and falling on
      // the way round, and leans into the corners the way anything that turns in the
      // air has to. The legs are tucked back and the wings do the work.
      bug.pos.set(at.x, at.y + AIR_UP + Math.sin(t * 1.5) * AIR_RISE, at.z);
      bug.heading = at.heading;
      bug.bank = at.turn * BANK;
      bug.up.set(0, 1, 0);
      bug.rock = -0.5;
      bug.wing = t * BEAT;
      return;
    }
    // The bob is along whatever the bug is standing on, so one on a wall bobs in and
    // out of it rather than up and down past it.
    const bob = 0.035 + Math.sin(t * 9) * 0.006;
    bug.pos.set(at.x + at.nx * bob, at.y + at.ny * bob, at.z + at.nz * bob);
    bug.heading = at.heading;
    bug.bank = 0;
    bug.up.set(at.nx, at.ny, at.nz);
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
    this.shell = this.legs = this.wings = null;
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

/**
 * Which surface a bug gets. They are dealt out in turn so that a building with
 * several findings is walked on at several heights rather than ringed at the bottom,
 * and a building too short or too small to have walls worth walking keeps them all
 * on the street.
 */
function surfaceFor(box, nth) {
  // Nothing has to hold a flying one up, so even a shed gets them; the walls and the
  // roof of a building too short or too narrow to be worth climbing do not.
  const air = () => ({ kind: 'air', at: AIR_AT[Math.floor(nth / 4) % AIR_AT.length] });
  if (!(box.h > CLIMBABLE) || box.w < 2 * EAVE + 0.4 || box.d < 2 * EAVE + 0.4) {
    return nth % 2 ? air() : { kind: 'street' };
  }
  switch (nth % 4) {
    case 3: return air();
    case 1: {
      // Spread up the facade rather than all at one height, and never at the very
      // top or the very bottom, where the wall runs into the roof or the ground.
      const bands = Math.max(1, Math.min(4, Math.floor(box.h / 1.2)));
      const band = (Math.floor(nth / 4) % bands) + 1;
      return { kind: 'wall', up: box.h * (band / (bands + 1)) };
    }
    case 2: return { kind: 'roof' };
    default: return { kind: 'street' };
  }
}

/**
 * lapOf is the loop a bug walks, as four corners in the plane of whatever it is
 * standing on, plus the direction that plane faces.
 *
 *   street  the footprint, one lane out, at the height the building stands on
 *   wall    the footprint itself at a height up the facade, each side facing out
 *   roof    the footprint drawn in from the edge, at the top, facing up
 *
 * A wall lap's four sides each face a different way, so the normal belongs to the
 * segment rather than to the lap; a street or roof lap has the same one throughout.
 */
function lapOf(box, surface) {
  const flat = surface.kind !== 'wall';
  const out = { street: LANE, roof: -EAVE, air: AIR_OUT, wall: 0 }[surface.kind];
  const hw = box.w / 2 + out, hd = box.d / 2 + out;
  const y = surface.kind === 'roof' ? box.y + box.h
    : surface.kind === 'air' ? box.y + box.h * surface.at
      : box.y;
  const corners = [
    [box.x - hw, box.z - hd], [box.x + hw, box.z - hd],
    [box.x + hw, box.z + hd], [box.x - hw, box.z + hd],
  ];
  const segments = [];
  let length = 0;
  for (let i = 0; i < corners.length; i++) {
    const [x0, z0] = corners[i], [x1, z1] = corners[(i + 1) % corners.length];
    const len = Math.hypot(x1 - x0, z1 - z0);
    // Walking the corners in this order keeps the building on the left, so the
    // outward normal is the walking direction turned right.
    const n = flat ? { nx: 0, ny: 1, nz: 0 }
      : { nx: (z1 - z0) / (len || 1), ny: 0, nz: -(x1 - x0) / (len || 1) };
    segments.push({ x0, z0, x1, z1, len, at: length, ...n });
    length += len;
  }
  // A wall lap sits on the footprint and a roof lap inside it; only the one in the
  // air reaches further out than the street. One box for the spatial index, wide
  // enough for whichever lap it holds.
  const bx = box.w / 2 + Math.max(LANE, out), bz = box.d / 2 + Math.max(LANE, out);
  return {
    segments, length: length || 1, y, kind: surface.kind,
    bounds: { x0: box.x - bx, x1: box.x + bx, z0: box.z - bz, z1: box.z + bz },
  };
}

// pointAt walks the lap: where the bug is at u in [0,1), which way it faces, and
// which way is up for it there.
function pointAt(lap, u) {
  const want = u * lap.length;
  let seg = lap.segments[lap.segments.length - 1];
  for (const s of lap.segments) {
    if (want < s.at + s.len) { seg = s; break; }
  }
  const t = seg.len ? (want - seg.at) / seg.len : 0;
  // How much of a corner the bug is in: the last fifth of a side and the first fifth
  // of the next one, which is what a flyer leans into. A lap is a rectangle, so every
  // corner turns the same way and the lean is always to the same side.
  const turn = Math.max(0, t - 0.8) / 0.2 + Math.max(0, 0.2 - t) / 0.2;
  return {
    x: seg.x0 + (seg.x1 - seg.x0) * t,
    y: lap.y,
    z: seg.z0 + (seg.z1 - seg.z0) * t,
    heading: Math.atan2(seg.x1 - seg.x0, seg.z1 - seg.z0),
    nx: seg.nx, ny: seg.ny, nz: seg.nz, turn: Math.min(1, turn),
  };
}

/**
 * The two geometries a bug is drawn from: the shell, which is one mesh painted in
 * two tones, and the legs, which are another so they can rock without the rest of it
 * following. Both stand on the origin with the nose along +z.
 *
 * The beetle is modelled in Blender (tools/bug.py) and arrives as bug.glb; until it
 * does - and if it never does - the map draws the one below out of spheres, so a bug
 * is on the street from the first frame.
 */
function geometry() {
  const model = bugParts();
  if (parts && fromModel === !!model) return parts;
  fromModel = !!model;
  parts = model ? modelled(model) : drawn();
  return parts;
}

// The model: its wing cases lit and tinted, its front end dark, its legs their own.
function modelled(model) {
  const shell = shade(model.get('shell').clone(), 1);
  const dark = shade(model.get('dark').clone(), DARK);
  return { shell: mergeGeometries([shell, dark]), legs: model.get('legs').clone(), wing: wing() };
}

// The stand-in: a squashed sphere with a smaller one in front and six sticks under it.
function drawn() {
  const body = shade(new THREE.SphereGeometry(BODY, 10, 7).scale(0.62, 0.48, 1).translate(0, BODY * 0.45, 0), 1);
  const head = shade(new THREE.SphereGeometry(BODY * 0.42, 8, 6).translate(0, BODY * 0.45, BODY * 0.82), DARK);
  const seam = shade(new THREE.BoxGeometry(0.01, 0.02, BODY * 1.4).translate(0, BODY * 0.86, -BODY * 0.12), DARK);
  const legs = [];
  for (let i = 0; i < LEGS; i++) {
    const side = i % 2 ? 1 : -1;
    legs.push(new THREE.BoxGeometry(BODY * 0.7, 0.012, 0.012)
      .rotateZ(side * 0.9)
      .translate(side * BODY * 0.55, 0.012, (Math.floor(i / 2) - 1) * BODY * 0.5));
  }
  return { shell: mergeGeometries([body, head, seam]), legs: mergeGeometries(legs), wing: wing() };
}

/**
 * The left wing, hinged on the body's own length so that turning it about that line
 * beats it (flap above). Flat, because a wing at thirty-odd beats a second is a blur
 * whatever shape it is - what carries is the outline and the way it sweeps back.
 *
 * It is built lying along the body from the hinge outwards, which puts its root at
 * the origin: the same matrix that places the beetle places this.
 */
function wing() {
  const w = WING, root = 0.018;
  const shape = new THREE.Shape();
  shape.moveTo(root, 0.02);
  shape.quadraticCurveTo(w * 0.55, 0.075, w, -0.01);   // the leading edge, curving out
  shape.quadraticCurveTo(w * 0.6, -0.06, root, -0.045); // and the trailing edge back in
  shape.lineTo(root, 0.02);
  const geo = new THREE.ShapeGeometry(shape, 8);
  // Lying flat over the back, hinged where it meets the body rather than at its tip.
  geo.rotateX(-Math.PI / 2);
  geo.translate(0, BODY * 0.55, -BODY * 0.1);
  return geo;
}

/**
 * Paints a geometry's roundness into its vertex colors, around a base tone the
 * instance color then tints: the wing cases take the severity color and the front
 * end a dark share of it.
 *
 * Nothing in this scene is lit, so a surface that is not painted is flat, and a
 * beetle's shell is the one thing about it that has to look curved. Brightest where
 * it faces up, a little brighter on one side than the other, which is the same
 * reckoning city.js makes for a tree.
 */
function shade(geo, base) {
  if (geo.index) geo = geo.toNonIndexed();
  geo.computeVertexNormals();
  const n = geo.getAttribute('normal');
  const c = new Float32Array(n.count * 3);
  for (let i = 0; i < n.count; i++) {
    const k = base * (0.62 + 0.38 * Math.max(0, n.getY(i)) + 0.1 * n.getX(i) - 0.05 * n.getZ(i));
    c[i * 3] = c[i * 3 + 1] = c[i * 3 + 2] = k;
  }
  geo.setAttribute('color', new THREE.BufferAttribute(c, 3));
  geo.deleteAttribute('normal'); // an unlit material never reads it
  return geo;
}

const rock = new THREE.Matrix4(), beat = new THREE.Matrix4();
const fwd = new THREE.Vector3(), side = new THREE.Vector3(), lean = new THREE.Vector3();

/**
 * Stands a bug on its surface: nose along the lap, back along the surface's normal.
 *
 * The model's nose is +z and its back +y, so the matrix's z column is the way it is
 * walking and its y column the way the wall or the roof faces; the x column is what
 * is left, which is the cross product of those two and keeps the beetle from being
 * mirrored. On the street this comes out as the plain turn about the vertical it
 * used to be.
 *
 * A flyer has nothing to stand on, so its "up" is rolled about the way it is going -
 * the lean into a corner, which is the whole difference between a beetle in the air
 * and a beetle sliding round one.
 */
function orient(m, bug) {
  fwd.set(Math.sin(bug.heading), 0, Math.cos(bug.heading));
  const up = bug.bank ? lean.copy(bug.up).applyAxisAngle(fwd, bug.bank) : bug.up;
  side.crossVectors(up, fwd).normalize();
  m.makeBasis(side, up, fwd);
  m.setPosition(bug.pos);
}

/**
 * One wing at an angle, as a matrix to hang off the bug's own: a turn about the
 * length of the body, mirrored for the other side. Mirroring is a negative x scale,
 * which is why the wings are drawn with both faces.
 */
function flap(angle, side) {
  beat.makeRotationZ(angle * side);
  if (side < 0) beat.scale(MIRROR);
  return beat;
}

// The legs' rocking, as a matrix to hang off the bug's own.
function rockAbout(angle) {
  return rock.makeRotationX(angle);
}

const MIRROR = new THREE.Vector3(-1, 1, 1);
