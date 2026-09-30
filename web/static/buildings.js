// What kind of building a box is drawn as in the city style, and what that looks like
// from far enough away that its windows are below a pixel.
//
// The type is form and material only - a slab with balconies, a glass tower, a brick
// walk-up - never color: the color of every facade is the box's own (its language,
// its history, dimming and hover), and a type only decides how that color is laid
// out and how much of the face is glass. The type comes from a hash of the node's id
// and from the box's proportions, so a file is the same building on every run and
// through every --watch redraw, and a file that grows into a tower changes type with
// it.
//
// The shaders (city.js) read the type per box from an attribute this module fills,
// and their far-away average of a facade is written from the LOOKS table below, so
// what the tests check here is what the map draws.

/** The building types, in the order the shaders number them. */
export const ARCHETYPES = ['residential', 'office', 'brick', 'panel', 'deco', 'warehouse', 'mixed'];
export const TYPE = Object.freeze(Object.fromEntries(ARCHETYPES.map((name, i) => [name, i])));

/** One story, in world units: the height the facades and every balcony are laid out on. */
export const STORY = 0.3;

// Proportions that decide between types. A box lower than LOW is one or two stories:
// a shed or warehouse when it is broad, a small shop or pavilion otherwise. From
// TOWER up a building is a tower, and from DECO up a tower may be an art-deco one
// with setbacks; HELIPAD is where a flat office roof gets a landing pad.
export const LOW = 0.75, TOWER = 3.6, DECO = 5.4, HELIPAD = 7;

/**
 * How each type reads from a distance, as the shaders average it (city.js
 * facadeFar). tone: the wall's brightness against the box's color (brightness only,
 * never hue); share: how much of the face is glass; tint: how much of the box's
 * color the glass carries (a curtain wall is tinted glass; a window in a brick wall
 * is a dark room). The glass share stays under half on every type, so the wall -
 * the data color - is what a facade mostly is.
 */
export const LOOKS = {
  residential: { tone: 1.0, share: 0.3, tint: 0.2 },
  office: { tone: 0.9, share: 0.4, tint: 0.75 },
  brick: { tone: 0.92, share: 0.24, tint: 0.15 },
  panel: { tone: 1.02, share: 0.26, tint: 0.15 },
  deco: { tone: 1.05, share: 0.3, tint: 0.35 },
  warehouse: { tone: 0.95, share: 0.1, tint: 0.15 },
  mixed: { tone: 1.0, share: 0.32, tint: 0.2 },
};

/**
 * A number in [0, 1) for a string, the same on every run (FNV-1a, 32 bits).
 */
