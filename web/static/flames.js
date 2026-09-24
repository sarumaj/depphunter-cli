// What fire looks like. fires.js decides what is burning and how hot; this draws it
// and knows nothing else.
//
// Not a model. A flame has no shape to model - what the eye reads as fire is the
// motion and the layering, not the silhouette, and a rigged plume out of Blender is a
// cone that wobbles. It is the same argument the net's bag settled (tools.js): the
// smooth translucent cone read as a plastic bag, and the netting had to be built out
// of the thing netting actually is. So fire is built out of what fire is - tongues
// that climb, thin, lean and go out, each on its own clock, with smoke above them and
// embers coming off the top.
//
// Three things make it convincing and all three are cheap:
//
//   Soft edges out of vertex alpha. A tongue is painted opaque down its spine and
//   clear at its edges, its root and its tip, so it has no outline anywhere - which
//   is the whole difference between fire and cut paper, and the only way to a soft
//   edge on a map with no textures in it.
//
//   Not additive, which is the obvious way to do all this and is wrong here. Adding
//   light only ever brightens what is behind it, and this map is a daylight scene:
//   green lawns, a pale sky, sunlit walls. Orange added to any of that is orange
//   nobody can see. It looks superb against a dark background and that is exactly the
//   trap - fire over a night sky is not the picture this map draws.
//
//   Crossed blades. Each tongue is two flat blades at right angles. A flat one is a
//   cutout the moment you walk round it, and this map is looked at from a fixed
//   isometric camera and from a walker's eyes - a billboard that turns to face one
//   is wrong for the other. A cross reads as volume from every side and costs two
//   triangles more.
//
//   Staggered lives. Every tongue is somewhere different in its own rise-and-die, so
//   the fire is never doing one thing; it licks. The clock is per instance and the
//   shape of a life is in `climb` below, where it can be read and tested.
//
// It is drawn with the same instancing bugs.js uses, for the same reason: a burning
// district is hundreds of tongues, and hundreds of meshes is most of a frame. Three
// instanced meshes draw all of it - the tongues, the smoke, the embers - whatever is
// alight. And like everything else out there the materials are unlit and bendable, so
// a fire on the far side of the planet leans over the horizon with the street it is
// standing in.

import * as THREE from './vendor/three.module.min.js';
import { mergeGeometries } from './vendor/BufferGeometryUtils.js';

// A tongue's life, in seconds, and how far it leans while it lives. Fires are lit at
// staggered phases so a fire is never all one age.
export const LIVES = 1.15;
export const LEANS = 0.22;
// How many tongues a fire gets: a couple when it has just caught, a crown of them
// when it is well alight. Smoke and embers only come once it is properly going.
// Many and small rather than few and large. A handful of big blades is a candle: the
// shape of each one is what you end up looking at. A crowd of little ones is a fire,
// because what you look at is the crowd - no single tongue is legible, they overlap
// into a body, and the body is doing something different every frame.
export const TONGUES = 16, PUFFS = 5, EMBERS = 5;
export const SMOKES_AT = 0.5;
// The bed: wide, low, barely moving tongues under the licking ones.
//
// This is the piece that turns a group of flames into a fire. Tongues on their own
// are a row of candles however many there are and however well they are staggered -
// each one has its own outline against the dark and the eye reads them one at a time.
// A fire is a body of light with tongues coming off it, and the body is what says the
// thing underneath is alight rather than decorated. Laid down first, wide enough to
// overlap each other and everything above them, they weld the whole into one.
export const BED = 7;

/**
 * Where a tongue is in its life, at a phase 0..1 through it.
 *
 * Returns how high it has climbed, how wide it still is and how bright it burns. A
 * flame is widest and brightest low down and gone by the top, so width and light both
 * fall away as the climb goes on - which is what makes a tongue read as something
 * being consumed rather than a shape moving upwards.
 */
export function climb(phase) {
  const t = phase - Math.floor(phase); // wraps, so a tongue lives over and over
  return {
    rise: t * t * (3 - 2 * t),        // eased, so it leaves fast and arrives slowly
    wide: Math.max(0, 1 - t * t * 1.15),
    light: Math.max(0, 1 - t ** 1.6),
  };
}

