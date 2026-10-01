// What stands out of the city's buildings, as geometry: balconies with railings,
// cornices and ledges, awnings over the shops, and on the roofs water tanks,
// air-conditioning units, antennas and railings - and the setbacks of the art-deco
// towers, which are not detail but the building itself, drawn as narrower tiers on a
// shorter shaft so that the top of the highest is the building's height.
//
// Everything here is placed from the building's type (buildings.js) the way the
// facade shader lays its windows out (city.js facade), so a balcony stands at the
// foot of the French window painted for it and an awning over the shop it shelters.
// Each kind of detail is one InstancedMesh, whatever the number of buildings: a few
// draws for all of them. They are built for the buildings near the walker, or for
// those on screen once the map is zoomed in far enough for them to be more than a
// pixel, a budget of buildings a frame; the facade paints its own flat stand-ins -
// a railing, the rooftop plant - where none are built (the aDetail attribute).
//
// None of it is solid. The walker collides with the boxes, which is what the map
// encodes; a balcony or an awning stands out of a facade by OVERHANG, less than the
// walker's own radius, so a body stopped by the wall is never under one, and the
// grapple and the jetpack treat a facade as the flat wall they always did. The one
// exception is the setbacks, which change where there is a roof to stand on:
// massTop gives the walker the tiers' roofs.

import * as THREE from './vendor/three.module.min.js';
import { TYPE, STORY, FACADE, DECO, HELIPAD, archetype, buildParameters, seedOf, isGround } from './buildings.js';

/** How far a balcony, an awning or a cornice stands out of a facade. */
export const OVERHANG = 0.075;
/** How far anything on a roof - a tank, a unit, an antenna - rises above it. */
export const GEAR_MAX = 0.24;
// The setbacks of an art-deco tower: the shaft rises to SHAFT of the height, the
// first tier to TIER, the crown to the top; each is inset by SETBACK of the
// footprint on every side from the one under it.
const SHAFT = 0.72, TIER = 0.86, SETBACK = 0.12;
// Level of detail. Walking, the buildings within WALK_REACH get their details;
// on the map, those on screen once a unit is MAP_ZOOM pixels or more. Never more
// than MAX_BOXES at once, and BUDGET new ones a frame. A map of no more than
// SMALL_MAP buildings gets them all while walking.
export const WALK_REACH = 12, MAP_ZOOM = 40, MAX_BOXES = 600, BUDGET = 40, SMALL_MAP = 250;

const RAIL = 0.1;      // a balcony railing's height
const SLAB = 0.018;    // a balcony slab's thickness
const LEDGE = 0.028;   // a cornice's depth and height
const PARAPET_RAIL = 0.06;

const next = s => {
  const v = Math.sin(s * 12.9898 + 4.1414) * 43758.5453;
  return v - Math.floor(v);
};
const fract = v => v - Math.floor(v);

// ------------------------------------------------------------------ setbacks

const tierCache = new WeakMap();

/**
 * The parts of a building's mass, bottom up: [{w, d, y0, y1}] above its base, the
 * first its own footprint. Null for every building but an art-deco tower with room
 * for its setbacks; the box then is the whole of it.
 *
 * Implements: REQ-CITY-034
 */
export function tiersOf(b) {
  if (b.h < DECO || isGround(b) || b.kind === 'symbol') return null;
  let tiers = tierCache.get(b);
  if (tiers === undefined) {
    tiers = archetype(b) === TYPE.deco ? [
      { w: b.w, d: b.d, y0: 0, y1: b.h * SHAFT },
      { w: b.w * (1 - 2 * SETBACK), d: b.d * (1 - 2 * SETBACK), y0: b.h * SHAFT, y1: b.h * TIER },
      { w: b.w * (1 - 4 * SETBACK), d: b.d * (1 - 4 * SETBACK), y0: b.h * TIER, y1: b.h },
    ] : null;
    tierCache.set(b, tiers);
  }
  return tiers;
}

/**
 * How high the building stands at (x, z), above its base: its height, or on a
 * setback tower the roof of the highest tier over that point. What the walker
 * stands on in the city style.
 *
 * Implements: REQ-CITY-034
 */
