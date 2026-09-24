// Bugs on the buildings. Every finding a scanner reported walks a lap on the building
// it belongs to, colored by how serious it is and shaped by it too; catching one with
// whatever primary tool is in your hands opens what was said about it.
//
// Shape carries the severity as well as color does, and carries it further: a color
// is no use in a crowd, in the dark, or from behind, and it is the one thing the
// tracker in the corner already says. So a critical finding is a caterpillar the
// length of a doorstep, hauling itself along the street; an ordinary one is the
// beetle; and the notes and the nits are mites you have to go and look for. They walk
// the same laps and are caught the same way - only the thing you are looking at
// changes, which is what makes a bad street legible from the end of it.
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
import { reachable } from './fires.js';

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
// One number per cell rather than "gx,gz": the crosshair asks a few hundred times a
// frame, and a key built by concatenation is a string allocated for each of them.
const cellOf = (gx, gz) => (gx + 32768) * 65536 + (gz + 32768);
// The crosshair widens its search with distance (walk.js), so a bug is indexed into
// every cell within this much of its lap - and no search may look farther.
const MARGIN = 1.5;
const BODY = 0.1;         // half the length of a bug's body
const LEGS = 6;
// How long a bug takes to leave the map once it has been caught.
const TAKE = 0.9;
// The same, in milliseconds and out loud, because the catch is not only an animation:
// app.js waits it out before it opens the details, so that what a walker sees is the
// bug coming to them and then the reading, rather than the reading over the top of a
// bug they never saw arrive.
export const TAKE_MS = TAKE * 1000;

/**
 * What being caught looks like, by the tool that did it. A catch that simply blinks
 * out says nothing about what took it, and the tools are the whole character of walk
 * mode - so each one carries its own gesture through to the thing it acted on.
 *
 * Each returns where the bug has got to a share `t` of the way through: `lift` along
 * whatever it was standing on, `toward` a share of the way to wherever it is being
 * taken, `shake` along the same axis as `lift` but added after that move, `size` and
 * `squash` (the up axis alone), and `spin` about its own back. `bubble` asks for the
 * soap film to be drawn around it.
 */
const TAKES = {
  // Sealed in a bubble that carries it off, turning slowly as it goes, until the soap
  // swells and the two of them go together.
  bubble: t => {
    const going = Math.max(0, (t - 0.6) / 0.4);
    return {
      lift: 1.7 * t * t,
      size: 1 - going * going,
      spin: t * 2.2,
      bubble: (0.1 + 0.16 * Math.min(1, t * 3)) * (1 + going * 0.6),
    };
  },
  // Reeled in: it comes down the line at the walker and is gone in the hand.
  reel: t => ({ toward: t * t, size: 1 - t * 0.85, spin: t * 16 }),
  // Scooped. The hoop passes over it and it goes with the hoop - which is why this is
  // the one catch drawn to the tool rather than to the walker: a bug that flew off
  // towards somebody's chest while the net went the other way was the whole trouble
  // with this one. Inside it, it fights: it bounces off the netting, turns itself
  // over and is squashed against the mesh, all of it fading as it tires, and only
  // then is it tipped out of the world. Full size until well past the middle, because
  // a bug that shrinks on the way in is a bug nobody saw caught.
  net: t => {
    const held = smooth(Math.min(1, t * 4.5)); // how far into the hoop it has got
    const gone = Math.max(0, (t - 0.62) / 0.38);
    const fight = held * (1 - gone);
    return {
      lift: 0.04,
      toward: held * 0.97,
      shake: Math.sin(t * 34) * 0.045 * fight,
      size: 1 - gone * gone,
      squash: 1 - 0.2 * fight - 0.12 * Math.sin(t * 40) * fight,
      spin: 13 + Math.sin(t * 46) * 11,
    };
  },
  // Pinned: driven down onto whatever it was standing on, and flattened there.
  pin: t => ({ lift: -0.05 * ramp(t), squash: 1 - 0.8 * ramp(t), size: 1 - 2 * Math.max(0, t - 0.5) }),
  // Foamed: it sags, shudders, settles and is buried.
  foam: t => ({ lift: -0.1 * t, size: 1 - t * t, spin: Math.sin(t * 26) * 0.35 }),
  // Photographed, which at this size is a blink and an empty street.
  flash: t => ({ size: Math.max(0, 1 - t * 1.9), spin: t * 4 }),
};
const ramp = t => Math.min(1, t * 4); // the part of a take that lands at once
const smooth = t => t * t * (3 - 2 * t); // ... and one that leaves and arrives gently
// The catches that carry a bug in the tool rather than back to the walker.
const IN_HAND = new Set(['net']);
const TAKEN = '#cfe6ff';              // the soap around a bubbled one