/**
 * How wide a tongue is at a height fraction up its own length: widest where it leaves
 * the fire, a little fuller just above that, and drawn to a point at the tip.
 *
 * Blunt at the bottom and pointed at the top is the whole difference between a flame
 * and a leaf. Pinch the root and the tongue stops looking like it is coming out of
 * anything - it floats, and a dozen of them float together, which is the one mistake
 * that makes a fire look like confetti.
 */
export const across = t => (1 - t) ** 0.55 * (1 + 0.3 * Math.sin(Math.PI * t));

// The colors a tongue is painted in, from its root to its tip, in the order they are
// passed through. The tip is near black on purpose: under additive blending that is
// what fading out means, and it is the only way to a soft edge without a texture.
const HEAT = [
  [1.00, 0.96, 0.78], // the core, which is nearly white
  [1.00, 0.78, 0.25],
  [0.97, 0.47, 0.10],
  [0.85, 0.24, 0.05],
  [0.66, 0.13, 0.03], // the tip, which stays a color rather than going to black
];

// How opaque a tongue is up its own length: clear where it leaves the fuel, solid
// through the body, gone by the tip. The root fade is what stops a dozen tongues
// drawing one hard lit bar across the foot of the fire, all in the same place.
export const alphaAt = t => Math.min(1, t * 7) * Math.max(0, 1 - t ** 1.7);
// How solid a single tongue is at its most solid.
//
// Low, because a fire is a dozen of them over one another and each has to be
// something you can see through - but not as low as that argument alone suggests. A
// fire is only ever a dozen deep where you are standing in front of it. Seen from a
// street away, across a roofline, the tongues are a few pixels wide and hardly cross
// at all, so each one is on its own against a bright sky and half of nothing is
// nothing: measured off the real map, a roof well alight came out at (175, 159, 148),
// which is gray. This is what a single tongue has to be worth for a fire to still be
// a fire at the distance most of them are seen from.
const BODY = 0.68;

/** The color at a height fraction up a tongue, mixed from the run above. */
export function heatAt(t) {
  const at = Math.min(HEAT.length - 1, Math.max(0, t)) * (HEAT.length - 1);
  const i = Math.min(HEAT.length - 2, Math.floor(at)), k = at - i;
  return HEAT[i].map((v, n) => v + (HEAT[i + 1][n] - v) * k);
}

// How much of a blade's opacity is left at its own edge: none to speak of. A blade
// of even opacity has a hard edge wherever it is drawn, and no number of them adds up
// to something alight - the eye finds every outline. Solid down the spine and clear
// at the sides, and a dozen of them cross into a body with no edges in it at all.
const EDGE = 0.04;

/**
 * One blade of a tongue: a strip up the Y axis, a unit tall and `across` wide, with
 * the heat run painted down its spine and taken almost to nothing at both edges.
 * Two of these crossed make a tongue.
 */
function blade(steps = 10) {
  const pos = [], col = [], idx = [];
  for (let i = 0; i <= steps; i++) {
    const t = i / steps, w = across(t) * 0.5;
    // Same color all the way across, and the edges clear instead. Fading a color
    // towards black would give the blade a dark rim on a pale wall; fading it towards
    // nothing gives it no rim at all, whatever it is drawn over.
    const c = heatAt(t), a = alphaAt(t) * BODY;
    pos.push(-w, t, 0, 0, t, 0, w, t, 0);
    col.push(...c, EDGE * a, ...c, a, ...c, EDGE * a);
    if (i < steps) {
      const a = i * 3;
      // Both halves of the rung, left edge to spine and spine to right edge.
      idx.push(a, a + 1, a + 3, a + 1, a + 4, a + 3);
      idx.push(a + 1, a + 2, a + 4, a + 2, a + 5, a + 4);
    }
  }
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
  // Four components, not three: three.js reads the alpha off the color attribute
  // when it has one, which is how a tongue gets a soft edge without a texture and
  // without a shader of its own.
  g.setAttribute('color', new THREE.Float32BufferAttribute(col, 4));
  g.setIndex(idx);
  return g;
}

// The crossing blade is the narrower of the two. Two equal blades put their bright
// spines through each other at right angles and the cross itself becomes the thing
// you can see - a seam straight up the middle of every tongue. Uneven, the tongue
// keeps its volume from any side and has no shape of its own to give away.
const CROSS = 0.6;