export function hashString(text) {
  let h = 0x811c9dc5;
  for (let i = 0; i < text.length; i++) {
    h ^= text.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return (h >>> 0) / 4294967296;
}

/**
 * A box's seed: its node's id hashed, or its position for a box without a node.
 * Two further numbers drawn from it are independent enough to use apart (variant).
 */
export function seedOf(b) {
  return hashString(b.node?.id != null ? String(b.node.id) : `${b.x},${b.z}`);
}

const next = s => {
  const v = Math.sin(s * 12.9898 + 4.1414) * 43758.5453;
  return v - Math.floor(v);
};

/**
 * The type a box is drawn as, as an index into ARCHETYPES, or -1 for boxes that are
 * not buildings (land and terraces). It depends on nothing but the seed, the box's
 * kind and its proportions.
 *
 * - Broad, low boxes are warehouses: a collapsed directory one story high, a file of
 *   a few lines.
 * - The tallest are towers: glass offices, and from DECO up art-deco towers with
 *   setbacks as often as not.
 * - Everything between is housing, brick walk-ups, concrete panel blocks, mixed
 *   use with shops under flats, and offices, weighted by height: brick and shops
 *   are low, panel blocks and slabs middling.
 * - A symbol's plot is a small pavilion (brick, panel or office), never a
 *   warehouse or a tower; a package on an island is drawn like any building.
 *
 * Implements: REQ-CITY-031
 */
export function archetype(b, seed = seedOf(b)) {
  if (b.kind === 'land' || b.kind === 'terrace') return -1;
  const h = b.h, footprint = Math.min(b.w, b.d);
  const r = next(seed);
  if (b.kind === 'symbol') return r < 0.45 ? TYPE.brick : r < 0.8 ? TYPE.panel : TYPE.office;
  if (h < LOW) return footprint >= 0.9 && r < 0.7 ? TYPE.warehouse : r < 0.85 ? TYPE.brick : TYPE.mixed;
  if (h >= DECO) return r < 0.5 ? TYPE.deco : r < 0.85 ? TYPE.office : TYPE.residential;
  if (h >= TOWER) return pick(r, [[TYPE.office, 0.35], [TYPE.residential, 0.3], [TYPE.panel, 0.25], [TYPE.mixed, 0.1]]);
  if (h >= 1.5) return pick(r, [[TYPE.residential, 0.3], [TYPE.panel, 0.22], [TYPE.brick, 0.18], [TYPE.mixed, 0.18], [TYPE.office, 0.12]]);
  return pick(r, [[TYPE.brick, 0.36], [TYPE.mixed, 0.26], [TYPE.residential, 0.22], [TYPE.panel, 0.16]]);
}

function pick(r, weights) {
  let at = 0;
  for (const [type, weight] of weights) {
    at += weight;
    if (r < at) return type;
  }
  return weights[weights.length - 1][0];
}

/**
 * The per-box numbers the city shaders read (attribute aBuild): the type, a variant
 * in [0, 1) for material and layout within the type, the height of this piece's
 * base above the building's own (0 for the box itself), and the building's full
 * height.
 */
export function buildParameters(b, seed = seedOf(b)) {
  return [archetype(b, seed), next(seed + 0.5), 0, Math.max(b.h, 0.01)];
}

// ------------------------------------------------------------------ far average

// The glass and lamp colors the shaders average a far facade with (city.js
// facadeFar), in linear color. A lit window counts for half as much from afar as it
// is bright up close: a night city seen from above would otherwise be the color of
// its lamps, and the legend would not read at night (the dark theme's map).
const GLASS = [0.12, 0.13, 0.14];
const LAMP = [1.0, 0.72, 0.38];

/**
 * The color a facade averages to once its windows are below a pixel - what the map
 * shows of it from the default view - for a linear base color [r, g, b], a type
 * index, a variant and day (0) or night (1). Mirrors city.js facadeFar.
 */
export function facadeFar(base, type, variant = 0.5, night = 0) {
  const look = LOOKS[ARCHETYPES[type]];
  const dark = k => 1 + (k - 1) * night;
  const litShare = 0.08 + (0.4 - 0.08) * night;
  const lampOn = litShare * (0.6 - 0.1 * night);
  return [0, 1, 2].map(i => {
    const wall = base[i] * look.tone * (0.92 + 0.16 * variant) * dark(0.5);
    const glass = GLASS[i] + (base[i] * 0.45 - GLASS[i]) * look.tint;
    const window = glass * dark(0.12) * (1 - lampOn) + LAMP[i] * 0.8 * lampOn;
    return wall * (1 - look.share) + window * look.share;
  });
}

/** GLSL constants for the LOOKS table, in ARCHETYPES order. */
export function looksGLSL() {
  const list = key => ARCHETYPES.map(name => LOOKS[name][key].toFixed(4)).join(', ');
  return `
const float LOOK_TONE[${ARCHETYPES.length}] = float[](${list('tone')});
const float LOOK_SHARE[${ARCHETYPES.length}] = float[](${list('share')});
const float LOOK_TINT[${ARCHETYPES.length}] = float[](${list('tint')});
const vec3 LOOK_GLASS = vec3(${GLASS.map(v => v.toFixed(4)).join(', ')});
const vec3 LOOK_LAMP = vec3(${LAMP.map(v => v.toFixed(4)).join(', ')});
${ARCHETYPES.map((name, i) => `const int ${name.toUpperCase()} = ${i};`).join('\n')}
const float STORY = ${STORY.toFixed(4)};
const float HELIPAD_H = ${HELIPAD.toFixed(4)};
const float DECO_H = ${DECO.toFixed(4)};
`;
}