export function massTop(b, x, z) {
  const tiers = tiersOf(b);
  if (!tiers) return b.h;
  let top = 0;
  for (const t of tiers) if (Math.abs(x - b.x) <= t.w / 2 + 1e-9 && Math.abs(z - b.z) <= t.d / 2 + 1e-9) top = t.y1;
  return top;
}

// ------------------------------------------------------------------ placement

// A box's four faces as the facade shader measures them (city.js cityTexture): the
// outward normal, the direction of increasing u along the face, the face's width,
// its index (the shader's faceId), and the turn that takes +z to the normal.
function faces(b, w = b.w, d = b.d) {
  return [
    { n: [1, 0], u: [0, 1], width: d, at: [b.x + w / 2, b.z], id: 0, turn: Math.PI / 2 },
    { n: [-1, 0], u: [0, -1], width: d, at: [b.x - w / 2, b.z], id: 1, turn: -Math.PI / 2 },
    { n: [0, 1], u: [-1, 0], width: w, at: [b.x, b.z + d / 2], id: 2, turn: 0 },
    { n: [0, -1], u: [1, 0], width: w, at: [b.x, b.z - d / 2], id: 3, turn: Math.PI },
  ];
}

/** The bay width of each type's facade, as city.js facade lays it out. */
export function bayWidth(type) {
  return type === TYPE.office ? 0.13 : type === TYPE.panel ? 0.3 : type === TYPE.warehouse ? 0.45
    : type === TYPE.deco ? 0.19 : type === TYPE.brick ? 0.22 : 0.26;
}

/**
 * Which ground-floor bay of a face holds the door: the same sum the shader does.
 */
export function doorBay(variant, faceId, bays) {
  return Math.floor(fract(variant * 7.31 + faceId * 0.23) * bays);
}

/** Whether a roof carries solar panels (and so nothing else). */
export const solarRoof = (type, variant, small) =>
  (type === TYPE.residential || type === TYPE.panel || type === TYPE.mixed) && !(type === TYPE.residential && variant < 0.22 && small > 0.6)
  && fract(variant * 13.7) > 0.6 && small > 0.6;

const _m = new THREE.Matrix4(), _q = new THREE.Quaternion(), _p = new THREE.Vector3(), _s = new THREE.Vector3();
const _up = new THREE.Vector3(0, 1, 0);

/**
 * Every detail of one building: [{kind, matrix (16 numbers), tinted}], matrices
 * placing each kind's unit geometry (DETAIL_GEOMETRY) in the world. `tinted`
 * details take the building's color; the rest are neutral materials.
 *
 * Implements: REQ-CITY-033
 */