/**
 * What each severity walks as: which of the three shapes, how big it is drawn, and
 * whether it can take to the air. A caterpillar cannot - it is the one that has to be
 * walked up to, which is the right way round for the worst thing on the map.
 */
const SHAPES = {
  critical: { shape: 'grub', scale: 1, flies: false },
  high: { shape: 'beetle', scale: 1.3, flies: true },
  medium: { shape: 'beetle', scale: 1, flies: true },
  low: { shape: 'mite', scale: 0.85, flies: true },
  info: { shape: 'mite', scale: 0.7, flies: true },
  unknown: { shape: 'mite', scale: 0.9, flies: true },
};
const shapeOf = severity => SHAPES[severity] || SHAPES.unknown;

// Each shape is drawn from two geometries, as two instanced meshes: the shell (body,
// head and the split down it) and the legs, which rock on their own, with a third for
// the wings of the shapes that fly. A hundred and forty beetles were a thousand draw
// calls as separate meshes, which is most of a frame in walk mode; they are a handful
// now, whatever the count, and a map showing all three shapes at once costs seven.
//
// The shell carries its shading in its vertex colors - the body takes the severity
// color, the head and the plate behind it stay nearly black - and both the severity
// color and the size are set per instance, so one mesh draws every color a shape
// comes in and every size of it. Nothing in this scene is lit, so both the split and
// the roundness have to be painted: without them a beetle at arm's length is a
// colored blob.
const SHELL = '#151515'; // the legs, which are the same dark whatever the severity
const DARK = 0.06;       // how dark the head and its plate are, in linear light
let parts = null;        // the geometries of every shape, once built
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
    // One set of instanced meshes per shape that is out there: the bodies, the legs
    // (which rock on their own) and, for the shapes that fly, a pair of wings each.
    this.drawn = [];
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
    // A vulnerability somebody can actually reach is not a bug: it burns (fires.js),
    // and a finding that did both would be answered twice - netted on the street and
    // still alight on the roof above. The split is the point of the analogy. Most
    // advisories against a lock file are against code nothing here calls, and those
    // are exactly the ones worth walking up to and catching when you get to them.
    //
    // The findings arrive worst first; a map that cannot show them all shows those.
    const wanted = [...index.all]
      .filter(f => !reachable(f))
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
        const k = cellOf(x, z);
        const cell = this.grid.get(k);
        if (cell) cell.push(bug); else this.grid.set(k, [bug]);
      }
    }
  }

  spawn(f, node, box, nth) {
    const kind = shapeOf(f.severity);
    const lap = lapOf(box, surfaceFor(box, nth, kind.flies));
    const flying = lap.kind === 'air';
    const bug = {
      f, node, box, lap, flying,
      shape: kind.shape,
      scale: kind.scale,
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
   * The instanced meshes, sized for the bugs that are out there: three a shape at
   * most, and only for the shapes any finding actually called for. They are rebuilt
   * with the layout rather than resized, because a layout is where the count changes.
   */
  build() {
    parts = geometry();
    this.drawn = [];
    const byShape = new Map();
    for (const bug of this.bugs) {
      const list = byShape.get(bug.shape);
      if (list) list.push(bug); else byShape.set(bug.shape, [bug]);
    }
    for (const [name, bugs] of byShape) {
      const shape = parts.get(name);
      const shell = new THREE.InstancedMesh(shape.shell,
        this.scene.bendable(new THREE.MeshBasicMaterial({ vertexColors: true })), bugs.length);
      const legs = new THREE.InstancedMesh(shape.legs,
        this.scene.bendable(new THREE.MeshBasicMaterial({ color: SHELL })), bugs.length);
      const meshes = [shell, legs];
      // Two instances a flyer, one wing each: the geometry is the left wing, and the
      // right one is the same matrix mirrored across the body. Mirroring turns the
      // triangles inside out, which is what DoubleSide is here for.
      const flyers = bugs.filter(b => b.flying).length;
      let wings = null;
      if (shape.wing && flyers) {
        wings = new THREE.InstancedMesh(shape.wing,
          this.scene.bendable(new THREE.MeshBasicMaterial({
            color: '#dfe7f0', transparent: true, opacity: 0.4, side: THREE.DoubleSide, depthWrite: false,
          })), flyers * 2);
        wings.renderOrder = 1; // over the shell it beats against, not fighting it for depth
        meshes.push(wings);
      }
      for (const mesh of meshes) {
        mesh.frustumCulled = false;
        mesh.count = 0;
        this.group.add(mesh);
      }
      this.drawn.push({ bugs, shell, legs, wings });
    }
    this.dirty = true;
  }

  /**
   * A frame. `at` is where the walker is and `hand` where the tool that is catching
   * them holds what it catches; the takes that draw a bug somewhere need one or the
   * other, and without either they simply shrink where they stand.
   */
  update(dt, now, at = null, hand = null) {
    if (!this.drawn.length) return;
    const m = new THREE.Matrix4(), r = new THREE.Matrix4();
    const c = new THREE.Color();
    for (const { bugs, shell, legs, wings } of this.drawn) {
      let n = 0, w = 0;
      for (const bug of bugs) {
        if (bug.caught && !bug.take) continue;
        if (bug.take && !this.taking(bug, dt, at, hand)) continue;
        if (!bug.take) {
          bug.u = (bug.u + (bug.speed * dt) / bug.lap.length) % 1;
          this.moveTo(bug, now);
        }
        orient(m, bug);
        shell.setMatrixAt(n, m);
        // The legs scurry: the whole set rocks, which reads as legs at this size.
        // Tucked back, the same rock is a flyer holding them out of the way.
        legs.setMatrixAt(n, r.multiplyMatrices(m, rockAbout(bug.rock)));
        if (bug.flying && wings && !bug.take) {
          // One beat, two wings: the same angle up on the left and down on the right
          // of a body whose x axis is its own, so they meet over its back and part
          // under it.
          const beat = Math.sin(bug.wing) * 0.9 + 0.35;
          wings.setMatrixAt(w++, r.multiplyMatrices(m, flap(beat, 1)));
          wings.setMatrixAt(w++, r.multiplyMatrices(m, flap(beat, -1)));
        }
        if (this.dirty) shell.setColorAt(n, c.set(this.colors[bug.f.severity] || this.colors.unknown));
        n++;
      }
      shell.count = legs.count = n;
      shell.instanceMatrix.needsUpdate = true;
      legs.instanceMatrix.needsUpdate = true;
      if (wings) {
        wings.count = w;
        wings.instanceMatrix.needsUpdate = true;
      }
      if (this.dirty && shell.instanceColor) shell.instanceColor.needsUpdate = true;
    }
    this.dirty = false;
  }

  /**
   * One frame of a bug being taken off the map. Moves it along whatever its tool does
   * to it and says whether it is still there to be drawn; the last frame lets go of
   * the soap, if there was any, and closes the instances up over the gap.
   */
  taking(bug, dt, at, hand = null) {
    const take = bug.take;
    take.t += dt / TAKE;
    if (take.t >= 1) {
      this.unbubble(bug);
      bug.take = null;
      this.dirty = true;
      return false;
    }
    const to = TAKES[take.how](take.t);
    // Where it is being taken: the hoop of the net carries what it catches, so a
    // netted one homes on the tool; everything else comes to the walker.
    const home = (IN_HAND.has(take.how) ? hand : null) || at;
    bug.pos.copy(take.at).addScaledVector(take.up, to.lift || 0);
    if (to.toward && home) bug.pos.lerp(home, to.toward);
    // The struggle is added after the move rather than before it: almost all of the
    // way to the hoop, anything mixed in beforehand would be damped to nothing.
    if (to.shake) bug.pos.addScaledVector(take.up, to.shake);
    bug.size = Math.max(0, to.size ?? 1);
    bug.squash = to.squash ?? 1;
    bug.heading += (to.spin || 0) * dt;
    bug.rock = 0;
    if (to.bubble) this.bubble(bug, to.bubble); else this.unbubble(bug);
    return bug.size > 0.01;
  }

  /** The soap around a bubbled one, made when it is first wanted and kept until it pops. */
  bubble(bug, r) {
    if (!bug.soap) {
      soapParts ||= [
        new THREE.SphereGeometry(1, 14, 10),
        this.scene.bendable(new THREE.MeshBasicMaterial({
          color: TAKEN, transparent: true, opacity: 0.32, depthWrite: false,
        })),
      ];
      bug.soap = new THREE.Mesh(soapParts[0], soapParts[1]);
      bug.soap.frustumCulled = false;
      bug.soap.renderOrder = 2;
      this.group.add(bug.soap);
    }
    bug.soap.position.copy(bug.pos);
    bug.soap.scale.setScalar(r);
  }

  unbubble(bug) {
    if (!bug.soap) return;
    this.group.remove(bug.soap);
    bug.soap = null;
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
    const cell = this.grid.get(cellOf(Math.floor(v.x / CELL), Math.floor(v.z / CELL)));
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

  /**
   * Catch one: it stops walking, and what it was carrying is read out. `how` is the
   * tool's own gesture (TAKES), carried through to the bug so that netting one and
   * photographing one do not look the same; a catch with no gesture named - taking a
   * finding from the panel, over on the map - simply stops it where it stands.
   */
  catch(bug, how = null) {
    if (!bug || bug.caught) return false;
    bug.caught = true;
    if (TAKES[how]) bug.take = { how, t: 0, at: bug.pos.clone(), up: bug.up.clone() };
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
    for (const bug of this.bugs) {
      bug.soap = null; // the group is about to be emptied; the material is shared
      bug.take = null;
    }
    for (const mesh of this.group.children) {
      mesh.material?.dispose?.();
      mesh.dispose?.();
    }
    this.group.clear();
    this.drawn = [];
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
function surfaceFor(box, nth, flies) {
  // Nothing has to hold a flying one up, so even a shed gets them; the walls and the
  // roof of a building too short or too narrow to be worth climbing do not. A shape
  // with no wings takes the street instead of the air, wherever the deal lands.
  const air = () => (flies ? { kind: 'air', at: AIR_AT[Math.floor(nth / 4) % AIR_AT.length] } : { kind: 'street' });
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
 * The geometries every shape is drawn from, by name: a shell, which is one mesh
 * painted in two tones, and the legs, which are another so they can rock without the
 * rest of it following, and for a shape that flies a wing as well. All of them stand
 * on the origin with the nose along +z.
 *
 * They are modelled in Blender (tools/bug.py) and arrive as bug.glb; until they do -
 * and for any shape an older file was built without - the map draws the ones below
 * out of spheres and boxes, so the street is never empty waiting on a download.
 */
function geometry() {
  const model = bugParts();
  if (parts && fromModel === !!model) return parts;
  fromModel = !!model;
  parts = new Map([
    ['beetle', beetle(model)],
    ['grub', grub(model)],
    ['mite', mite(model)],
  ]);
  return parts;
}

/**
 * One shape out of the model file: `names` are the meshes bug.py exports for it -
 * shell, dark front end, legs, and the flight wing where it has one - and `drawn`
 * builds the same thing here. A file built before a shape existed has none of its
 * meshes, and the drawn one stands in whole; a file that has the body but no wing
 * keeps its body and borrows the drawn wing, which is the one part that was added
 * after the beetle shipped.
 */
function fromFile(model, names, drawn) {
  const [shell, dark, legs, flight] = names;
  if (!model || !model.get(shell) || !model.get(dark) || !model.get(legs)) return drawn();
  return {
    shell: mergeGeometries([shade(model.get(shell).clone(), 1), shade(model.get(dark).clone(), DARK)]),
    legs: model.get(legs).clone(),
    wing: flight ? model.get(flight)?.clone() || wing() : null,
  };
}

// The beetle, which is what most findings are: wing cases that take the severity's
// color, a dark front end, six legs and one flight wing.
const beetle = model => fromFile(model, ['shell', 'dark', 'legs', 'wing'], () => {
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
});

/**
 * The caterpillar a critical finding walks as: a rank of segments four times the
 * length of a beetle, swelling in the middle and drawn in to a dark head. It has no
 * wings - the worst thing on the map is the one that has to be walked up to - and its
 * feet are stubs in pairs under every segment.
 */
const grub = model => fromFile(model, ['grub_shell', 'grub_dark', 'grub_legs'], () => {
  const SEGMENTS = 7, SEGMENT = BODY * 0.52, R = BODY * 0.52;
  const body = [];
  for (let i = 0; i < SEGMENTS; i++) {
    const t = i / (SEGMENTS - 1);
    // Fattest a third of the way back and tapering to the tail, which is what tells a
    // caterpillar from a length of rope.
    const r = R * (0.62 + 0.38 * Math.sin(Math.PI * Math.min(1, t * 1.25)));
    body.push(new THREE.SphereGeometry(r, 9, 7)
      .scale(1, 0.86, 1.05)
      .translate(0, r * 0.82, (t - 0.5) * SEGMENT * SEGMENTS));
  }
  const nose = (SEGMENTS / 2) * SEGMENT + R * 0.2;
  const head = shade(new THREE.SphereGeometry(R * 0.66, 9, 7).translate(0, R * 0.7, nose), DARK);
  const jaws = shade(new THREE.BoxGeometry(R * 0.9, R * 0.3, R * 0.3).translate(0, R * 0.42, nose + R * 0.5), DARK);
  const legs = [];
  for (let i = 0; i < SEGMENTS; i++) {
    const at = (i / (SEGMENTS - 1) - 0.5) * SEGMENT * SEGMENTS;
    for (const side of [-1, 1]) {
      legs.push(new THREE.BoxGeometry(0.016, 0.03, 0.018).translate(side * R * 0.7, 0.014, at));
    }
  }
  return { shell: mergeGeometries([shade(mergeGeometries(body), 1), head, jaws]), legs: mergeGeometries(legs), wing: null };
});

/**
 * The mite the small findings walk as: a dome barely wider than it is long, with the
 * head tucked under the front of it. Half a beetle's length and rounder, so a note
 * about a style rule is something you have to go and look for rather than something
 * that reads as a problem from across the street.
 */
const mite = model => fromFile(model, ['mite_shell', 'mite_dark', 'mite_legs', 'mite_wing'], () => {
  const R = BODY * 0.66;
  const dome = shade(new THREE.SphereGeometry(R, 10, 7).scale(1, 0.72, 0.92).translate(0, R * 0.62, 0), 1);
  const head = shade(new THREE.SphereGeometry(R * 0.4, 8, 6).scale(1, 0.8, 1).translate(0, R * 0.34, R * 0.76), DARK);
  const legs = [];
  for (let i = 0; i < LEGS; i++) {
    const side = i % 2 ? 1 : -1;
    legs.push(new THREE.BoxGeometry(R * 0.6, 0.01, 0.01)
      .rotateZ(side * 1.05)
      .translate(side * R * 0.5, 0.01, (Math.floor(i / 2) - 1) * R * 0.5));
  }
  return { shell: mergeGeometries([dome, head]), legs: mergeGeometries(legs), wing: wing().scale(0.7, 0.7, 0.7) };
});

/**
 * The stand-in left wing, for a bug.glb modelled before there was one in it. The
 * shape is tools/bug.py's, drawn here the way the stand-in beetle above is drawn:
 * flat, hinged on the body's own length so that turning it about that line beats it
 * (flap above), and lying over the back from the hinge outwards.
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
 * mirrored. On the street it comes out as a plain turn about the vertical.
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
  // How big this severity walks, and how much of it is left if it is being taken off
  // the map: the basis is scaled rather than the geometry, so one mesh per shape still
  // draws every size of it.
  const s = bug.scale * (bug.size ?? 1);
  m.scale(SIZE.set(s, s * (bug.squash ?? 1), s));
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
const SIZE = new THREE.Vector3();
let soapParts = null; // the bubble a bubbled bug leaves in, built once
