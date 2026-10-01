// The city style's shaders (city.js): the noise every surface is painted with, and
// what MapScene.bendable adds to the boxes' material - the facades, roofs, streets and
// shores of the city, the circuit board and the galaxy, all painted per pixel from the
// box's own color.

import { looksGLSL } from './buildings.js';

export const NOISE_GLSL = `
float hash12(vec2 p) {
  vec3 p3 = fract(vec3(p.xyx) * 0.1031);
  p3 += dot(p3, p3.yzx + 33.33);
  return fract((p3.x + p3.y) * p3.z);
}
float hash13(vec3 p) {
  p = fract(p * 0.1031);
  p += dot(p, p.zyx + 31.32);
  return fract((p.x + p.y) * p.z);
}
float vnoise(vec2 p) {
  vec2 i = floor(p), f = fract(p);
  f = f * f * (3.0 - 2.0 * f);
  return mix(mix(hash12(i), hash12(i + vec2(1, 0)), f.x), mix(hash12(i + vec2(0, 1)), hash12(i + vec2(1, 1)), f.x), f.y);
}
float fbm(vec2 p) {
  float s = 0.0, a = 0.5;
  for (int i = 0; i < 5; i++) { s += a * vnoise(p); p = p * 2.03 + 17.1; a *= 0.5; }
  return s;
}
/**
 * A star on a plane: the plane is cut into cells and a few of them hold one at their
 * center. What it is worth, round and falling off from there, or nothing.
 *
 * Filling the cell instead - which is how the deck, the void and the sky were all
 * drawn - gives a grid of squares that grow and shrink with the angle the surface is
 * seen at. A star that has shrunk below a pixel is widened to a pixel here and dimmed
 * by exactly as much as it was widened, so a field of them fades out into the
 * distance instead of flickering. The derivative is taken before anything branches on
 * the cell, because a derivative asked for inside a branch is not defined.
 */
float starDot(vec2 q, float rarity, float size) {
  float px = length(fwidth(q));
  vec2 cell = floor(q);
  if (hash12(cell + 5.0) < rarity) return 0.0;
  // How much the widening costs it, worked out before the width is capped: a star
  // seen from far enough away has to go out altogether, or a field of them reaching
  // into the distance ends as an even carpet of speckle.
  float fade = (size * size) / max(size * size, px * px);
  float w = min(max(size, px), 0.3);
  float r = length(fract(q) - 0.5);
  return exp(-(r * r) / (w * w)) * fade * (0.25 + 0.75 * hash12(cell + 3.0));
}
`;

// ------------------------------------------------------------------ box shaders

/** Vertex declarations for city materials (before main). */
export const CITY_VERT_HEAD = `
attribute float aKind;
attribute float aFade;     // 1: dimmed, drawn plain (MapScene.setColors)
attribute vec3 aBoxCenter; // base center, for non-instanced boxes
attribute vec3 aBoxSize;
attribute vec4 aBuild;     // the building's type, variant, lift and full height (buildings.js)
attribute float aDetail;   // 1 where details.js has built the box's balconies and gear
varying vec3 vLP;          // position relative to the box's base center, world units
varying vec3 vObjN;
varying vec3 vWorld;       // where the fragment is drawn, for the angle it is seen at
flat varying vec4 vBuild;
flat varying float vDetail;
// Per box. Flat: interpolation noise in a seed, run through a hash, speckles windows.
flat varying vec3 vSize;
flat varying float vKind;
flat varying float vFade;
flat varying vec2 vSeed;
`;

/** Vertex body (inside main, where the transformed position is known). */
export const CITY_VERT_BODY = `
#ifdef USE_INSTANCING
  vSize = vec3(length(instanceMatrix[0].xyz), length(instanceMatrix[1].xyz), length(instanceMatrix[2].xyz));
  vLP = transformed * vSize;
  vSeed = instanceMatrix[3].xz;
  vWorld = bendWorld((modelMatrix * instanceMatrix * vec4(transformed, 1.0)).xyz);
#else
  vSize = aBoxSize;
  vLP = transformed - aBoxCenter;
  vSeed = aBoxCenter.xz;
  vWorld = bendWorld((modelMatrix * vec4(transformed, 1.0)).xyz);
#endif
  vBuild = aBuild;
  vDetail = aDetail;
  vObjN = normal;
  vKind = aKind;
  vFade = aFade;
`;