export function detailsOf(b) {
  const out = [];
  const [type, variant] = buildParameters(b);
  if (type < 0 || isGround(b) || b.kind === 'symbol' || Math.min(b.w, b.d) < 0.6) return out;
  const seed = seedOf(b);
  const tiers = tiersOf(b);
  const roofAt = tiers ? tiers[0].y1 : b.h;
  const put = (kind, x, y, z, turn, sx, sy, sz, tinted) => {
    _q.setFromAxisAngle(_up, turn);
    _m.compose(_p.set(x, y, z), _q, _s.set(sx, sy, sz));
    out.push({ kind, matrix: Array.from(_m.elements), tinted, variant });
  };
  const upper = type === TYPE.mixed ? (variant < 0.5 ? TYPE.residential : TYPE.brick) : type;
  // A facade is laid out FACADE times smaller than it is drawn: what is measured on
  // it is measured in its units (laid), and placed in the world's.
  const laid = v => v / FACADE, story = STORY * FACADE;
  const podiumTop = type === TYPE.mixed ? story * (laid(b.h) > 1.6 ? 2 : 1) : 0;

  for (const f of faces(b)) {
    const bays = Math.max(1, Math.floor(laid(f.width) / bayWidth(upper)));
    const bay = f.width / bays;
    const along = u => [f.at[0] + f.u[0] * u, f.at[1] + f.u[1] * u];
    // Balconies: under every French window of a residential facade, story by story
    // up to the one under the cornice.
    if (upper === TYPE.residential) {
      const every = variant < 0.5 ? 1 : 2, offset = Math.floor(variant * 10);
      for (let k = 1; (k + 0.86) * STORY < laid(roofAt) - 0.075; k++) {
        if (k * story < podiumTop) continue;
        for (let i = 0; i < bays; i++) {
          if ((i + offset) % every !== 0) continue;
          const u = -f.width / 2 + (i + 0.5) * bay;
          const [x, z] = along(u);
          put('balcony', x, b.y + k * story, z, f.turn, bay * 0.84, FACADE, 1, true);
        }
      }
    }
    // Awnings over the shops: on the ground floor of every type with shops, over
    // each bay the shader gives an awning, where the building is a story high.
    const shops = type !== TYPE.office && type !== TYPE.warehouse && type !== TYPE.panel && laid(roofAt) > STORY + 0.02;
    if (shops) {
      const groundBays = Math.max(1, Math.floor(laid(f.width) / bayWidth(upper)));
      const groundBay = f.width / groundBays;
      const door = doorBay(variant, f.id, groundBays);
      for (let i = 0; i < groundBays; i++) {
        if (i === door || fract(variant * 3.3 + i * 0.37) < 0.35) continue;
        const u = -f.width / 2 + (i + 0.5) * groundBay;
        const [x, z] = along(u);
        put('awning', x, b.y + story * 0.8, z, f.turn, groundBay * 0.92, FACADE, 1, true);
      }
    }
    // A cornice under the roof line of brick, art-deco and mixed buildings, and on
    // brick a string course over the ground floor.
    if (upper === TYPE.brick || type === TYPE.deco || type === TYPE.mixed) {
      const [x, z] = along(0);
      put('ledge', x, b.y + roofAt - 0.05 * FACADE, z, f.turn, f.width + 2 * LEDGE, LEDGE * FACADE, LEDGE, true);
      if (upper === TYPE.brick && roofAt > story * 2) put('ledge', x, b.y + story - 0.012 * FACADE, z, f.turn, f.width + 2 * LEDGE * 0.6, LEDGE * 0.8 * FACADE, LEDGE * 0.6, true);
    }
  }
  // The tiers of a setback tower carry a cornice each.
  if (tiers) {
    for (const t of tiers.slice(1)) {
      for (const f of faces(b, t.w, t.d)) {
        put('ledge', f.at[0], b.y + t.y1 - 0.05 * FACADE, f.at[1], f.turn, f.width + 2 * LEDGE, LEDGE * FACADE, LEDGE, true);
      }
    }
  }

  // The roof: what stands on the highest part of it.
  const top = tiers ? tiers[tiers.length - 1] : { w: b.w, d: b.d, y1: b.h };
  const y = b.y + top.y1;
  const small = Math.min(top.w, top.d);
  const spot = (k, reach = 0.32) => [b.x + (next(seed + k) - 0.5) * top.w * reach * 2, b.z + (next(seed + k + 0.37) - 0.5) * top.d * reach * 2];
  const solar = solarRoof(type, variant, small);
  const green = type === TYPE.residential && variant < 0.22 && small > 0.6;
  if (type === TYPE.deco) {
    put('antenna', b.x, y, b.z, 0, 1, 1, 1, false);
  } else if (!solar && !green) {
    if (type === TYPE.brick || (type === TYPE.residential && variant > 0.6) || (type === TYPE.mixed && variant > 0.7)) {
      const [x, z] = spot(1, 0.2);
      put('tank', x, y, z, next(seed + 2) * 6.28, 1, 1, 1, false);
    }
    const units = type === TYPE.office ? 3 : type === TYPE.warehouse ? 1 : 2;
    const helipad = type === TYPE.office && b.h >= HELIPAD && small > 0.7;
    for (let i = 0; i < units; i++) {
      // Clear of a helipad: out in the corners.
      const [x, z] = helipad
        ? [b.x + (i % 2 ? 1 : -1) * top.w * 0.36, b.z + (i < 2 ? 1 : -1) * top.d * 0.36]
        : spot(3 + i * 2, type === TYPE.brick ? 0.3 : 0.34);
      put('unit', x, y, z, Math.floor(next(seed + 9 + i) * 4) * Math.PI / 2, 1, 1, 1, false);
    }
    if ((type === TYPE.office || type === TYPE.panel) && b.h >= 2) {
      const [x, z] = helipad ? [b.x + top.w * 0.36, b.z - top.d * 0.36] : spot(11, 0.36); // the free corner
      put('antenna', x, y, z, 0, 1, 0.75 + 0.25 * next(seed + 12), 1, false);
    }
  }
  // Railings along the parapet of offices and panel blocks.
  if (type === TYPE.office || type === TYPE.panel) {
    for (const f of faces(b, top.w, top.d)) {
      const x = f.at[0] - f.n[0] * 0.012, z = f.at[1] - f.n[1] * 0.012;
      put('rail', x, y, z, f.turn, f.width - 0.024, 1, 1, false);
    }
  }
  return out;
}