/** A tongue: two blades at right angles, so it has a front from wherever it is seen. */
export function tongueGeometry() {
  return mergeGeometries([blade(), blade().scale(CROSS, 1, 1).rotateY(Math.PI / 2)]);
}

/** A puff of smoke, and an ember: both are the cheapest solid that is not a square. */
// Once subdivided for the smoke, because an unsubdivided one is a hexagon from every
// angle and a hexagon over a roof is not smoke however faintly it is drawn. An ember
// is small enough that nobody can tell, and there are more of them.
const puffGeometry = () => new THREE.IcosahedronGeometry(0.5, 1);
const emberGeometry = () => new THREE.IcosahedronGeometry(0.022, 0);

/**
 * A number that looks random and is not: the same seat gives the same jitter every
 * time, so a fire does not reshuffle itself on a relayout, and nothing here has to
 * carry a seed about.
 */
export const wobble = (n, salt = 0) => {
  const x = Math.sin((n + 1) * 12.9898 + salt * 78.233) * 43758.5453;
  return x - Math.floor(x);
};

/**
 * How big a blaze on a given roof is: how far across the flames stand, and how high
 * they reach at that heat.
 *
 * Exported because two things need the same answer and they must not each have their
 * own. This decides where the tongues are drawn; it also decides where a walker gets
 * burnt (walk.js). Two copies of it would drift, and the way you would find out is a
 * walker scorched by a fire they are standing clear of, or standing in one that does
 * nothing - neither of which looks like a bug in a number.
 *
 * Against the building, not against nothing: a blaze on a roof is about as wide as the
 * roof and about as tall as it is wide. Twice that is a bonfire standing on a shed.
 */
export function blazeSize(heat, w, d) {
  const spread = Math.min(1.6, Math.max(0.35, Math.min(w, d) * 0.72));
  return { spread, tall: (0.26 + heat * 0.5) * Math.max(0.7, spread * 1.3) };
}

// How far the heat carries past the flames themselves, in map units. Close: this is a
// fire on a roof, not a firestorm, and a walker on the next roof is watching it rather
// than in it.
export const SCORCHES = 0.9;

/**
 * Whether a walker standing at (x, feet, z) is in the fire on `f` - which carries the
 * roof it stands on (y), the footprint (w, d) and how hard it is burning.
 *
 * Below the roof there is a building in the way, and above the flames there is air, so
 * both are safe: what burns is standing in it, or flying low through it.
 */
export function inBlaze(x, feet, z, f) {
  const { spread, tall } = blazeSize(f.heat, f.w, f.d);
  if (Math.hypot(x - f.x, z - f.z) > spread + SCORCHES) return false;
  const up = feet - f.y;
  return up > -0.5 && up < tall + SCORCHES;
}

/**
 * The fires, drawn.
 *
 * `place` is given what is alight and where; `step` advances the clock. Nothing here
 * decides what burns - that is fires.js - and nothing here reads the scene except to
 * add three meshes to it.
 */
export class Flames {
  /**
   * `cap` is the most fires there will ever be at once - fires.js caps them, and the
   * meshes are sized for that once. Sizing them for what is alight instead would
   * rebuild three instanced meshes every time a fire took or went out, which is
   * several times a minute while a front is moving, and each rebuild throws away
   * geometry the GPU has already been given.
   */
  constructor(scene, cap = 60) {
    this.scene = scene;
    this.cap = cap;
    this.group = new THREE.Group();
    this.group.visible = false;
    this.at = [];      // what is alight: { x, y, z, w, d, heat }
    this.clock = 0;
    this.drawn = null;
    scene.scene.add(this.group);
  }

  show(on) { this.group.visible = on; }

  /** Whether anything is alight, which is what the map asks before it keeps redrawing. */
  get burning() { return this.at.length; }

  /**
   * What is alight and where. `fires` is what fires.js has lit; `boxOf` turns a node
   * id into the box the layout put it in, and anything it does not know is not drawn -
   * a burning package whose island is filtered away is still burning, just not here.
   */
  place(fires, boxOf) {
    this.at = [];
    for (const { id, heat } of fires) {
      if (this.at.length >= this.cap) break;
      const b = boxOf(id);
      if (b) this.at.push({ x: b.x, z: b.z, y: b.y + b.h, w: b.w, d: b.d, heat, id });
    }
    this.build();
  }