/** Fragment declarations: surface functions, all in linear color. */
// Implements: REQ-CITY-001, REQ-CITY-002
export const CITY_FRAG_HEAD = NOISE_GLSL + looksGLSL() + `
uniform float uBend;
uniform float uNight;
// 0 city, 1 circuit board, 2 galaxy. The boxes' material is compiled once per style
// (MapScene.bendable) with the style as a constant, so each program carries only its
// own painters: a uniform branch still costs its code on some GPUs, and in the
// software renderer headless browsers use it costs as though every branch ran.
#ifdef CITY_STYLE
#define uStyle CITY_STYLE
#else
uniform float uStyle;
#endif
varying vec3 vLP;
varying vec3 vObjN;
varying vec3 vWorld;
flat varying vec4 vBuild;
flat varying float vDetail;
flat varying vec3 vSize;
flat varying float vKind;
flat varying float vFade;
flat varying vec2 vSeed;

const float FADE = 0.72; // how far a dimmed box is blended towards its plain color

vec3 cityTexture(vec3 base);

// 1 where t lies in [lo, hi], with edges softened over w (antialiasing).
float band(float t, float lo, float hi, float w) {
  return smoothstep(lo - w, lo + w, t) - smoothstep(hi - w, hi + w, t);
}
float dark(float night) { return mix(1.0, night, uNight); }

// The pixel footprints the ground's textures are antialiased by, taken together:
// streets() paints only the layers a pixel shows, and a derivative asked for inside a
// branch its neighbors do not all take is undefined.
struct Footprint {
  vec2 paving;  // of p / 0.18
  vec2 grass;   // of p * 60
  vec2 asphalt; // of p * 40
  vec2 mowing;  // of p / 0.35
  vec2 paths;   // of p / 2.2
};
Footprint footprint(vec2 p) {
  return Footprint(fwidth(p / 0.18), fwidth(p * 60.0), fwidth(p * 40.0), fwidth(p / 0.35), fwidth(p / 2.2));
}

vec3 paving(vec3 base, vec2 p, vec2 footprint) {
  vec2 t = p / 0.18, w = footprint + 1e-4;
  float far = smoothstep(0.2, 0.5, max(w.x, w.y));
  float joint = (1.0 - band(fract(t.x), 0.07, 0.93, w.x) * band(fract(t.y), 0.07, 0.93, w.y)) * (1.0 - far);
  vec3 c = mix(vec3(0.46, 0.45, 0.42), base, 0.3) * (0.92 + 0.16 * mix(hash12(floor(t)), 0.5, far));
  return mix(c, c * 0.72, joint) * dark(0.42);
}
vec3 paving(vec3 base, vec2 p) { return paving(base, p, fwidth(p / 0.18)); }

// A lawn: one green with gentle, large variation and a few drier patches, and blades
// up close. Bushes and trees are real geometry (makeProps), not painted on.
// Implements: REQ-CITY-003, REQ-CITY-023
vec3 grass(vec3 base, vec2 p, vec2 footprint) {
  vec2 w = footprint;
  float far = smoothstep(0.3, 1.0, max(w.x, w.y));
  vec3 g = vec3(0.12, 0.24, 0.05) * (0.9 + 0.12 * vnoise(p * 0.6) + 0.06 * vnoise(p * 3.1));
  g = mix(g, vec3(0.2, 0.28, 0.08), 0.3 * smoothstep(0.6, 0.85, vnoise(p * 1.7 + 4.0)));
  g *= 0.85 + 0.3 * (far < 1.0 ? mix(vnoise(p * 60.0), 0.5, far) : 0.5); // no blades to look up from afar
  return mix(g, base, 0.08) * dark(0.35);
}
vec3 grass(vec3 base, vec2 p) { return grass(base, p, fwidth(p * 60.0)); }

// Implements: REQ-CITY-003, REQ-CITY-015
vec3 asphalt(vec2 p, vec2 footprint) {
  vec2 w = footprint;
  float far = smoothstep(0.3, 1.0, max(w.x, w.y));
  // Grain and cracks fade out with distance, and are not looked up once they have.
  float grain = far < 1.0 ? mix(0.55 * vnoise(p * 40.0) + 0.45 * vnoise(p * 9.0), 0.5, far) : 0.5;
  vec3 c = vec3(0.034, 0.036, 0.041) * (0.75 + 0.5 * grain);
  c *= 1.0 - 0.2 * smoothstep(0.6, 0.64, fbm(p * 0.8 + 3.7));                 // repaired patches
  if (far < 1.0) {
    float crack = 1.0 - smoothstep(0.0, 0.012, abs(fbm(p * 2.6) - 0.5));
    c *= 1.0 - 0.4 * crack * (1.0 - far) * smoothstep(0.45, 0.6, vnoise(p * 1.3)); // cracks, here and there
  }
  return c;
}

// A cell's obstacles: the nearest footprints around it (up to 8, as indices into
// uRoadRects), written by setRoads.
uniform highp sampler2D uRoadIdx;
uniform highp sampler2D uRoadRects;
uniform vec4 uRoadGrid; // grid origin x, z; cells per unit; width of uRoadRects
uniform float uRoadOn;
uniform vec3 uGroundRef; // a terrace's usual color (the palette's), and land's
uniform vec3 uLandRef;

// How a box's color differs from its kind's usual one, as a multiplier.
// Implements: REQ-CITY-004
vec3 tint(vec3 base, vec3 ref) { return clamp(base / max(ref, vec3(0.02)), 0.4, 2.2); }

const float SIDEWALK = 0.065;
const float CARRIAGE = 0.42; // farther from every obstacle than this is a park, not a street

// A pocket park where the packing left a hole: a mown lawn crossed by gravel paths
// (PARK_PATHS apart; makeProps keeps its bushes and trees off them).
// Implements: REQ-CITY-010, REQ-CITY-023
vec3 park(vec3 base, vec2 p, Footprint f) {
  vec3 c = grass(base, p, f.grass);
  vec2 m = p / 0.35, mw = f.mowing + 1e-4;
  c *= 1.0 + 0.05 * (band(fract(m.x), 0.0, 0.5, mw.x) * 2.0 - 1.0) * (1.0 - smoothstep(0.3, 0.6, mw.x)); // mowing stripes
  vec2 t = p / 2.2, w = f.paths + 1e-4;
  float path = max(band(fract(t.x), 0.47, 0.53, w.x), band(fract(t.y), 0.47, 0.53, w.y));
  vec3 gravel = vec3(0.42, 0.38, 0.3) * (0.85 + 0.3 * vnoise(p * 50.0)) * dark(0.4);
  c = mix(c, c * 0.8, max(band(fract(t.x), 0.455, 0.545, w.x), band(fract(t.y), 0.455, 0.545, w.y))); // worn edges
  return mix(c, gravel, path);
}

// What the free space of a terrace top looks like, measured once and painted by
// whichever style is in force: the nearest obstacle, the one facing it across the
// street, and where the two stop facing each other.
struct Road {
  vec2 p;      // world position
  float d1;    // distance to the nearest obstacle
  vec2 n1;     // the direction away from it
  float d2;    // to the obstacle facing it across the street; 1e9 when none does
  vec2 ends;   // along the street, where both sides face each other
  bool xRoad;  // the street runs along x
};

Road roadField(vec3 lp, vec3 sz) {
  vec2 p = vSeed + lp.xz; // world position
  vec2 h = sz.xz * 0.5;
  // Candidates: distance to the obstacle, direction from it to p, and the obstacle's
  // extent along the street it borders.
  float d[12]; vec2 n[12]; vec2 ext[12];
  for (int j = 0; j < 12; j++) { d[j] = 1e9; n[j] = vec2(0.0); ext[j] = vec2(0.0); }
  d[0] = lp.x + h.x; n[0] = vec2(1.0, 0.0);  ext[0] = vSeed.y + vec2(-h.y, h.y);
  d[1] = h.x - lp.x; n[1] = vec2(-1.0, 0.0); ext[1] = ext[0];
  d[2] = lp.z + h.y; n[2] = vec2(0.0, 1.0);  ext[2] = vSeed.x + vec2(-h.x, h.x);
  d[3] = h.y - lp.z; n[3] = vec2(0.0, -1.0); ext[3] = ext[2];
  if (uRoadOn > 0.5) {
    ivec2 size = textureSize(uRoadIdx, 0);
    ivec2 cell = clamp(ivec2(floor((p - uRoadGrid.xy) * uRoadGrid.z)), ivec2(0), ivec2(size.x / 2 - 1, size.y - 1));
    int rw = int(uRoadGrid.w);
    for (int t = 0; t < 2; t++) {
      vec4 ids = texelFetch(uRoadIdx, ivec2(cell.x * 2 + t, cell.y), 0);
      for (int k = 0; k < 4; k++) {
        if (ids[k] < 0.0) continue;
        int i = int(ids[k] + 0.5);
        vec4 r = texelFetch(uRoadRects, ivec2(i % rw, i / rw), 0);
        vec2 v = p - clamp(p, r.xy, r.zw);
        float dist = length(v);
        if (dist < 1e-4) continue; // inside: the terrace itself, or one it stands on
        int j = 4 + t * 4 + k;
        d[j] = dist;
        n[j] = v / dist;
        ext[j] = abs(n[j].x) > abs(n[j].y) ? r.yw : r.xz;
      }
    }
  }
  // The nearest, and the nearest facing it across the street, carried along rather
  // than looked up by index: an array indexed by a value known only at run time is
  // kept in memory rather than registers on most GPUs.
  float dA = d[0]; vec2 nA = n[0], extA = ext[0];
  for (int j = 1; j < 12; j++) if (d[j] < dA) { dA = d[j]; nA = n[j]; extA = ext[j]; }
  // The nearest obstacle facing it across the street, if the street is straight here.
  bool facing = false;
  float d2 = 1e9;
  vec2 extB = vec2(0.0);
  for (int j = 0; j < 12; j++) if (dot(n[j], nA) < -0.95 && d[j] < d2) { d2 = d[j]; extB = ext[j]; facing = true; }

  Road r;
  r.p = p;
  r.d1 = dA;
  r.n1 = nA;
  r.xRoad = abs(nA.x) < 0.5;
  r.d2 = 1e9;
  r.ends = vec2(0.0);
  // A park between two obstacles is two streets, not one.
  if (facing && dA + d2 <= 2.0 * CARRIAGE) {
    r.d2 = d2;
    r.ends = vec2(max(extA.x, extB.x), min(extA.y, extB.y));
  }
  return r;
}

// The top of a terrace as a city: asphalt between the buildings, sidewalks, crossings
// and pocket parks.
// Implements: REQ-CITY-006, REQ-CITY-007, REQ-CITY-008, REQ-CITY-009, REQ-CITY-010
vec3 streets(vec3 base, vec3 lp, vec3 sz) {
  Road r = roadField(lp, sz);
  vec2 p = r.p;
  float d1 = r.d1, d2 = r.d2;
  bool facing = d2 < 1e8;
  float width = d1 + d2, s = (d2 - d1) * 0.5; // s: distance from the center line
  float along = r.xRoad ? p.x : p.y, across = r.xRoad ? p.y : p.x;
  float w = fwidth(d1) + 1e-4, wa = fwidth(along) + 1e-4, ws = fwidth(s) + 1e-4, wx = fwidth(across);
  Footprint f = footprint(p);

  // How much of each layer the pixel shows, from the sidewalk out: each is painted
  // only where it shows, since each costs a handful of noise lookups.
  float edge = CARRIAGE + SIDEWALK;
  float kRoad = smoothstep(SIDEWALK - w, SIDEWALK + w, d1);
  float kPaving = band(d1, CARRIAGE, edge, w);
  float kPark = smoothstep(edge - w, edge + w, d1);

  vec3 c = vec3(0.0);
  if (kRoad > 0.0 && kPaving < 1.0 && kPark < 1.0) {
    c = asphalt(p, f.asphalt);
    if (!facing && d1 < CARRIAGE) {
      // A street along a park: the dashed line on its middle.
      float mid = (SIDEWALK + CARRIAGE) * 0.5;
      float dash = band(fract(along / 0.3), 0.0, 0.5, wa / 0.3) * band(d1, mid - 0.008, mid + 0.008, w);
      c = mix(c, vec3(0.78, 0.6, 0.12), dash * (1.0 - smoothstep(0.02, 0.06, wa)));
    }
    if (facing) {
      vec2 e = r.ends; // where both sides face each other
      float fromEnd = min(along - e.x, e.y - along);
      float near = 1.0 - smoothstep(0.02, 0.06, wa);
      // Wheel tracks, one pair per lane.
      c *= 1.0 - 0.14 * band(abs(s), width * 0.22 - 0.025, width * 0.22 + 0.025, ws) * step(0.3, width);
      float zebra = 0.0;
      if (e.y - e.x > 1.6 && fromEnd > 0.03 && fromEnd < 0.15 && d1 > SIDEWALK + 0.01) {
        zebra = band(fract(across / 0.05), 0.0, 0.5, wx / 0.05) * near;
        c = mix(c, vec3(0.62, 0.62, 0.6), zebra);
      }
      if (width > 0.28 && fromEnd > 0.2) {
        // A manhole every few meters on the center line, the dashed line between them.
        float m = length(vec2(s, (fract(along / 2.7 + 0.5) - 0.5) * 2.7));
        float lid = 1.0 - smoothstep(0.042 - w, 0.042 + w, m);
        c = mix(c, vec3(0.05, 0.05, 0.055) * (0.8 + 0.4 * band(fract(p.x * 40.0), 0.0, 0.5, 0.2)), lid * near);
        c = mix(c, vec3(0.1), band(m, 0.036, 0.042, w) * near);
        float dash = band(fract(along / 0.3), 0.0, 0.5, wa / 0.3) * (1.0 - smoothstep(0.008 - ws, 0.008 + ws, s));
        c = mix(c, vec3(0.78, 0.6, 0.12), dash * (1.0 - lid) * near);
      }
    }
    c *= dark(0.5);
  }
  // Sidewalk along every obstacle and edge, and around parks, behind pale curb stones.
  vec3 curb = vec3(0.5, 0.5, 0.48) * dark(0.45);
  vec3 paved = kRoad < 1.0 || kPaving > 0.0 ? paving(base, p, f.paving) : vec3(0.0);
  vec3 walk = mix(paved, curb, band(d1, SIDEWALK - 0.014, SIDEWALK, w));
  c = mix(walk, c, kRoad);
  c = mix(c, paved, kPaving);
  c = mix(c, curb, band(d1, CARRIAGE, CARRIAGE + 0.014, w));
  if (kPark > 0.0) c = mix(c, park(base, p, f), kPark);
  return c * tint(base, uGroundRef); // nesting levels alternate, hover shows
}

// ------------------------------------------------------------------ circuit board

// The board styles: a part sits at PAD from the copper that feeds it, which is the
// same measurement the city uses for its sidewalk.
const float PAD = 0.03;
const vec3 GLOW = vec3(0.25, 0.8, 0.95); // the galaxy's light, before it is tinted

// Solder mask over woven glass, which is what gives a board its color up close and
// its flat green from across the room.
vec3 solderMask(vec3 base, vec2 p) {
  vec2 t = p * 190.0, w = fwidth(t);
  float weave = mix(0.5 + 0.5 * sin(t.x) * sin(t.y), 0.5, smoothstep(0.4, 1.2, max(w.x, w.y)));
  vec3 c = vec3(0.018, 0.08, 0.045) * (0.8 + 0.4 * weave);
  // Mask is sprayed, not painted: it pools and thins across a board, and the copper
  // under it shows through where it is thin. Without this an empty stretch of board
  // is one dead green, which is what a large one mostly is.
  float pool = fbm(p * 2.2);
  c *= 0.86 + 0.28 * pool;
  c += vec3(0.022, 0.014, 0.004) * smoothstep(0.7, 0.96, pool);
  return mix(c, base * 0.3, 0.16) * dark(0.55);
}

// Bare substrate: the board's own fiberglass, where no mask was printed.
vec3 substrate(vec3 base, vec2 p) {
  vec2 t = p * 130.0, w = fwidth(t);
  float weave = mix(0.5 + 0.5 * sin(t.x) * sin(t.y), 0.5, smoothstep(0.4, 1.2, max(w.x, w.y)));
  vec3 c = vec3(0.3, 0.26, 0.12) * (0.78 + 0.44 * weave);
  return mix(c, base, 0.1) * dark(0.5);
}

// The top of a terrace as a board: a hatched ground pour over the open copper, traces
// down the middle of every street with vias along them, and round every part a ring of
// solder pads inside a silkscreen outline.
// Implements: REQ-MAP-054
vec3 traces(vec3 base, vec3 lp, vec3 sz) {
  Road r = roadField(lp, sz);
  vec2 p = r.p;
  float w = fwidth(r.d1) + 1e-4;
  vec3 mask = solderMask(base, p);
  vec3 copper = vec3(0.46, 0.27, 0.1) * (0.88 + 0.24 * vnoise(p * 70.0)) * dark(0.55);
  vec3 tin = vec3(0.68, 0.7, 0.75) * (0.86 + 0.28 * vnoise(p * 90.0)) * dark(0.5);
  vec3 silk = vec3(0.85, 0.87, 0.9) * dark(0.5);

  vec3 c = mask;
  // The ground pour, hatched. Once a pixel spans more than a line of it the hatch is
  // taken to its mean: left alone it beats against the pixel grid and a board seen
  // at a shallow angle is covered in moire.
  vec2 ht = vec2((p.x + p.y) / 0.085);
  float hw = fwidth(ht.x);
  float hatch = mix(band(fract(ht.x), 0.0, 0.6, hw), 0.6, smoothstep(0.35, 1.1, hw));
  c = mix(c, mix(mask, copper * 0.6, 0.45 + 0.55 * hatch), smoothstep(PAD + 0.02, PAD + 0.1, r.d1) * 0.7);

  if (r.d2 < 1e8) {
    float width = r.d1 + r.d2, s = (r.d2 - r.d1) * 0.5; // s: from the center line
    float ws = fwidth(s) + 1e-4;
    // Several traces side by side, as many as the street is wide enough for.
    float lanes = clamp(floor(width / 0.1), 1.0, 5.0);
    float pitch = width / (lanes + 1.0);
    float t = abs(mod(s + width * 0.5 + pitch * 0.5, pitch) - pitch * 0.5);
    c = mix(c, copper, (1.0 - smoothstep(0.011 - ws, 0.011 + ws, t)) * step(0.06, width));
    // Vias down the center of the street, every so often.
    float along = r.xRoad ? p.x : p.y;
    float m = length(vec2(s, (fract(along / 1.1 + 0.5) - 0.5) * 1.1));
    float via = step(0.2, width) * (1.0 - smoothstep(0.03, 0.03 + w * 2.0, m));
    c = mix(c, tin, via);
    c = mix(c, vec3(0.03, 0.035, 0.045), via * (1.0 - smoothstep(0.013, 0.013 + w * 2.0, m)));
  }
  // The footprint of the part: pads against it, a silkscreen outline around them.
  c = mix(c, tin, band(r.d1, 0.0, PAD, w));
  c = mix(c, silk, band(r.d1, PAD + 0.012, PAD + 0.024, w));
  return c * tint(base, uGroundRef);
}

// A chip package: black epoxy with a parting line and a row of tin pins along the foot
// of every face. A tall part is a heatsink instead, finned from top to bottom.
// Implements: REQ-MAP-053
vec3 chipFace(vec3 base, float u, float faceW, float v, float h, vec2 seed) {
  float wv = fwidth(v) + 1e-4, wu = fwidth(u) + 1e-4;
  if (h > 2.2) {
    vec3 metal = mix(vec3(0.38, 0.4, 0.43), base * 0.7, 0.3);
    vec2 ft = vec2(u / 0.055), fw = fwidth(ft);
    float fin = band(fract(ft.x), 0.1, 0.9, fw.x);
    vec3 c = metal * mix(0.62, 1.12, fin) * (0.92 + 0.16 * vnoise(vec2(u, v) * 30.0));
    c = mix(c, metal * 0.5, band(v, 0.0, 0.1, wv)); // the base it is bolted to
    return c * dark(0.5);
  }
  vec3 epoxy = mix(vec3(0.05, 0.052, 0.06), base * 0.35, 0.14) * (0.9 + 0.2 * vnoise(vec2(u, v) * 45.0));
  vec3 c = epoxy * dark(0.55);
  c = mix(c, epoxy * 1.9, band(v, h * 0.5 - 0.005, h * 0.5 + 0.005, wv)); // the mold parting line
  float pinH = min(0.08, h * 0.32);
  float pins = band(fract(u / 0.05), 0.18, 0.82, wu / 0.05) * band(v, 0.0, pinH, wv);
  vec3 tin = vec3(0.7, 0.72, 0.76) * (0.85 + 0.3 * vnoise(vec2(u, v) * 110.0)) * dark(0.5);
  c = mix(c, tin, pins * (1.0 - smoothstep(0.25, 0.7, wu / 0.05)));
  // A silkscreen part number, two bars of it, on the upper half of the body.
  vec2 lt = vec2(u / 0.028, (v - h * 0.62) / 0.05);
  vec2 lw = fwidth(lt) + 1e-4;
  float label = band(fract(lt.x), 0.15, 0.7, lw.x) * band(lt.y, 0.0, 0.55, lw.y)
    * step(0.35, hash12(floor(vec2(lt.x, lt.y * 2.0)) + seed)) * step(h * 0.62, v) * step(v, h * 0.62 + 0.05);
  c = mix(c, vec3(0.8, 0.82, 0.85) * dark(0.5), label * (1.0 - smoothstep(0.3, 0.8, lw.x)));
  return c;
}

// The top of a package: matte epoxy, a bevel, the pin-1 dimple and a printed code.
vec3 chipTop(vec3 base, vec3 lp, vec3 sz, float e) {
  float w = fwidth(e) + 1e-4;
  vec3 c = mix(vec3(0.055, 0.057, 0.065), base * 0.32, 0.14) * (0.92 + 0.16 * vnoise(lp.xz * 45.0 + vSeed));
  c = mix(c, c * 1.7, 1.0 - smoothstep(0.022 - w, 0.022 + w, e));
  vec2 at = -sz.xz * 0.5 + vec2(min(0.1, sz.x * 0.25), min(0.1, sz.z * 0.25));
  c = mix(c, c * 0.4, 1.0 - smoothstep(0.028 - w, 0.028 + w, length(lp.xz - at)));
  vec2 lt = lp.xz / vec2(0.03, 0.07);
  vec2 lw = fwidth(lt) + 1e-4;
  float code = band(fract(lt.x), 0.15, 0.72, lw.x) * band(fract(lt.y), 0.3, 0.62, lw.y)
    * step(0.4, hash12(floor(lt) + vSeed)) * step(0.045, e);
  c = mix(c, vec3(0.78, 0.8, 0.84), code * (1.0 - smoothstep(0.3, 0.8, max(lw.x, lw.y))));
  return c * dark(0.55);
}

// The edge of the board: its layers, copper between prepreg, with the mask on top.
// Implements: REQ-MAP-054
vec3 boardEdge(vec3 base, vec2 p) {
  float t = p.y / 0.05, w = fwidth(t) + 1e-4;
  vec3 c = vec3(0.28, 0.24, 0.11) * (0.85 + 0.3 * vnoise(p * 60.0));
  c = mix(c, vec3(0.46, 0.28, 0.11), band(fract(t), 0.0, 0.16, w));
  c = mix(c, vec3(0.018, 0.08, 0.045), step(0.94, fract(t * 0.25)));
  return mix(c, base * 0.35, 0.16) * dark(0.5);
}

// ------------------------------------------------------------------ galaxy

// The deck of a platform out in the dark: dust, a drift of stars, and the light of
// whatever runs underneath it.
vec3 dust(vec3 base, vec2 p) {
  vec3 c = vec3(0.028, 0.026, 0.058) * (0.7 + 0.6 * vnoise(p * 7.0));
  c += vec3(0.19, 0.07, 0.3) * smoothstep(0.42, 0.95, fbm(p * 0.4));
  c += vec3(0.75, 0.8, 1.0) * starDot(p * 55.0, 0.965, 0.16) * 0.9;
  c += vec3(1.0, 0.88, 0.7) * starDot(p * 17.0 + 5.0, 0.99, 0.12) * 1.3;
  return mix(c, base * 0.35, 0.1);
}

// The top of a terrace as a platform: plating, with a lit conduit down the middle of
// every gap and light along every edge.
// Implements: REQ-MAP-056
vec3 conduits(vec3 base, vec3 lp, vec3 sz) {
  Road r = roadField(lp, sz);
  vec2 p = r.p;
  float w = fwidth(r.d1) + 1e-4;
  vec3 c = vec3(0.022, 0.02, 0.05) * (0.75 + 0.5 * vnoise(p * 11.0));
  c += vec3(0.1, 0.04, 0.2) * smoothstep(0.5, 0.95, fbm(p * 0.6));
  vec2 pt = p / 0.5, pw = fwidth(pt) + 1e-4;
  float seam = max(band(fract(pt.x), 0.0, 0.02, pw.x), band(fract(pt.y), 0.0, 0.02, pw.y));
  c += vec3(0.04, 0.05, 0.08) * seam; // plating seams
  if (r.d2 < 1e8) {
    float s = (r.d2 - r.d1) * 0.5, ws = fwidth(s) + 1e-4;
    float core = 1.0 - smoothstep(0.013 - ws, 0.013 + ws, abs(s));
    c += GLOW * (core * 0.9 + exp(-abs(s) * 22.0) * 0.35);
  }
  c += GLOW * 0.7 * band(r.d1, 0.0, 0.022, w); // every platform is rimmed with light
  return c * tint(base, uGroundRef);
}

// A spire: faceted, dark at the foot and lit towards the tip, with strata of light
// across it and a scatter of windows that read as stars.
// Implements: REQ-MAP-056
vec3 crystalFace(vec3 base, float u, float faceW, float v, float h, vec2 seed) {
  float t = clamp(v / max(h, 1e-3), 0.0, 1.0);
  vec3 c = mix(base * 0.14, base * 0.62, t);
  float facet = floor(u / 0.2);
  c *= 0.8 + 0.34 * hash12(vec2(facet, floor(v / 0.9)) + seed);
  float bandT = v / 0.45;
  c += GLOW * band(fract(bandT), 0.0, 0.07, fwidth(bandT)) * 0.45;
  vec2 cell = vec2(u / 0.11, v / 0.15);
  vec2 cw = fwidth(cell) + 1e-4;
  float id = hash12(floor(cell) + seed);
  float win = band(fract(cell.x), 0.25, 0.75, cw.x) * band(fract(cell.y), 0.3, 0.7, cw.y);
  c += win * step(mix(0.93, 0.82, uNight), id) * vec3(0.85, 0.92, 1.0) * 0.9;
  c += GLOW * 0.55 * smoothstep(0.82, 1.0, t); // the tip glows
  return c;
}

vec3 crystalTop(vec3 base, vec3 lp, vec3 sz, float e) {
  float w = fwidth(e) + 1e-4;
  vec3 c = base * 0.45 + GLOW * 0.1;
  c += GLOW * 0.7 * (1.0 - smoothstep(0.035 - w, 0.035 + w, e));
  c *= 0.9 + 0.2 * vnoise(lp.xz * 22.0 + vSeed);
  return c;
}

// The side of a platform: rock with the same light running through it in veins.
vec3 crystalWall(vec3 base, vec2 p) {
  vec3 c = base * 0.16 * (0.75 + 0.5 * vnoise(p * 13.0));
  float vein = 1.0 - smoothstep(0.0, 0.022, abs(fbm(p * 2.4) - 0.5));
  c += GLOW * vein * 0.4;
  return c;
}

// Terrace sides carry a staircase near one end of each long side (the other end is
// where rampsFor puts a ramp): stairs and ramps show where the levels connect. The
// stairs are paint - a terrace wall is higher than a step, so the walker takes the
// ramp or jumps.
// Implements: REQ-CITY-011
vec3 stairs(vec3 c, float u, float faceW, float v, float h) {
  u -= faceW * 0.5 - 0.34;
  if (faceW < 1.5 || abs(u) > 0.16) return c;
  float wv = fwidth(v) + 1e-4, wu = fwidth(u) + 1e-4;
  float f = fract(v / (h / 4.0));
  vec3 tread = mix(vec3(0.42, 0.41, 0.39), vec3(0.58, 0.57, 0.55), smoothstep(0.7, 0.8, f)) * dark(0.45);
  tread *= 1.0 - 0.3 * band(f, 0.0, 0.08, wv * 4.0 / h);
  return mix(tread, vec3(0.2) * dark(0.5), band(abs(u), 0.13, 0.16, wu)); // stringers
}

// ------------------------------------------------------------------ the city's buildings

// Which type a box is drawn as (buildings.js): the facades and roofs below are laid
// out by it. Every material is the box's own color at some brightness, or a neutral
// one (glass, frames, metal) over a small share of the face: the type is read from
// the form, never from a hue.

float luma(vec3 c) { return dot(c, vec3(0.2126, 0.7152, 0.0722)); }

// What a window reflects: the reflected ray's height picks between the street, the
// horizon and the sky, gray more than blue, so glass never reads as a color of its
// own. At night the sky in the glass is as dark as the sky.
vec3 skyIn(vec3 n, vec3 view) {
  vec3 r = reflect(-view, n);
  vec3 day = mix(vec3(0.3, 0.31, 0.32), vec3(0.62, 0.68, 0.76), smoothstep(-0.05, 0.5, r.y));
  day *= mix(0.55, 1.0, smoothstep(-0.35, 0.0, r.y));
  vec3 night = mix(vec3(0.012, 0.013, 0.018), vec3(0.03, 0.035, 0.055), smoothstep(-0.05, 0.5, r.y));
  return mix(day, night, uNight);
}

// Glass reflects more the flatter it is seen (Fresnel): head on it shows the room,
// along the street the sky.
float fresnel(vec3 n, vec3 view) {
  float c = clamp(dot(n, view), 0.0, 1.0);
  return 0.06 + 0.94 * pow(1.0 - c, 5.0);
}

// A room's light: warm or cool by floor (an office floor is lit by tubes, a flat by
// lamps), dimmer by day, when it has the daylight to compete with.
vec3 lampFor(float floorId, float id, float coolShare) {
  vec3 warm = vec3(1.0, 0.72, 0.38), cool = vec3(0.74, 0.86, 1.0);
  return mix(warm, cool, step(floorId, coolShare)) * (0.55 + 0.45 * id) * mix(0.4, 1.0, uNight);
}

// One pane: the room behind it and the sky in front of it. The room is darker towards
// its middle, where the eye sees deepest into it, and a lit one brightest there too.
// One window in three has blinds drawn part-way down and one in four curtains to the
// sides; their slats and folds go before the pane does. f: where in the pane (0..1
// across and up); fw: the pixel footprint of that.
vec3 pane(vec3 base, vec2 f, vec2 fw, float id, float tint, vec3 lamp, float lit, vec3 sky, float glancing) {
  vec2 e = min(f, 1.0 - f);
  float depth = smoothstep(0.0, 1.0, min(e.x, e.y) * 2.0);
  vec3 room = vec3(0.05, 0.051, 0.055) * (1.0 - 0.6 * depth);
  room = mix(room, lamp * (0.75 + 0.25 * depth), lit);
  float fine = 1.0 - smoothstep(0.06, 0.16, max(fw.x, fw.y));
  float kind = fract(id * 7.13);
  if (kind < 0.34) {
    float drop = 0.15 + 0.75 * fract(id * 3.71);
    float slat = mix(0.7, band(fract(f.y * 10.0), 0.15, 0.85, fw.y * 10.0), fine);
    vec3 blind = mix(vec3(0.46, 0.45, 0.42) * dark(0.2), lamp * 0.55, lit) * (0.72 + 0.28 * slat);
    room = mix(room, blind, smoothstep(1.0 - drop - fw.y, 1.0 - drop + fw.y, f.y));
  } else if (kind < 0.6) {
    float side = 0.1 + 0.2 * fract(id * 5.3);
    float fold = mix(0.5, 0.5 + 0.5 * sin(f.x * 50.0), fine * (1.0 - smoothstep(0.02, 0.06, fw.x)));
    vec3 cloth = mix(vec3(0.5, 0.48, 0.44) * dark(0.2), lamp * 0.62, lit) * (0.78 + 0.22 * fold);
    room = mix(room, cloth, 1.0 - band(f.x, side, 1.0 - side, fw.x));
  }
  // Tinted glass carries the building's own color at the sky's brightness.
  vec3 glass = sky * mix(vec3(1.0), base / max(max(base.r, max(base.g, base.b)), 0.05), tint);
  return mix(room, glass, clamp(glancing * (1.0 - 0.7 * lit) + tint * 0.3, 0.0, 1.0));
}

/**
 * What a facade averages to once its windows are below a pixel: the wall at its
 * type's brightness and the glass over its type's share of the face. buildings.js
 * facadeFar is the same sum, which is what the tests hold the type table to.
 */
vec3 facadeFar(vec3 base, int look, float variant) {
  float litShare = mix(0.08, 0.4, uNight);
  float lampOn = litShare * mix(0.6, 0.5, uNight);
  vec3 wall = base * LOOK_TONE[look] * (0.92 + 0.16 * variant) * dark(0.5);
  vec3 glass = mix(LOOK_GLASS, base * 0.45, LOOK_TINT[look]);
  vec3 window = glass * dark(0.12) * (1.0 - lampOn) + LOOK_LAMP * 0.8 * lampOn;
  return mix(wall, window, LOOK_SHARE[look]);
}

// English bond: a course of stretchers, then a course of headers half as long and
// centered on the joints below, with mortar lighter than the brick; each brick a
// little lighter or darker than the next, never another color.
vec3 brickBond(vec3 wall, float u, float v, vec2 seed) {
  float course = v / 0.022;
  float row = floor(course);
  float header = step(0.5, mod(row, 2.0));
  float brick = mix(0.064, 0.032, header);
  float x = u / brick + 0.5 * header;
  vec2 bw = vec2(fwidth(u) / brick, fwidth(course)) + 1e-4;
  float bfar = smoothstep(0.25, 0.55, max(bw.x, bw.y));
  float mortar = 1.0 - band(fract(x), 0.06, 0.94, bw.x) * band(fract(course), 0.14, 0.86, bw.y);
  wall *= mix(0.84 + 0.32 * hash12(vec2(floor(x), row) + seed), 1.0, bfar);
  return mix(wall, wall * 1.3 + 0.018, mortar * (1.0 - bfar) * 0.55);
}

// A canvas awning over a shop window: stripes of the building's color, darkened,
// and off-white - or plain - with a scalloped hem. y: 0 at the hem, 1 at the wall.
vec3 awning(vec3 base, float u, float y, float striped) {
  float s = u / 0.034, ws = fwidth(s) + 1e-4;
  float stripe = mix(band(fract(s), 0.0, 0.5, ws), 0.5, smoothstep(0.3, 0.8, ws)) * striped;
  vec3 c = mix(base * 0.5, vec3(0.78, 0.77, 0.73), stripe * 0.8) * dark(0.4);
  c *= 0.8 + 0.2 * y; // the canvas slopes away from the light at the hem
  return c;
}

// Large-scale shading, at any distance: darker where two faces meet and, on the
// ground floor, towards the foot, with grime splashed up it.
vec3 facadeShade(vec3 c, float u, float v, float edge, vec2 seed) {
  c *= mix(0.84, 1.0, smoothstep(0.0, 0.05, edge));
  if (vBuild.z < 0.01) {
    float grime = mix(vnoise(vec2(u * 12.0, 3.0) + seed), 0.5, smoothstep(0.3, 1.0, fwidth(u * 12.0)));
    c *= mix(0.68 + 0.08 * grime, 1.0, smoothstep(0.0, 0.2, v));
  }
  return c;
}

/**
 * A facade: whole bays across the face, STORY-high stories, laid out by the
 * building's type.
 *
 * - residential: framed windows, and on every bay or every other one a French
 *   window onto a balcony, whose railing is painted where details.js has built
 *   none;
 * - office: a curtain wall of tinted glass between dark mullions, a spandrel of the
 *   building's color at each floor, a glass lobby;
 * - brick: English bond, sash windows with stone lintels and sills, a string course
 *   and a toothed cornice;
 * - panel: precast panels with their seams showing, one window each;
 * - deco: piers running the full height, dark spandrels, and a stone band at each
 *   setback;
 * - warehouse: ribbed cladding, clerestory windows under the eaves, roll-up doors;
 * - mixed: a stone-faced shop floor (two on taller ones) under flats or brick.
 *
 * Ground floors have shopfronts under awnings and a door, or a lobby. Stains run down
 * from the windows, the foot and the corners are darker, and the space under a
 * cornice is in its shadow. At night rooms are lit warm or cool by floor.
 *
 * v: height above the building's ground; H: the building's full height; top: the
 * height of this piece's roof (below H on the lower tiers of a setback tower).
 *
 * Implements: REQ-CITY-013, REQ-CITY-032
 */
vec3 facade(vec3 base, float u, float faceW, float v, float H, float roofAt, vec2 seed, vec3 n) {
  int look = int(vBuild.x + 0.5);
  int type = look;
  float variant = vBuild.y;
  float podiumTop = STORY * (H > 1.6 ? 2.0 : 1.0);
  bool podium = type == MIXED && v < podiumTop;
  if (type == MIXED) type = variant < 0.5 ? RESIDENTIAL : BRICK;
  float bayW = type == OFFICE ? 0.13 : type == PANEL ? 0.3 : type == WAREHOUSE ? 0.45 : type == DECO ? 0.19 : type == BRICK ? 0.22 : 0.26;
  float bays = max(1.0, floor(faceW / bayW));
  vec2 cell = vec2((u + faceW * 0.5) / (faceW / bays), v / STORY);
  vec2 f = fract(cell), w = fwidth(cell) + 1e-4;
  vec2 bay = floor(cell);
  float far = smoothstep(0.3, 0.7, max(w.x, w.y));
  float id = hash12(bay + seed * 1.37);
  float top = roofAt - v;             // how far below this piece's roof
  float wv = fwidth(v) + 1e-4;
  float edge = faceW * 0.5 - abs(u);
  // Seen from far enough away there is nothing to draw but the average: most of a
  // map's facades, most of the time, and the one path that has to be cheap.
  if (far > 0.99) return facadeShade(facadeFar(base, look, variant), u, v, edge, seed);
  bool ground = bay.y < 0.5;
  bool cornice = top < 0.075;
  vec3 view = normalize(cameraPosition - vWorld);
  vec3 sky = skyIn(n, view);
  float glancing = fresnel(n, view);

  // The wall: the box's color at the type's brightness, a little lighter or darker
  // and rougher or smoother per building.
  float rough = 0.6 + 0.8 * fract(variant * 7.7);
  vec3 wall = base * LOOK_TONE[look] * (0.92 + 0.16 * variant);
  wall *= 1.0 + 0.12 * rough * (vnoise(vec2(u, v) * 22.0 + seed) - 0.5) * (1.0 - far);
  if (podium) {
    // Stone facing: smooth, lighter, with rusticated joints.
    wall *= 1.12;
    float joint = band(fract(v / 0.075), 0.0, 0.08, wv / 0.075);
    wall *= 1.0 - 0.25 * joint * (1.0 - smoothstep(0.2, 0.5, wv / 0.075));
  } else if (type == BRICK) {
    wall = brickBond(wall, u, v, seed);
  } else if (type == PANEL) {
    wall *= mix(0.93 + 0.14 * hash12(bay + seed * 2.1), 1.0, far);
    float seam = 1.0 - band(f.x, 0.015, 0.985, w.x) * band(f.y, 0.02, 0.98, w.y);
    wall *= 1.0 - 0.38 * seam * (1.0 - far);
    wall *= 1.0 + 0.08 * (vnoise(vec2(u, v) * 90.0 + seed) - 0.5) * (1.0 - smoothstep(0.3, 1.0, fwidth(u * 90.0)));
  } else if (type == WAREHOUSE) {
    float rib = u / 0.028, rw = fwidth(rib);
    wall *= 0.9 + 0.2 * mix(0.5 + 0.5 * cos(rib * 6.2832), 0.5, smoothstep(0.3, 0.8, rw));
  } else if (type == DECO) {
    float pier = 1.0 - band(f.x, 0.16, 0.84, w.x);
    wall *= mix(1.0, 1.14, pier * (1.0 - far));
  }
  wall *= dark(0.5);

  // The windows: where in the bay, how framed, how divided.
  vec4 rect = vec4(0.2, 0.8, 0.28, 0.86);   // x0, x1, y0, y1 within the bay
  float frameW = 0.035, mullion = 1.0, transom = 0.0;
  vec3 frame = mix(wall, vec3(0.62, 0.62, 0.6) * dark(0.45), 0.6);
  float tint = LOOK_TINT[look];
  float balcony = 0.0;
  if (type == OFFICE) { rect = vec4(0.04, 0.96, 0.25, 0.97); frameW = 0.0; mullion = 0.0; }
  else if (type == BRICK) { rect = vec4(0.27, 0.73, 0.26, 0.8); frameW = 0.05; transom = 0.55; }
  else if (type == PANEL) { rect = vec4(0.19, 0.81, 0.3, 0.8); frameW = 0.03; frame = mix(wall, vec3(0.5) * dark(0.45), 0.6); }
  else if (type == DECO) { rect = vec4(0.23, 0.77, 0.2, 0.9); frameW = 0.02; transom = 0.82; frame = vec3(0.12, 0.12, 0.13) * dark(0.5); }
  else if (type == RESIDENTIAL) {
    // French windows onto the balconies: on every bay, or every other one.
    float every = variant < 0.5 ? 1.0 : 2.0;
    balcony = step(mod(bay.x + floor(variant * 10.0), every), 0.5);
    if (balcony > 0.5) rect = vec4(0.24, 0.76, 0.04, 0.86);
  }
  float upper = (ground || cornice || podium) ? 0.0 : 1.0;
  if (type == WAREHOUSE) upper = 0.0;
  float inWin = band(f.x, rect.x, rect.y, w.x) * band(f.y, rect.z, rect.w, w.y) * upper;
  float inFrame = band(f.x, rect.x - frameW, rect.y + frameW, w.x) * band(f.y, rect.z - frameW, rect.w + frameW, w.y) * upper * (1.0 - inWin);
  vec2 size = rect.yw - rect.xz;
  vec2 pf = (f - rect.xz) / size, pfw = w / size;
  float fine = 1.0 - smoothstep(0.08, 0.2, max(pfw.x, pfw.y));
  float bars = max(mullion * band(pf.x, 0.47, 0.53, pfw.x), transom > 0.0 ? band(pf.y, transom - 0.025, transom + 0.025, pfw.y) : 0.0) * fine;

  // The rooms: lit by some share of them, whole floors of an office at once.
  float floorId = hash12(vec2(bay.y, seed.x * 0.13 + seed.y * 0.71));
  float coolShare = type == OFFICE ? 0.75 : type == PANEL ? 0.35 : type == DECO ? 0.5 : 0.15;
  vec3 lamp = lampFor(floorId, id, coolShare);
  float litShare = mix(0.08, 0.4, uNight);
  float lit = step(1.0 - litShare, id);
  if (type == OFFICE || type == DECO) lit = max(lit, step(1.0 - mix(0.02, 0.25, uNight), fract(floorId * 13.7)));

  vec3 c = wall;
  if (type == OFFICE && upper > 0.5) {
    // Between the panes, dark mullions; under them, the spandrel.
    float mull = 1.0 - band(f.x, 0.04, 0.96, w.x);
    c = mix(c, vec3(0.08, 0.085, 0.09) * dark(0.6), mull * (1.0 - far));
    c = mix(c, c * 0.75, band(f.y, 0.21, 0.25, w.y) * (1.0 - far));
  }
  if (type == DECO && upper > 0.5) {
    // Dark spandrels between the windows of a bay, the piers left standing proud.
    float inner = band(f.x, 0.16, 0.84, w.x);
    float span = inner * (1.0 - band(f.y, rect.z, rect.w, w.y));
    c = mix(c, c * 0.62, span * (1.0 - far));
    c = mix(c, c * 1.5, span * band(f.y, 0.06, 0.09, w.y) * (1.0 - far)); // a thin bright rule across it
  }
  // Sills under the windows, and a stone lintel over them in brick.
  if (type != OFFICE && type != DECO && upper > 0.5) {
    float sill = band(f.x, rect.x - 0.05, rect.y + 0.05, w.x) * band(f.y, rect.z - frameW - 0.05, rect.z - frameW, w.y);
    c = mix(c, wall * 1.3 + 0.02 * dark(0.5), sill * (1.0 - balcony));
    if (type == BRICK) {
      float lintel = band(f.x, rect.x - 0.06, rect.y + 0.06, w.x) * band(f.y, rect.w + frameW, rect.w + frameW + 0.08, w.y);
      c = mix(c, mix(wall, vec3(0.7, 0.69, 0.66) * dark(0.45), 0.6), lintel);
    }
  }
  c = mix(c, frame, inFrame);
  if (inWin > 0.0) c = mix(c, mix(pane(base, pf, pfw, id, tint, lamp, lit, sky, glancing), frame, bars), inWin);
  // A painted balcony on the French windows: its slab edge and a railing of bars.
  if (balcony > 0.5 && upper > 0.5 && vDetail < 0.5) {
    float rail = band(f.x, 0.08, 0.92, w.x) * band(f.y, 0.0, 0.33, w.y);
    float bar = band(fract(f.x * bays * faceW / 0.035), 0.0, 0.3, w.x * bays * faceW / 0.035);
    vec3 railing = mix(c * 0.55, vec3(0.1, 0.1, 0.11) * dark(0.6), 0.6 * bar * (1.0 - smoothstep(0.1, 0.3, w.x * 20.0)));
    railing = mix(railing, wall * 1.2, band(f.y, 0.0, 0.07, w.y)); // the slab's edge
    railing = mix(railing, vec3(0.2, 0.2, 0.21) * dark(0.6), band(f.y, 0.3, 0.33, w.y)); // the handrail
    c = mix(c, railing, rail * (1.0 - far));
  }

  // The ground floor: a lobby, loading doors, or shops under awnings with a door.
  // The door's bay, by a sum details.js can do too: it keeps its awnings off it.
  float faceId = n.x > 0.5 ? 0.0 : n.x < -0.5 ? 1.0 : n.z > 0.5 ? 2.0 : 3.0;
  float doorBay = floor(fract(variant * 7.31 + faceId * 0.23) * bays);
  float isDoor = 1.0 - step(0.5, abs(bay.x - doorBay));
  if (ground && type != WAREHOUSE) {
    vec3 shopLamp = vec3(1.0, 0.8, 0.55) * mix(0.2, 0.9, uNight);
    if (type == OFFICE) {
      float glassFront = band(f.x, 0.03, 0.97, w.x) * band(f.y, 0.0, 0.9, w.y);
      vec2 lf = vec2(f.x, f.y / 0.9);
      c = mix(c, vec3(0.08, 0.085, 0.09) * dark(0.6), 1.0 - band(f.y, 0.0, 0.9, w.y));
      if (glassFront > 0.0) c = mix(c, pane(base, lf, w, id * 0.5 + 0.5, tint, shopLamp, 0.6 + 0.4 * uNight, sky, glancing), glassFront);
      c = mix(c, vec3(0.06) * dark(0.6), isDoor * band(f.x, 0.3, 0.7, w.x) * band(f.y, 0.0, 0.7, w.y) * (1.0 - band(f.x, 0.34, 0.66, w.x) * band(f.y, 0.04, 0.66, w.y)));
    } else if (type == PANEL && !podium) {
      float small = band(f.x, 0.3, 0.7, w.x) * band(f.y, 0.4, 0.78, w.y) * (1.0 - isDoor);
      if (small > 0.0) c = mix(c, pane(base, (f - vec2(0.3, 0.4)) / vec2(0.4, 0.38), w / vec2(0.4, 0.38), id, tint, lamp, lit, sky, glancing), small);
    } else {
      float shop = band(f.x, 0.07, 0.93, w.x) * band(f.y, 0.06, 0.6, w.y) * (1.0 - isDoor);
      vec2 sf = (f - vec2(0.07, 0.06)) / vec2(0.86, 0.54);
      c = mix(c, vec3(0.1, 0.1, 0.105) * dark(0.6), band(f.x, 0.05, 0.95, w.x) * band(f.y, 0.04, 0.62, w.y) * (1.0 - isDoor));
      if (shop > 0.0) c = mix(c, pane(base, sf, w / vec2(0.86, 0.54), id * 0.3 + 0.7, tint, shopLamp, mix(0.5, 1.0, uNight), sky, glancing), shop);
      // The awning over it, and the fascia board over that.
      float awn = band(f.y, 0.62, 0.8, w.y) * band(f.x, 0.04, 0.96, w.x) * (1.0 - isDoor) * step(0.35, fract(variant * 3.3 + bay.x * 0.37));
      // Where details.js has built the awning, only its shadow is left to paint.
      vec3 canvas = vDetail > 0.5 ? c * 0.55 : awning(base, u, (f.y - 0.62) / 0.18, step(0.5, fract(variant * 5.1)));
      c = mix(c, canvas, awn * (1.0 - far));
      c = mix(c, wall * 0.45, band(f.y, 0.82, 0.95, w.y) * band(f.x, 0.02, 0.98, w.x) * (1.0 - isDoor) * (1.0 - far));
    }
    // The entrance: a door in a stone surround, lit from inside at night.
    float surround = isDoor * band(f.x, 0.24, 0.76, w.x) * band(f.y, 0.0, 0.8, w.y);
    float door = isDoor * band(f.x, 0.3, 0.7, w.x) * band(f.y, 0.0, 0.74, w.y);
    c = mix(c, mix(wall, vec3(0.62, 0.61, 0.58) * dark(0.45), 0.6), surround);
    vec3 leaf = mix(vec3(0.16, 0.12, 0.09) * dark(0.5), vec3(1.0, 0.8, 0.55) * 0.7, uNight * 0.7);
    leaf *= 1.0 - 0.35 * band(f.x, 0.49, 0.51, w.x);            // two leaves
    leaf = mix(leaf, sky * 0.8, band(f.y, 0.4, 0.68, w.y) * band(fract(f.x * 2.5), 0.15, 0.85, w.x * 2.5) * 0.7); // their glazing
    c = mix(c, leaf, door);
  }
  if (type == WAREHOUSE) {
    // A strip of windows under the eaves, a sign band below it, and along the
    // ground roll-up doors with a personnel door beside one.
    float strip = band(top, 0.05, 0.12, wv);
    float sm = u / 0.09, smw = fwidth(sm) + 1e-4;
    vec2 sf = vec2(fract(sm), (0.12 - top) / 0.07);
    if (strip > 0.0) c = mix(c, mix(pane(base, sf, vec2(smw, wv / 0.07), hash12(vec2(floor(sm), 3.0) + seed), tint, lamp, lit, sky, glancing), vec3(0.1) * dark(0.6), band(sf.x, 0.0, 0.08, smw) * (1.0 - far)), strip);
    c = mix(c, wall * 0.62, band(top, 0.13, 0.17, wv));
    float doorH = min(0.27, roofAt - 0.2);
    float dx = f.x;
    float dock = step(0.5, mod(bay.x, 2.0) + step(bays, 1.5)) * band(dx, 0.14, 0.86, w.x) * band(v, 0.0, doorH, wv);
    float open = step(0.8, hash12(bay + seed * 3.3));
    float ribs = mix(0.5 + 0.5 * cos(v / 0.012 * 6.2832), 0.5, smoothstep(0.3, 0.8, wv / 0.012));
    vec3 shutter = mix(vec3(0.36, 0.37, 0.38) * (0.85 + 0.15 * ribs), vec3(0.03) + vec3(1.0, 0.75, 0.45) * 0.35 * uNight, open) * dark(0.45);
    c = mix(c, vec3(0.12, 0.12, 0.13) * dark(0.6), band(dx, 0.11, 0.89, w.x) * band(v, 0.0, doorH + 0.025, wv) * (1.0 - dock) * step(0.5, mod(bay.x, 2.0) + step(bays, 1.5)));
    c = mix(c, shutter, dock);
    c = mix(c, vec3(0.03), band(dx, 0.1, 0.14, w.x) * band(v, 0.0, 0.05, wv) + band(dx, 0.86, 0.9, w.x) * band(v, 0.0, 0.05, wv)); // bumpers
    float man = isDoor * (1.0 - step(0.5, mod(bay.x, 2.0))) * band(dx, 0.42, 0.62, w.x) * band(v, 0.0, 0.2, wv) * step(1.5, bays);
    c = mix(c, vec3(0.2, 0.21, 0.22) * dark(0.5), man);
  }
  // The crown: a cornice (toothed on brick) or a metal coping, and a stone band
  // at each setback of an art-deco tower.
  if (cornice) {
    if (type == OFFICE || type == WAREHOUSE) {
      c = mix(wall * 0.9, vec3(0.3, 0.31, 0.32) * dark(0.5), band(top, 0.0, 0.018, wv));
    } else {
      c = wall * 1.18;
      if (type == BRICK) c = mix(c, c * 0.62, band(top, 0.03, 0.045, wv) * band(fract(u / 0.03), 0.25, 0.65, fwidth(u) / 0.03 + 1e-4) * (1.0 - far));
      c = mix(c, c * 0.8, band(top, 0.0, 0.012, wv));
    }
  }
  if (type == DECO) {
    for (int k = 0; k < 2; k++) {
      float at = H * (k == 0 ? 0.72 : 0.86);
      float d = at - v;
      c = mix(c, wall * 1.25, band(d, 0.0, 0.05, wv) * step(at, roofAt + 0.01));
    }
  }
  if (type == BRICK && !podium) c = mix(c, wall * 1.2, band(v, STORY - 0.02, STORY + 0.02, wv)); // the string course
  if (podium) c = mix(c, wall * 0.9, band(v, podiumTop - 0.025, podiumTop, wv));

  // Weathering: stains run down from the windows and the cornice, more on rough walls.
  float sx = u * 30.0 + seed.x * 7.0, sfw = fwidth(sx);
  float streak = smoothstep(0.55, 0.85, vnoise(vec2(sx, v * 1.3 + seed.y)));
  streak = mix(streak, 0.12, smoothstep(0.3, 1.0, sfw));
  c *= 1.0 - 0.13 * rough * streak * (1.0 - inWin);
  // The shadow a cornice throws on the wall under it.
  if (type != OFFICE && type != WAREHOUSE) c *= 1.0 - 0.22 * band(top, 0.075, 0.13, wv);

  return facadeShade(mix(c, facadeFar(base, look, variant), far), u, v, edge, seed);
}

/**
 * A flat roof: gravel inside a parapet that throws its shadow on it, and on it what
 * its type carries:
 *
 * - warehouse: a sawtooth of north lights, glazing and ribbed metal;
 * - office: a plant penthouse and a window-cleaning track; a helipad from HELIPAD up;
 * - deco: stepped rings up to the base of a spire;
 * - brick: tar, chimneys and a timber water tank;
 * - residential: some planted as green roofs;
 * - the rest: a plant room with air-conditioning units, or rows of solar panels.
 *
 * Implements: REQ-CITY-014, REQ-CITY-032
 */
vec3 roof(vec3 base, vec3 lp, vec3 sz, float e) {
  int type = int(vBuild.x + 0.5);
  float variant = vBuild.y, H = vBuild.w;
  float w = fwidth(e) + 1e-4;
  vec2 gw = fwidth(lp.xz * 60.0);
  float gravel = mix(vnoise(lp.xz * 60.0 + vSeed), 0.5, smoothstep(0.4, 1.0, max(gw.x, gw.y)));
  vec3 c = base * 0.8 * (0.9 + 0.2 * vnoise(lp.xz * 16.0 + vSeed)) * (0.9 + 0.2 * gravel);
  vec2 q = lp.xz;
  float inside = smoothstep(0.075 - w, 0.075 + w, e);
  float small = min(sz.x, sz.z);
  if (type == WAREHOUSE) {
    float t = (variant < 0.5 ? q.x : q.y) / 0.2, tw = fwidth(t) + 1e-4;
    float s = fract(t);
    vec3 north = mix(vec3(0.07, 0.08, 0.09), vec3(1.0, 0.78, 0.5) * 0.6, uNight * step(0.4, hash12(vec2(floor(t), 1.0) + vSeed)));
    vec3 saw = mix(base * (0.6 + 0.4 * s), north, band(s, 0.0, 0.3, tw));
    saw = mix(saw, mix(base * 0.8, north, 0.3), smoothstep(0.3, 0.8, tw));
    c = mix(c, saw, inside);
  } else if (type == OFFICE) {
    c = mix(c, vec3(0.1, 0.1, 0.11), band(e, 0.1, 0.114, w)); // the cleaning cradle's track
    if (H >= HELIPAD_H && small > 0.7) {
      float r = length(q) / (small * 0.36);
      float rw = fwidth(r) + 1e-4;
      vec2 hq = q / (small * 0.36);
      float letter = (band(abs(hq.x), 0.24, 0.36, rw) * band(abs(hq.y), 0.0, 0.42, rw)
        + band(abs(hq.y), 0.0, 0.05, rw) * band(abs(hq.x), 0.0, 0.3, rw));
      vec3 pad = vec3(0.16, 0.165, 0.17) * (0.9 + 0.1 * gravel);
      pad = mix(pad, vec3(0.82, 0.82, 0.78), max(band(r, 0.78, 0.86, rw), min(letter, 1.0)) * (1.0 - smoothstep(0.1, 0.3, rw)));
      c = mix(c, pad, 1.0 - smoothstep(1.0 - rw, 1.0 + rw, r));
      c = mix(c, c * 0.7, band(r, 1.0, 1.08, rw));
    } else {
      vec2 at = (vec2(hash12(vSeed + 1.7), hash12(vSeed + 8.1)) - 0.5) * sz.xz * 0.25;
      vec2 d = abs(q - at) - sz.xz * vec2(0.22, 0.16);
      float room = 1.0 - smoothstep(-w, w, max(d.x, d.y));
      float louver = band(fract((q.x - at.x) / 0.02), 0.0, 0.5, fwidth(q.x) / 0.02 + 1e-4) * (1.0 - smoothstep(0.3, 0.7, fwidth(q.x) / 0.02));
      c = mix(c, vec3(0.38, 0.385, 0.39) * (0.9 + 0.12 * louver), room);
    }
  } else if (type == DECO) {
    float ring = floor(e / 0.07);
    c *= 1.0 + 0.1 * mod(ring, 2.0) * (1.0 - smoothstep(0.3, 0.8, fwidth(e / 0.07)));
    c = mix(c, c * 0.72, band(fract(e / 0.07), 0.0, 0.12, fwidth(e / 0.07) + 1e-4) * (1.0 - smoothstep(0.3, 0.8, fwidth(e / 0.07))));
    c = mix(c, vec3(0.18, 0.18, 0.19), 1.0 - smoothstep(0.06 - w, 0.06 + w, length(q)));
  } else if (type == BRICK) {
    c = base * 0.55 * (0.85 + 0.3 * vnoise(q * 9.0 + vSeed)) * (0.92 + 0.16 * gravel);
    for (int i = 0; i < 2; i++) {
      vec2 chimney = (vec2(hash12(vSeed + float(i) * 4.3), hash12(vSeed + float(i) * 2.9 + 3.0)) - 0.5) * sz.xz * 0.7;
      vec2 d = abs(q - chimney);
      c = mix(c, base * 0.42, 1.0 - smoothstep(0.04 - w, 0.04 + w, max(d.x, d.y * 1.6)));
    }
    if (vDetail < 0.5) {
      vec2 tank = (vec2(hash12(vSeed + 6.1), hash12(vSeed + 1.3)) - 0.5) * sz.xz * 0.4;
      float r = length(q - tank);
      c = mix(c, vec3(0.3, 0.25, 0.2) * (0.9 + 0.1 * band(fract(atan(q.y - tank.y, q.x - tank.x) * 5.0), 0.0, 0.5, 0.2)), 1.0 - smoothstep(0.09 - w, 0.09 + w, r));
      c = mix(c, c * 0.6, band(r, 0.075, 0.09, w));
    }
  } else if (type == RESIDENTIAL && variant < 0.22 && small > 0.6) {
    // A green roof: sedum in beds between gravel walks. It is a little green, never
    // enough to be taken for another building's color.
    vec2 beds = q / 0.18, bw = fwidth(beds) + 1e-4;
    float bed = band(fract(beds.x), 0.1, 0.9, bw.x) * band(fract(beds.y), 0.1, 0.9, bw.y);
    bed = mix(bed, 0.64, smoothstep(0.3, 0.7, max(bw.x, bw.y)));
    vec3 sedum = mix(base * 0.62, vec3(0.16, 0.24, 0.09), 0.4) * (0.75 + 0.5 * vnoise(q * 40.0 + vSeed));
    c = mix(c, sedum, bed * inside);
  } else {
    float solar = fract(variant * 13.7); // as details.js solarRoof has it
    if (solar > 0.6 && small > 0.6) {
      vec2 t = q / vec2(0.1, 0.16);
      vec2 tw = fwidth(t) + 1e-4;
      float panel = band(fract(t.x), 0.08, 0.92, tw.x) * band(fract(t.y), 0.1, 0.75, tw.y) * smoothstep(0.08 - w, 0.08 + w, e);
      vec3 cell = mix(vec3(0.05, 0.08, 0.16), vec3(0.12, 0.18, 0.3), band(fract(t.x * 3.0), 0.45, 0.55, tw.x * 3.0));
      c = mix(c, cell * dark(0.4), panel * (1.0 - 0.5 * smoothstep(0.3, 0.6, max(tw.x, tw.y))));
    } else if (vDetail < 0.5) {
      vec2 at = (vec2(hash12(vSeed), hash12(vSeed + 5.3)) - 0.5) * sz.xz * 0.35;
      vec2 d = abs(q - at);
      float s = small * 0.15;
      float unit = 1.0 - smoothstep(s - w, s + w, max(d.x, d.y)); // a rooftop plant room
      c = mix(c, vec3(0.3, 0.31, 0.33), unit * 0.85);
      for (int i = 0; i < 2; i++) {
        vec2 ac = (vec2(hash12(vSeed + float(i) * 7.1), hash12(vSeed + float(i) * 3.3 + 1.0)) - 0.5) * sz.xz * 0.6;
        vec2 qa = abs(q - ac);
        float box = 1.0 - smoothstep(0.045 - w, 0.045 + w, max(qa.x, qa.y));
        float fan = 1.0 - smoothstep(0.025 - w, 0.025 + w, length(q - ac));
        c = mix(c, mix(vec3(0.55, 0.56, 0.57), vec3(0.12), fan), box * (1.0 - unit));
      }
    }
  }
  // The parapet, its coping lighter, and the shadow it throws inside.
  c = mix(c, c * 0.72, band(e, 0.035, 0.065, w));
  c = mix(c, base * 1.08, 1.0 - smoothstep(0.035 - w, 0.035 + w, e));
  return c * dark(0.55);
}

// Walls of dressed stone: courses of blocks, every other course offset.
vec3 retaining(vec3 base, vec2 p, float mixBase) {
  vec2 t = p / vec2(0.22, 0.09);
  t.x += 0.5 * mod(floor(t.y), 2.0);
  vec2 w = fwidth(t) + 1e-4;
  float far = smoothstep(0.25, 0.6, max(w.x, w.y));
  float stone = band(fract(t.x), 0.04, 0.96, w.x) * band(fract(t.y), 0.08, 0.92, w.y);
  vec3 c = mix(vec3(0.3, 0.29, 0.27), base, mixBase) * (0.85 + 0.3 * mix(hash12(floor(t)), 0.5, far));
  return mix(c * 0.6, c, mix(stone, 0.85, far)) * dark(0.45);
}

// Dimmed boxes (a selection or the legend) keep their facades and roofs, only much
// fainter, so the focus stands out without the city losing its texture.
// Implements: REQ-CITY-005
vec3 cityColor(vec3 base) {
  vec3 c = cityTexture(base);
  return vFade > 0.5 ? mix(c, base, FADE) : c;
}

// Which surface a fragment is on is the same question in every style - a shore, the
// top of a terrace, a symbol plot, a roof, a wall, a facade - so the styles differ only
// in which painter answers it.
// Implements: REQ-CITY-001, REQ-MAP-050
vec3 cityTexture(vec3 base) {
  vec3 n = normalize(vObjN);
  float k = floor(vKind + 0.5);
  bool circuit = uStyle > 0.5 && uStyle < 1.5;
  bool galaxy = uStyle > 1.5;

  vec3 lp = vLP, sz = vSize;
  if (n.y > 0.5) {
    float ex = sz.x * 0.5 - abs(lp.x), ez = sz.z * 0.5 - abs(lp.z);
    float e = min(ex, ez);
    vec2 p = lp.xz + vSeed;
    if (k < 0.5) {
      if (circuit) return substrate(base, p) * tint(base, uLandRef);
      if (galaxy) return dust(base, p) * tint(base, uLandRef);
      return grass(base, p) * tint(base, uLandRef);
    }
    if (k < 1.5) return circuit ? traces(base, lp, sz) : galaxy ? conduits(base, lp, sz) : streets(base, lp, sz);
    if (k > 5.5) {
      if (circuit) return solderMask(base, p) * tint(base, uGroundRef);
      if (galaxy) return dust(base, p) * tint(base, uGroundRef);
      return paving(base, p);
    }
    return circuit ? chipTop(base, lp, sz, e) : galaxy ? crystalTop(base, lp, sz, e) : roof(base, lp, sz, e);
  }
  if (n.y < -0.5) return base;
  bool sideX = abs(n.x) > 0.5;
  float u = sideX ? lp.z * sign(n.x) : -lp.x * sign(n.z);
  float faceW = sideX ? sz.z : sz.x;
  // Keep the per-face shade the flat colors carry.
  float shade = sideX ? 0.62 : 0.78;
  vec2 wall = vec2(u, lp.y);
  vec3 c;
  if (k < 0.5) {
    if (circuit) c = boardEdge(vec3(0.3, 0.24, 0.16), wall) * shade / 0.7;
    else if (galaxy) c = crystalWall(vec3(0.42, 0.36, 0.6), wall) * shade / 0.7;
    else c = retaining(vec3(0.3, 0.24, 0.16), wall, 0.0) * shade / 0.7;
  } else if (k > 5.5) {
    c = circuit ? boardEdge(base, wall) : galaxy ? crystalWall(base, wall) : retaining(base, wall, 0.35);
  } else if (k < 1.5) {
    if (circuit) c = boardEdge(base, wall);
    else if (galaxy) c = crystalWall(base, wall);
    else c = stairs(retaining(base, wall, 0.35), u, faceW, lp.y, sz.y);
  } else {
    vec2 seed = vSeed + n.xz * 3.1;
    if (circuit) c = chipFace(base, u, faceW, lp.y, sz.y, seed);
    else if (galaxy) c = crystalFace(base, u, faceW, lp.y, sz.y, seed);
    else return facade(base, u / FACADE, faceW / FACADE, (lp.y + vBuild.z) / FACADE, vBuild.w / FACADE, (vBuild.z + sz.y) / FACADE, seed, n);
  }
  // Anything standing on something is darker where the two meet. There are no lights
  // in this scene and so no shadows either, and without this a building floats over
  // its own plot: the band is what puts it back down on it. A short wall gets a
  // shorter one, or a curb would be all shadow.
  float foot = min(0.24, sz.y * 0.35);
  return c * mix(0.7, 1.0, smoothstep(0.0, foot, lp.y + sz.y * 0.5));
}
`;

export const CITY_FRAG_BODY = `
diffuseColor.rgb = cityColor(diffuseColor.rgb);
`;