// ------------------------------------------------------------------ geometry

// The detail geometries. Each vertex carries a neutral color (color), whether it
// takes the building's color instead (aTint), and what is painted on it (aPaint): 0
// nothing, 1 railing bars across x, 2 railing bars across z, 3 awning canvas.
// Faces are shaded in the shader from their world normal, the way the boxes are.
function part(geometry, color, tint = 0, paint = 0) {
  const g = geometry.index ? geometry.toNonIndexed() : geometry;
  const n = g.getAttribute('position').count;
  g.setAttribute('color', new THREE.Float32BufferAttribute(Array.from({ length: n }, () => color).flat(), 3));
  g.setAttribute('aTint', new THREE.Float32BufferAttribute(new Array(n).fill(tint), 1));
  g.setAttribute('aPaint', new THREE.Float32BufferAttribute(new Array(n).fill(paint), 1));
  g.deleteAttribute('uv');
  return g;
}

function merged(parts) {
  const out = new THREE.BufferGeometry();
  for (const [name, size] of [['position', 3], ['normal', 3], ['color', 3], ['aTint', 1], ['aPaint', 1]]) {
    const arrays = parts.map(p => p.getAttribute(name).array);
    const all = new Float32Array(arrays.reduce((a, x) => a + x.length, 0));
    let o = 0;
    for (const a of arrays) { all.set(a, o); o += a.length; }
    out.setAttribute(name, new THREE.BufferAttribute(all, size));
  }
  out.computeBoundingSphere();
  return out;
}

const box = (w, h, d, x, y, z) => new THREE.BoxGeometry(w, h, d).translate(x, y, z);
const METAL = [0.11, 0.115, 0.12], STEEL = [0.5, 0.51, 0.52], TIMBER = [0.3, 0.24, 0.18], CONCRETE = [0.55, 0.55, 0.53];

function awningGeometry() {
  // A canvas sloping from the wall down and out, with a valance hanging at its hem.
  const reach = OVERHANG - 0.004, drop = 0.055, slope = Math.atan2(drop, reach), length = Math.hypot(drop, reach);
  const canvas = new THREE.BoxGeometry(1, 0.005, length).translate(0, 0, length / 2).rotateX(slope);
  const valance = box(1, 0.018, 0.004, 0, -drop - 0.009, reach);
  return merged([part(canvas, [1, 1, 1], 1, 3), part(valance, [1, 1, 1], 1, 3)]);
}