  /** The three meshes, sized once for the most fires there can be. */
  build() {
    if (this.drawn || !this.at.length) return;
    const want = this.cap;
    const lit = geo => this.scene.bendable(new THREE.MeshBasicMaterial({
      vertexColors: geo === 'tongue', color: geo === 'tongue' ? undefined : '#ff8a2b',
      // No depth written, because a fire is a dozen surfaces over one another and each
      // has to be seen through; drawn from both sides, because a tongue leans past its
      // own blade and the far half is what you are looking at when it does.
      transparent: true, depthWrite: false, side: THREE.DoubleSide,
      opacity: geo === 'tongue' ? 1 : 0.8,
    }));
    const tongues = new THREE.InstancedMesh(tongueGeometry(), lit('tongue'), want * (TONGUES + BED));
    const embers = new THREE.InstancedMesh(emberGeometry(), lit('ember'), want * EMBERS);
    // Smoke is the one thing out here that is not additive: it takes light away rather
    // than adding it, which is the whole of what smoke does to what is behind it.
    const smoke = new THREE.InstancedMesh(puffGeometry(), this.scene.bendable(
      new THREE.MeshBasicMaterial({
        // Thin and pale rather than dark and solid: smoke is something you see the
        // city through, and a black ball over a roof is a hole in the map.
        // Very faint indeed. A puff is a solid with an outline, and nothing here can
        // soften its rim - the falloff a tongue gets is painted along a known axis,
        // and a ball has no such axis to paint along. What it can be is too thin for
        // the outline to register, and then several of them at different sizes read
        // as one soft mass instead of as a row of circles.
        color: '#585049', transparent: true, opacity: 0.032, depthWrite: false,
      })), want * PUFFS);
    // Over the buildings they stand on rather than fighting them for depth, and smoke
    // over the flames it is rising off.
    tongues.renderOrder = 3;
    embers.renderOrder = 4;
    smoke.renderOrder = 2;
    for (const m of [smoke, tongues, embers]) {
      m.frustumCulled = false; // they lean and rise past the box three.js measured
      this.group.add(m);
    }
    this.drawn = { tongues, embers, smoke };
  }

  clear() {
    for (const m of this.group.children) {
      m.geometry.dispose();
      m.material.dispose();
    }
    this.group.clear();
  }