/** Each detail kind's unit geometry, placed by detailsOf's matrices. */
export const DETAIL_GEOMETRY = {
  // Width 1 along the face, standing out along +z: a slab of the building's color,
  // railings round its three open sides and a handrail on top.
  balcony: merged([
    part(box(1, SLAB, OVERHANG, 0, SLAB / 2, OVERHANG / 2), [1, 1, 1], 1),
    part(box(1, RAIL, 0.004, 0, SLAB + RAIL / 2, OVERHANG - 0.002), METAL, 0, 1),
    part(box(0.004, RAIL, OVERHANG, -0.498, SLAB + RAIL / 2, OVERHANG / 2), METAL, 0, 2),
    part(box(0.004, RAIL, OVERHANG, 0.498, SLAB + RAIL / 2, OVERHANG / 2), METAL, 0, 2),
    part(box(1, 0.008, 0.012, 0, SLAB + RAIL, OVERHANG - 0.006), METAL, 0),
  ]),
  awning: awningGeometry(),
  // A unit cube behind the face's line, standing out of it by its own depth.
  ledge: part(box(1, 1, 1, 0, 0.5, 0.5), [1, 1, 1], 1),
  // A timber tank on steel legs, under a conical lid.
  tank: merged([
    ...[[-1, -1], [1, -1], [-1, 1], [1, 1]].map(([sx, sz]) => part(box(0.01, 0.03, 0.01, sx * 0.05, 0.015, sz * 0.05), METAL)),
    part(new THREE.CylinderGeometry(0.075, 0.075, 0.075, 12).translate(0, 0.03 + 0.0375, 0), TIMBER),
    part(new THREE.CylinderGeometry(0.078, 0.078, 0.006, 12).translate(0, 0.06, 0), METAL),
    part(new THREE.ConeGeometry(0.08, 0.03, 12).translate(0, 0.105 + 0.015, 0), METAL),
  ]),
  // An air-conditioning unit: a casing with a fan on top.
  unit: merged([
    part(box(0.1, 0.055, 0.075, 0, 0.0275, 0), STEEL),
    part(new THREE.CylinderGeometry(0.026, 0.026, 0.006, 10).translate(0, 0.058, 0), METAL),
  ]),
  // A mast with two cross-arms.
  antenna: merged([
    part(new THREE.CylinderGeometry(0.004, 0.007, 0.24, 5).translate(0, 0.12, 0), METAL),
    part(box(0.07, 0.004, 0.004, 0, 0.17, 0), METAL),
    part(box(0.045, 0.004, 0.004, 0, 0.21, 0), METAL),
  ]),
  // A railing along a parapet: bars, and the rail on top.
  rail: merged([
    part(box(1, PARAPET_RAIL, 0.004, 0, PARAPET_RAIL / 2, 0), CONCRETE, 0, 1),
    part(box(1, 0.007, 0.01, 0, PARAPET_RAIL, 0), STEEL),
  ]),
};
export const DETAIL_KINDS = Object.keys(DETAIL_GEOMETRY);

/**
 * The material the details share: bendable (MapScene.bendable) like everything on
 * the map, shaded per face as the boxes are, the building's color on the tinted
 * parts. Railing bars are cut out of their panels up close and drawn as their
 * average once a bar is smaller than a pixel; awnings are striped or plain by
 * building. Everything dims at night as the facades do.
 */
function detailMaterial(bendable) {
  const material = bendable(new THREE.MeshBasicMaterial({ vertexColors: true }));
  const bend = material.onBeforeCompile;
  material.onBeforeCompile = (shader, renderer) => {
    bend(shader, renderer);
    shader.vertexShader = `
attribute float aTint;
attribute float aPaint;
attribute float aVariant;
varying float vPaint;
varying float vVariant;
varying vec3 vPart;   // position in the part, in world units
varying float vShade;
` + shader.vertexShader
      .replace('#include <color_vertex>', `#include <color_vertex>
#ifdef USE_INSTANCING_COLOR
  vColor.xyz = color * mix(vec3(1.0), instanceColor.xyz, aTint);
#endif`)
      .replace('#include <begin_vertex>', `#include <begin_vertex>
  vPaint = aPaint;
  vVariant = aVariant;
  vPart = position;
  vec3 worldNormal = normal;
#ifdef USE_INSTANCING
  vPart *= vec3(length(instanceMatrix[0].xyz), length(instanceMatrix[1].xyz), length(instanceMatrix[2].xyz));
  worldNormal = normalize(mat3(instanceMatrix) * normal);
#endif
  // The boxes' own face shading: tops full, sides as if lit from the north-west.
  vShade = worldNormal.y > 0.5 ? 1.0 : worldNormal.y < -0.5 ? 0.45 : abs(worldNormal.x) > 0.5 ? 0.62 : 0.78;`);
    shader.fragmentShader = `
uniform float uNight;
varying float vPaint;
varying float vVariant;
varying vec3 vPart;
varying float vShade;
float band(float t, float lo, float hi, float w) { return smoothstep(lo - w, lo + w, t) - smoothstep(hi - w, hi + w, t); }
` + shader.fragmentShader.replace('#include <color_fragment>', `#include <color_fragment>
  if (vPaint > 0.5 && vPaint < 2.5) {
    // Bars every 3 centimeters, with a bottom rail: cut out up close, averaged afar.
    float t = (vPaint < 1.5 ? vPart.x : vPart.z) / 0.03;
    float w = fwidth(t);
    float bar = band(fract(t), 0.0, 0.22, w);
    if (w < 0.3) {
      if (bar < 0.5) discard;
    } else {
      diffuseColor.rgb *= 0.8;
    }
  } else if (vPaint > 2.5) {
    // Canvas: stripes of the building's color darkened and off-white, or plain.
    float t = vPart.x / 0.034, w = fwidth(t) + 1e-4;
    float stripe = mix(band(fract(t), 0.0, 0.5, w), 0.5, smoothstep(0.3, 0.8, w)) * step(0.5, fract(vVariant * 5.1));
    diffuseColor.rgb = mix(diffuseColor.rgb * 0.5, vec3(0.78, 0.77, 0.73), stripe * 0.8);
  }
  diffuseColor.rgb *= vShade * mix(1.0, 0.5, uNight);`);
  };
  material.customProgramCacheKey = () => 'bend-detail';
  return material;
}

// ------------------------------------------------------------------ the details of a map

/**
 * The details of a layout: one InstancedMesh per kind in `group`, rebuilt as the
 * view moves (update). Dimmed boxes have none. `flags` holds 1 for every box whose
 * details are built, for the facade shader (aDetail): it paints its stand-ins
 * elsewhere.
 *
 * Implements: REQ-CITY-033, REQ-PERF-009
 */
export class Details {
  constructor(bendable) {
    this.group = new THREE.Group();
    this.material = detailMaterial(bendable);
    this.meshes = new Map();
    this.boxes = [];
    this.flags = new Float32Array(0);
    this.colors = [];
    this.faded = [];
    this.cache = new Map();   // box index -> its details
    this.active = new Set();  // box indexes built
    this.enabled = true;
    this.lastView = '';
    this.pending = false;
    this.version = 0;     // counts rebuilds, for whoever uploads `flags`
    this.ranges = new Map();
    this.eligible = [];
  }

  /** A new layout; `on`: whether its style has details (the city does). */
  setBoxes(boxes, on = true) {
    this.boxes = boxes;
    this.enabled = on;
    this.cache.clear();
    this.active.clear();
    this.flags = new Float32Array(boxes.length);
    this.lastView = '';
    this.pending = false;
    this.eligible = [];
    if (on) boxes.forEach((b, i) => { if (b.kind !== 'land' && b.kind !== 'terrace' && b.kind !== 'symbol' && !isGround(b)) this.eligible.push(i); });
    this.rebuild();
  }

  /**
   * The boxes' colors and dimming, as MapScene.setColors has them. A change of
   * color repaints that box's details in place; a change of dimming rebuilds.
   */
  setColors(colors, faded = [], changed = null) {
    const was = this.faded;
    this.colors = colors;
    this.faded = faded;
    const list = changed || [...this.active];
    let rebuild = false;
    for (const i of list) if (!faded[i] !== !was[i]) rebuild = true;
    if (rebuild) { this.lastView = ''; return; }
    for (const i of list) if (this.active.has(i)) this.paint(i);
  }

  /** Which boxes a view wants details for, nearest first. */
  wanted(view) {
    if (!this.enabled) return [];
    const out = [];
    if (view.walking) {
      const all = this.eligible.length <= SMALL_MAP;
      for (const i of this.eligible) {
        const b = this.boxes[i];
        const d = Math.hypot(b.x - view.x, b.z - view.z);
        if (all || d < WALK_REACH) out.push([d, i]);
      }
    } else {
      if (!(view.zoom >= MAP_ZOOM)) return [];
      for (const i of this.eligible) {
        const b = this.boxes[i];
        const at = view.project(b.x, b.y + b.h / 2, b.z);
        if (!at || Math.abs(at.x) > 1.15 || Math.abs(at.y) > 1.15) continue;
        out.push([Math.hypot(at.x, at.y), i]);
      }
    }
    out.sort((a, b) => a[0] - b[0]);
    return out.slice(0, MAX_BOXES).map(e => e[1]).filter(i => !this.faded[i]);
  }