  /**
   * A frame of fire. Every tongue climbs its own life, leaning as it goes and
   * shrinking as it climbs; the smoke above it rises slower and spreads; the embers
   * come off the top and drift.
   */
  step(dt) {
    this.clock += dt;
    const d = this.drawn;
    if (!d) return;
    const m = new THREE.Matrix4(), q = new THREE.Quaternion(), s = new THREE.Vector3(), p = new THREE.Vector3();
    const tint = new THREE.Color();
    let t = 0, e = 0, k = 0;
    this.at.forEach((fire, n) => {
      // A fire covers its own roof: the wider the building, the wider the blaze, but
      // never so wide that a warehouse looks like it is under a different fire.
      const { spread, tall } = blazeSize(fire.heat, fire.w, fire.d);
      // Even a fire that has only just caught gets a few: two tongues is two shapes,
      // and two shapes is a pair of objects rather than a small fire.
      const many = Math.max(6, Math.round(TONGUES * (0.35 + fire.heat * 0.65)));
      for (let i = 0; i < many + BED && t < d.tongues.count; i++, t++) {
        const seed = n * 31 + i;
        const bed = i < BED;
        const phase = this.clock / LIVES + wobble(seed);
        const { rise, wide, light } = climb(phase);
        // Where on the roof this tongue stands, and which way it leans as it climbs.
        const room = bed ? spread * 0.5 : spread;
        const ax = (wobble(seed, 1) - 0.5) * room, az = (wobble(seed, 2) - 0.5) * room;
        const lean = (bed ? LEANS * 0.15 : LEANS) * rise * (0.4 + fire.heat);
        // Each tongue is its own size for good, on top of the size its life gives it.
        // Without this every one is the same flame at a different moment, which the
        // eye picks out immediately however well they are staggered.
        // A bed tongue is half as tall and two and a half times as wide as a licking
        // one, and it hardly moves: the body of a fire seethes where the tongues leap.
        const own = bed ? 0.3 + wobble(seed, 8) * 0.25 : 0.4 + wobble(seed, 8) * 0.7;
        const fat = bed ? 3.4 : 1;
        // Tongues away from the middle are smaller, so the blaze has a body and an
        // edge rather than being a slab of equal flames.
        // The bed keeps its width wherever it sits, or a fire has a bright middle and
        // nothing at the sides, which is a lamp.
        const mid = bed ? 1 : 1 - Math.min(1, Math.hypot(ax, az) / (spread * 0.8)) * 0.45;
        p.set(fire.x + ax + Math.sin(phase * 5.1 + seed) * lean,
          fire.y + (wobble(seed, 9) - 0.75) * tall * 0.3 + rise * tall * own * 0.5,
          fire.z + az + Math.cos(phase * 4.3 + seed) * lean);
        q.setFromAxisAngle(UP, wobble(seed, 3) * Math.PI);
        const w = spread * 0.42 * fat * (0.35 + 0.5 * wide) * own * mid;
        s.set(w, tall * own * mid * (0.45 + 0.75 * wide), w);
        d.tongues.setMatrixAt(t, m.compose(p, q, s));
        // Brightness is the tongue's own life times how hard the fire is burning, so
        // one just caught glows and one well alight is white in the middle.
        // What the fire's heat does to a tongue is shift its color, not dim it.
        //
        // An instance tint multiplies, and multiplying towards nothing under ordinary
        // blending gives a black flame - which on a sunlit lawn is a hole in the map.
        // So a cool fire is a redder fire, and a tongue going out goes by shrinking
        // and clearing, which is how a flame actually leaves: `wide` takes its size
        // and the alpha painted up its length takes the rest. `light` spends the last
        // of its life on the red rather than on the lot.
        const hot = fire.heat * (0.55 + light * 0.45);
        d.tongues.setColorAt(t, tint.setRGB(1, 0.62 + hot * 0.34, 0.26 + hot * 0.64));
      }
      if (fire.heat >= SMOKES_AT) {
        for (let i = 0; i < PUFFS && k < d.smoke.count; i++, k++) {
          const seed = n * 17 + i;
          const up = (this.clock * 0.3 / (1 + i * 0.2) + wobble(seed, 4)) % 1;
          // Leaving the top of the flames rather than hanging over them, and spreading
          // as it climbs the way smoke does once it is off the heat.
          p.set(fire.x + (wobble(seed, 5) - 0.5) * spread * 0.6 + up * spread,
            fire.y + tall * 0.9 + up * tall * 2.2, fire.z + (wobble(seed, 6) - 0.5) * spread * 0.6);
          s.setScalar(spread * (0.3 + up * 1.5) * (0.6 + wobble(seed, 8) * 0.9));
          d.smoke.setMatrixAt(k, m.compose(p, IDENTITY, s));
        }
        for (let i = 0; i < EMBERS && e < d.embers.count; i++, e++) {
          const seed = n * 13 + i;
          const up = (this.clock * 0.8 / (1 + i * 0.15) + wobble(seed, 7)) % 1;
          p.set(fire.x + Math.sin(up * 7 + seed) * spread * 0.7,
            fire.y + tall * (0.5 + up * 2), fire.z + Math.cos(up * 6 + seed) * spread * 0.7);
          s.setScalar(Math.max(0.05, 1 - up) * (0.25 + fire.heat * 0.5));
          d.embers.setMatrixAt(e, m.compose(p, IDENTITY, s));
        }
      }
    });
    // Whatever was not wanted this frame is put where nothing can see it, which is
    // cheaper than rebuilding the mesh every time a fire takes or goes out.
    hide(d.tongues, t); hide(d.embers, e); hide(d.smoke, k);
    d.tongues.instanceMatrix.needsUpdate = true;
    d.embers.instanceMatrix.needsUpdate = true;
    d.smoke.instanceMatrix.needsUpdate = true;
    if (d.tongues.instanceColor) d.tongues.instanceColor.needsUpdate = true;
  }
}

const UP = new THREE.Vector3(0, 1, 0);
const IDENTITY = new THREE.Quaternion();
const NOWHERE = new THREE.Matrix4().makeScale(0, 0, 0);

/** Every instance from `from` on, scaled to nothing. */
function hide(mesh, from) {
  for (let i = from; i < mesh.count; i++) mesh.setMatrixAt(i, NOWHERE);
}