  /**
   * Brings the details up to date with a view: {walking, x, z} for the walker,
   * {zoom, project(x, y, z) -> NDC or null, key} for the map camera. Builds at most
   * BUDGET boxes' details a call; returns true while there is more to do, for the
   * caller to draw another frame.
   */
  update(view) {
    const key = view.walking ? `w${Math.round(view.x)},${Math.round(view.z)}` : `m${view.key}`;
    if (key === this.lastView && !this.pending) return false;
    const want = this.wanted(view);
    let built = 0, more = false;
    const chosen = new Set();
    for (const i of want) {
      if (!this.cache.has(i)) {
        if (built >= BUDGET) { more = true; continue; }
        this.cache.set(i, detailsOf(this.boxes[i]));
        built++;
      }
      chosen.add(i);
    }
    const same = chosen.size === this.active.size && [...chosen].every(i => this.active.has(i));
    this.lastView = key;
    this.pending = more;
    if (!same) {
      this.active = chosen;
      this.rebuild();
    }
    return more;
  }

  // Every kind's instances from the active boxes' details, and each box's range in
  // them for repainting.
  rebuild() {
    this.version++;
    const byKind = new Map(DETAIL_KINDS.map(k => [k, []]));
    this.flags.fill(0);
    this.ranges = new Map();
    for (const i of this.active) {
      for (const d of this.cache.get(i) || []) byKind.get(d.kind).push([i, d]);
      this.flags[i] = 1;
    }
    for (const kind of DETAIL_KINDS) {
      const items = byKind.get(kind);
      items.sort((a, b) => a[0] - b[0]);
      let mesh = this.meshes.get(kind);
      if (!mesh || mesh.instanceMatrix.count < items.length) {
        if (mesh) { this.group.remove(mesh); mesh.dispose(); }
        const capacity = Math.max(16, 2 ** Math.ceil(Math.log2(Math.max(1, items.length))));
        const geometry = DETAIL_GEOMETRY[kind].clone();
        geometry.setAttribute('aVariant', new THREE.InstancedBufferAttribute(new Float32Array(capacity), 1));
        mesh = new THREE.InstancedMesh(geometry, this.material, capacity);
        mesh.instanceColor = new THREE.InstancedBufferAttribute(new Float32Array(capacity * 3).fill(1), 3);
        mesh.frustumCulled = false; // bent in walk mode, like the props
        mesh.userData.kind = kind;
        this.meshes.set(kind, mesh);
        this.group.add(mesh);
      }
      const variant = mesh.geometry.getAttribute('aVariant');
      const ranges = new Map();
      items.forEach(([i, d], k) => {
        mesh.instanceMatrix.array.set(d.matrix, k * 16);
        variant.setX(k, d.variant);
        const r = ranges.get(i);
        if (r) r[1]++; else ranges.set(i, [k, 1]);
      });
      mesh.count = items.length;
      mesh.visible = items.length > 0; // no draw at all for a kind nobody has
      mesh.instanceMatrix.needsUpdate = true;
      variant.needsUpdate = true;
      this.ranges.set(kind, { ranges, items });
    }
    for (const i of this.active) this.paint(i, false);
    for (const mesh of this.meshes.values()) mesh.instanceColor.needsUpdate = true;
  }

  // One box's tinted details in its color.
  paint(i, upload = true) {
    const css = this.colors[i];
    if (!css) return;
    const color = parsed(css);
    for (const [kind, { ranges, items }] of this.ranges) {
      const r = ranges.get(i);
      if (!r) continue;
      const mesh = this.meshes.get(kind);
      for (let k = r[0]; k < r[0] + r[1]; k++) {
        if (items[k][1].tinted) mesh.instanceColor.setXYZ(k, color.r, color.g, color.b);
        else mesh.instanceColor.setXYZ(k, 1, 1, 1);
      }
      if (upload) mesh.instanceColor.needsUpdate = true;
    }
  }

  /** How many of each kind are built, for the tests and the frame counter. */
  counts() {
    return Object.fromEntries([...this.meshes].map(([k, m]) => [k, m.count]));
  }

  dispose() {
    for (const mesh of this.meshes.values()) { mesh.geometry.dispose(); mesh.dispose(); }
    this.material.dispose();
  }
}

const colorCache = new Map();
function parsed(css) {
  let c = colorCache.get(css);
  if (!c) {
    if (colorCache.size > 512) colorCache.clear();
    colorCache.set(css, c = new THREE.Color(css));
  }
  return c;
}
