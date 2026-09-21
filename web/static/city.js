// The look of the map, all procedural (no image assets): shader "textures" for the
// boxes, plus props, on the isometric map and in walk mode alike; walk mode adds a sky
// dome and rippling water.
//
// Three styles dress the same geometry (uStyle, set by MapScene.setStyle). A city:
// facades and roofs, a street network on every terrace, grassy shores, trees and
// lamps. A circuit board: chip packages, copper traces down every street, solder mask,
// capacitors and LEDs. A galaxy: crystal spires, glowing conduits, dust and beacons.
// The three share all their geometry and all their measurements - only the paint
// differs - so a style is a look, not a second renderer.
//
// The textures are unlit and antialiased by pixel footprint, so detail fades out
// when zoomed out rather than flickering.
//
// Streets - traces on a board, conduits in a galaxy - are the free space of a terrace
// top: everything not covered by a child
// (building, district, nested terrace, symbol plot). So they connect by
// construction - the gaps between buildings are side streets, the padding along a
// terrace's edge its ring road. The shader measures, per fragment, the distance to
// the nearest obstacles (the terrace's own edges and the footprints listed for its
// cell in a lookup texture, see setRoads): close to one is sidewalk, halfway between
// two facing ones is the centre line, and where facing obstacles end is a crossing.
//
// Colors stay data first: facades and roofs are modulated from the box's own
// color (language, size, history, dimming, hover), never replaced. Streets and lawns
// have colors of their own, tinted by how far the box's color is from its kind's
// usual one (uGroundRef, uLandRef): nesting levels, hover and flashes still show.

import * as THREE from './vendor/three.module.min.js';
import { plants } from './props.js';

/** Box kinds as the shaders see them (attribute aKind). */
export function kindCode(b) {
  switch (b.kind) {
    case 'land': return 0;
    case 'terrace': return b.node.kind === 'file' ? 6 : 1; // a file's symbol plot is paved, not a block
    case 'district': return 2;
    case 'building': return 3;
    case 'symbol': return 4;
    case 'package': return 5;
  }
  return 3;
}

const NOISE_GLSL = `
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
 * centre. What it is worth, round and falling off from there, or nothing.
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
attribute vec3 aBoxCenter; // base centre, for non-instanced boxes
attribute vec3 aBoxSize;
varying vec3 vLP;          // position relative to the box's base centre, world units
varying vec3 vObjN;
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
#else
  vSize = aBoxSize;
  vLP = transformed - aBoxCenter;
  vSeed = aBoxCenter.xz;
#endif
  vObjN = normal;
  vKind = aKind;
  vFade = aFade;
`;

/** Fragment declarations: surface functions, all in linear color. */
export const CITY_FRAG_HEAD = NOISE_GLSL + `
uniform float uBend;
uniform float uNight;
uniform float uStyle; // 0 city, 1 circuit board, 2 galaxy
varying vec3 vLP;
varying vec3 vObjN;
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

vec3 paving(vec3 base, vec2 p) {
  vec2 t = p / 0.18, w = fwidth(t) + 1e-4;
  float far = smoothstep(0.2, 0.5, max(w.x, w.y));
  float joint = (1.0 - band(fract(t.x), 0.07, 0.93, w.x) * band(fract(t.y), 0.07, 0.93, w.y)) * (1.0 - far);
  vec3 c = mix(vec3(0.46, 0.45, 0.42), base, 0.3) * (0.92 + 0.16 * mix(hash12(floor(t)), 0.5, far));
  return mix(c, c * 0.72, joint) * dark(0.42);
}

// A lawn: one green with gentle, large variation and a few drier patches, and blades
// up close. Bushes and trees are real geometry (makeProps), not painted on.
vec3 grass(vec3 base, vec2 p) {
  vec2 w = fwidth(p * 60.0);
  float far = smoothstep(0.3, 1.0, max(w.x, w.y));
  vec3 g = vec3(0.12, 0.24, 0.05) * (0.9 + 0.12 * vnoise(p * 0.6) + 0.06 * vnoise(p * 3.1));
  g = mix(g, vec3(0.2, 0.28, 0.08), 0.3 * smoothstep(0.6, 0.85, vnoise(p * 1.7 + 4.0)));
  g *= 0.85 + 0.3 * mix(vnoise(p * 60.0), 0.5, far);
  return mix(g, base, 0.08) * dark(0.35);
}

vec3 asphalt(vec2 p) {
  vec2 w = fwidth(p * 40.0);
  float far = smoothstep(0.3, 1.0, max(w.x, w.y));
  float grain = mix(0.55 * vnoise(p * 40.0) + 0.45 * vnoise(p * 9.0), 0.5, far);
  vec3 c = vec3(0.034, 0.036, 0.041) * (0.75 + 0.5 * grain);
  c *= 1.0 - 0.2 * smoothstep(0.6, 0.64, fbm(p * 0.8 + 3.7));                 // repaired patches
  float crack = 1.0 - smoothstep(0.0, 0.012, abs(fbm(p * 2.6) - 0.5));
  c *= 1.0 - 0.4 * crack * (1.0 - far) * smoothstep(0.45, 0.6, vnoise(p * 1.3)); // cracks, here and there
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
vec3 tint(vec3 base, vec3 ref) { return clamp(base / max(ref, vec3(0.02)), 0.4, 2.2); }

const float SIDEWALK = 0.065;
const float CARRIAGE = 0.42; // farther from every obstacle than this is a park, not a street

// A pocket park where the packing left a hole: a mown lawn crossed by gravel paths
// (PARK_PATHS apart; makeProps keeps its bushes and trees off them).
vec3 park(vec3 base, vec2 p) {
  vec3 c = grass(base, p);
  vec2 m = p / 0.35, mw = fwidth(m) + 1e-4;
  c *= 1.0 + 0.05 * (band(fract(m.x), 0.0, 0.5, mw.x) * 2.0 - 1.0) * (1.0 - smoothstep(0.3, 0.6, mw.x)); // mowing stripes
  vec2 t = p / 2.2, w = fwidth(t) + 1e-4;
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
  int a = 0;
  for (int j = 1; j < 12; j++) if (d[j] < d[a]) a = j;
  // The nearest obstacle facing it across the street, if the street is straight here.
  int b = -1;
  float d2 = 1e9;
  for (int j = 0; j < 12; j++) if (dot(n[j], n[a]) < -0.95 && d[j] < d2) { d2 = d[j]; b = j; }

  Road r;
  r.p = p;
  r.d1 = d[a];
  r.n1 = n[a];
  r.xRoad = abs(n[a].x) < 0.5;
  r.d2 = 1e9;
  r.ends = vec2(0.0);
  // A park between two obstacles is two streets, not one.
  if (b >= 0 && d[a] + d2 <= 2.0 * CARRIAGE) {
    r.d2 = d2;
    r.ends = vec2(max(ext[a].x, ext[b].x), min(ext[a].y, ext[b].y));
  }
  return r;
}

// The top of a terrace as a city: asphalt between the buildings, sidewalks, crossings
// and pocket parks.
vec3 streets(vec3 base, vec3 lp, vec3 sz) {
  Road r = roadField(lp, sz);
  vec2 p = r.p;
  float d1 = r.d1, d2 = r.d2;
  vec2 n1 = r.n1;
  bool facing = d2 < 1e8;

  float w = fwidth(d1) + 1e-4;
  vec3 c = asphalt(p);
  if (!facing && d1 < CARRIAGE) {
    // A street along a park: the dashed line on its middle.
    float along = abs(n1.x) < 0.5 ? p.x : p.y, wa = fwidth(along) + 1e-4;
    float mid = (SIDEWALK + CARRIAGE) * 0.5;
    float dash = band(fract(along / 0.3), 0.0, 0.5, wa / 0.3) * band(d1, mid - 0.008, mid + 0.008, w);
    c = mix(c, vec3(0.78, 0.6, 0.12), dash * (1.0 - smoothstep(0.02, 0.06, wa)));
  }
  if (facing) {
    float width = d1 + d2, s = (d2 - d1) * 0.5; // s: distance from the centre line
    bool xRoad = r.xRoad;                       // the street runs along x
    float along = xRoad ? p.x : p.y, across = xRoad ? p.y : p.x;
    vec2 e = r.ends; // where both sides face each other
    float fromEnd = min(along - e.x, e.y - along);
    float wa = fwidth(along) + 1e-4, ws = fwidth(s) + 1e-4;
    float near = 1.0 - smoothstep(0.02, 0.06, wa);
    // Wheel tracks, one pair per lane.
    c *= 1.0 - 0.14 * band(abs(s), width * 0.22 - 0.025, width * 0.22 + 0.025, ws) * step(0.3, width);
    float zebra = 0.0;
    if (e.y - e.x > 1.6 && fromEnd > 0.03 && fromEnd < 0.15 && d1 > SIDEWALK + 0.01) {
      zebra = band(fract(across / 0.05), 0.0, 0.5, fwidth(across) / 0.05) * near;
      c = mix(c, vec3(0.62, 0.62, 0.6), zebra);
    }
    if (width > 0.28 && fromEnd > 0.2) {
      // A manhole every few metres on the centre line, the dashed line between them.
      float m = length(vec2(s, (fract(along / 2.7 + 0.5) - 0.5) * 2.7));
      float lid = 1.0 - smoothstep(0.042 - w, 0.042 + w, m);
      c = mix(c, vec3(0.05, 0.05, 0.055) * (0.8 + 0.4 * band(fract(p.x * 40.0), 0.0, 0.5, 0.2)), lid * near);
      c = mix(c, vec3(0.1), band(m, 0.036, 0.042, w) * near);
      float dash = band(fract(along / 0.3), 0.0, 0.5, wa / 0.3) * (1.0 - smoothstep(0.008 - ws, 0.008 + ws, s));
      c = mix(c, vec3(0.78, 0.6, 0.12), dash * (1.0 - lid) * near);
    }
  }
  c *= dark(0.5);
  // Sidewalk along every obstacle and edge, and around parks, behind pale curb stones.
  vec3 curb = vec3(0.5, 0.5, 0.48) * dark(0.45);
  vec3 walk = paving(base, p);
  walk = mix(walk, curb, band(d1, SIDEWALK - 0.014, SIDEWALK, w));
  c = mix(walk, c, smoothstep(SIDEWALK - w, SIDEWALK + w, d1));
  float edge = CARRIAGE + SIDEWALK;
  c = mix(c, paving(base, p), band(d1, CARRIAGE, edge, w));
  c = mix(c, curb, band(d1, CARRIAGE, CARRIAGE + 0.014, w));
  c = mix(c, park(base, p), smoothstep(edge - w, edge + w, d1));
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
    float width = r.d1 + r.d2, s = (r.d2 - r.d1) * 0.5; // s: from the centre line
    float ws = fwidth(s) + 1e-4;
    // Several traces side by side, as many as the street is wide enough for.
    float lanes = clamp(floor(width / 0.1), 1.0, 5.0);
    float pitch = width / (lanes + 1.0);
    float t = abs(mod(s + width * 0.5 + pitch * 0.5, pitch) - pitch * 0.5);
    c = mix(c, copper, (1.0 - smoothstep(0.011 - ws, 0.011 + ws, t)) * step(0.06, width));
    // Vias down the centre of the street, every so often.
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
  c = mix(c, epoxy * 1.9, band(v, h * 0.5 - 0.005, h * 0.5 + 0.005, wv)); // the mould parting line
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
// where rampsFor puts a ramp): the walker steps up anyway, stairs and ramps show
// where the levels connect.
vec3 stairs(vec3 c, float u, float faceW, float v, float h) {
  u -= faceW * 0.5 - 0.34;
  if (faceW < 1.5 || abs(u) > 0.16) return c;
  float wv = fwidth(v) + 1e-4, wu = fwidth(u) + 1e-4;
  float f = fract(v / (h / 4.0));
  vec3 tread = mix(vec3(0.42, 0.41, 0.39), vec3(0.58, 0.57, 0.55), smoothstep(0.7, 0.8, f)) * dark(0.45);
  tread *= 1.0 - 0.3 * band(f, 0.0, 0.08, wv * 4.0 / h);
  return mix(tread, vec3(0.2) * dark(0.5), band(abs(u), 0.13, 0.16, wu)); // stringers
}

// A flat roof: gravel, a parapet, and per building either a plant room with a couple
// of air-conditioning units or rows of solar panels.
vec3 roof(vec3 base, vec3 lp, vec3 sz, float e) {
  float w = fwidth(e) + 1e-4;
  vec2 gw = fwidth(lp.xz * 60.0);
  float gravel = mix(vnoise(lp.xz * 60.0 + vSeed), 0.5, smoothstep(0.4, 1.0, max(gw.x, gw.y)));
  vec3 c = base * 0.8 * (0.9 + 0.2 * vnoise(lp.xz * 16.0 + vSeed)) * (0.9 + 0.2 * gravel);
  c = mix(c, base * 1.08, 1.0 - smoothstep(0.035 - w, 0.035 + w, e)); // parapet
  float style = hash12(vSeed * 0.37 + 11.0);
  if (style > 0.6 && min(sz.x, sz.z) > 0.6) {
    // Solar panels in rows, inside the parapet.
    vec2 t = lp.xz / vec2(0.1, 0.16);
    vec2 tw = fwidth(t) + 1e-4;
    float panel = band(fract(t.x), 0.08, 0.92, tw.x) * band(fract(t.y), 0.1, 0.75, tw.y) * smoothstep(0.08 - w, 0.08 + w, e);
    vec3 cell = mix(vec3(0.05, 0.08, 0.16), vec3(0.12, 0.18, 0.3), band(fract(t.x * 3.0), 0.45, 0.55, tw.x * 3.0));
    c = mix(c, cell * dark(0.4), panel * (1.0 - 0.5 * smoothstep(0.3, 0.6, max(tw.x, tw.y))));
    return c * dark(0.55);
  }
  vec2 at = (vec2(hash12(vSeed), hash12(vSeed + 5.3)) - 0.5) * sz.xz * 0.35;
  vec2 q = abs(lp.xz - at);
  float s = min(sz.x, sz.z) * 0.15;
  float unit = 1.0 - smoothstep(s - w, s + w, max(q.x, q.y)); // a rooftop plant room
  c = mix(c, vec3(0.3, 0.31, 0.33), unit * 0.85);
  for (int i = 0; i < 2; i++) {
    vec2 ac = (vec2(hash12(vSeed + float(i) * 7.1), hash12(vSeed + float(i) * 3.3 + 1.0)) - 0.5) * sz.xz * 0.6;
    vec2 qa = abs(lp.xz - ac);
    float box = 1.0 - smoothstep(0.045 - w, 0.045 + w, max(qa.x, qa.y));
    float fan = 1.0 - smoothstep(0.025 - w, 0.025 + w, length(lp.xz - ac));
    c = mix(c, mix(vec3(0.55, 0.56, 0.57), vec3(0.12), fan), box * (1.0 - unit));
  }
  return c * dark(0.55);
}

// A facade: whole window bays across the face, 0.3-unit storeys, a shopfront with one
// door on the ground floor and a cornice on top. Each building picks a style: brick
// with framed windows, concrete panels, or (tall ones) a glass curtain wall. Lit
// windows glow, more of them at night.
vec3 facade(vec3 base, float u, float faceW, float v, float h, vec2 seed) {
  float bays = max(1.0, floor(faceW / 0.24));
  vec2 cell = vec2((u + faceW * 0.5) / (faceW / bays), v / 0.3);
  vec2 f = fract(cell), w = fwidth(cell) + 1e-4;
  float far = smoothstep(0.3, 0.7, max(w.x, w.y));
  float id = hash12(floor(cell) + seed * 1.37);
  float ground = 1.0 - step(1.0, cell.y);
  float top = step(h - 0.07, v);
  float style = hash12(vSeed * 0.37 + 11.0);
  bool tower = h > 2.4 && style > 0.45;
  bool brick = !tower && style < 0.5;

  vec3 wall = base * (0.92 + 0.12 * vnoise(vec2(u, v) * 26.0));
  if (brick) {
    vec2 bt = vec2(u, v) / vec2(0.06, 0.025);
    bt.x += 0.5 * mod(floor(bt.y), 2.0);
    vec2 bw = fwidth(bt) + 1e-4;
    float bfar = smoothstep(0.3, 0.6, max(bw.x, bw.y));
    float mortar = 1.0 - band(fract(bt.x), 0.06, 0.94, bw.x) * band(fract(bt.y), 0.12, 0.88, bw.y);
    wall *= 0.9 + 0.2 * mix(hash12(floor(bt) + seed), 0.5, bfar);
    wall = mix(wall, wall * 1.3 + 0.015, mortar * (1.0 - bfar) * 0.6);
  } else if (!tower) {
    wall *= 1.0 - 0.12 * (band(f.x, 0.0, 0.025, w.x) + band(f.x, 0.975, 1.0, w.x)) * (1.0 - far); // panel joints
  }
  wall *= dark(0.5);
  wall = mix(wall, wall * 1.2, top);
  wall *= 1.0 - 0.18 * band(f.y, 0.94, 1.0, w.y) * (1.0 - far); // floor lines

  float upper = (1.0 - ground) * (1.0 - top);
  float win = tower
    ? band(f.x, 0.04, 0.96, w.x) * band(f.y, 0.14, 0.94, w.y) * upper
    : band(f.x, 0.2, 0.8, w.x) * band(f.y, 0.3, 0.82, w.y) * upper;
  float frame = tower ? 0.0 : band(f.x, 0.16, 0.84, w.x) * band(f.y, 0.26, 0.86, w.y) * upper * (1.0 - win);
  float sill = tower ? 0.0 : band(f.x, 0.14, 0.86, w.x) * band(f.y, 0.2, 0.26, w.y) * upper;
  float doorBay = floor(hash12(seed + 2.0) * bays);
  float isDoor = 1.0 - step(0.5, abs(floor(cell.x) - doorBay));
  float door = band(f.x, 0.28, 0.72, w.x) * band(f.y, 0.0, 0.72, w.y) * ground * isDoor;
  float shop = band(f.x, 0.06, 0.94, w.x) * band(f.y, 0.08, 0.72, w.y) * ground * (1.0 - top) * (1.0 - isDoor);

  float litShare = mix(0.1, 0.55, uNight);
  float lit = step(1.0 - litShare, id);
  vec3 glass = mix(vec3(0.16, 0.22, 0.3), vec3(0.01, 0.014, 0.02), uNight) * (0.7 + 0.6 * hash12(floor(cell) + seed));
  if (tower) glass = mix(glass, mix(vec3(0.3, 0.42, 0.55), vec3(0.02, 0.03, 0.05), uNight), clamp(v / h, 0.0, 1.0) * 0.6); // the sky, reflected
  vec3 warm = vec3(1.0, 0.7, 0.32) * (0.55 + 0.45 * id);
  vec3 c = mix(wall, wall * 0.55, frame);
  c = mix(c, wall * 1.35 + 0.02 * dark(0.5), sill);
  c = mix(c, mix(glass, warm, lit), win);
  c = mix(c, mix(glass * 1.4, warm, uNight * 0.8), shop);
  c = mix(c, mix(vec3(0.16, 0.1, 0.06) * dark(0.5), warm * 0.8, uNight * 0.6), door);
  vec3 avg = mix(wall, mix(glass, warm, litShare), mix(tower ? 0.6 : 0.3, 0.45, uNight));
  return mix(c, avg, far);
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
vec3 cityColor(vec3 base) {
  vec3 c = cityTexture(base);
  return vFade > 0.5 ? mix(c, base, FADE) : c;
}

// Which surface a fragment is on is the same question in every style - a shore, the
// top of a terrace, a symbol plot, a roof, a wall, a facade - so the styles differ only
// in which painter answers it.
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
    else c = facade(base, u, faceW, lp.y, sz.y, seed);
  }
  // Anything standing on something is darker where the two meet. There are no lights
  // in this scene and so no shadows either, and without this a building floats over
  // its own plot: the band is what puts it back down on it. A short wall gets a
  // shorter one, or a kerb would be all shadow.
  float foot = min(0.24, sz.y * 0.35);
  return c * mix(0.7, 1.0, smoothstep(0.0, foot, lp.y + sz.y * 0.5));
}
`;

export const CITY_FRAG_BODY = `
diffuseColor.rgb = cityColor(diffuseColor.rgb);
`;

// ------------------------------------------------------------------ street lookup

// Street shading needs, per fragment, the obstacles near it. A grid over the map
// lists for each cell the ROAD_SLOTS footprints nearest to it (within ROAD_REACH,
// wider than the widest street's half), so the shader never loops over all boxes.
const ROAD_CELL = 0.5, ROAD_REACH = 0.9, ROAD_SLOTS = 8, RECT_TEX_W = 1024;
const MAX_ROAD_CELLS = 1 << 20, MAX_ROAD_TEX = 4096; // memory and texture-size bounds

/** Uniforms the street shader reads; setRoads fills them. */
export function roadUniforms() {
  return {
    uRoadIdx: { value: null }, uRoadRects: { value: null },
    uRoadGrid: { value: new THREE.Vector4() }, uRoadOn: { value: 0 },
    uGroundRef: { value: new THREE.Color(1, 1, 1) }, uLandRef: { value: new THREE.Color(1, 1, 1) },
  };
}

/**
 * Builds the street lookup for a layout: every box but land is an obstacle for the
 * terrace it stands on. A fragment ignores footprints it lies inside (its own
 * terrace and those below it), so one grid serves all terrace levels.
 */
export function setRoads(u, boxes) {
  for (const k of ['uRoadIdx', 'uRoadRects']) {
    u[k].value?.dispose();
    u[k].value = null;
  }
  u.uRoadOn.value = 0;
  const obs = boxes.filter(b => b.kind !== 'land');
  if (!obs.length) return;
  let minX = Infinity, maxX = -Infinity, minZ = Infinity, maxZ = -Infinity;
  for (const b of obs) {
    minX = Math.min(minX, b.x - b.w / 2); maxX = Math.max(maxX, b.x + b.w / 2);
    minZ = Math.min(minZ, b.z - b.d / 2); maxZ = Math.max(maxZ, b.z + b.d / 2);
  }
  // Huge maps get coarser cells; more obstacles then share a cell's slots.
  const per = Math.min(1 / ROAD_CELL, Math.sqrt(MAX_ROAD_CELLS / Math.max(1, (maxX - minX) * (maxZ - minZ))),
    MAX_ROAD_TEX / 2 / Math.max(1, maxX - minX), MAX_ROAD_TEX / Math.max(1, maxZ - minZ));
  const W = Math.ceil((maxX - minX) * per) + 1, H = Math.ceil((maxZ - minZ) * per) + 1;
  const dist = new Float32Array(W * H * ROAD_SLOTS).fill(Infinity);
  const ids = new Float32Array(W * H * ROAD_SLOTS).fill(-1); // doubles as the texture: 2 RGBA texels per cell
  const rects = new Float32Array(Math.ceil(obs.length / RECT_TEX_W) * RECT_TEX_W * 4);
  const cellOf = (v, min, n) => Math.min(n - 1, Math.max(0, Math.floor((v - min) * per)));

  obs.forEach((b, id) => {
    const x0 = b.x - b.w / 2, x1 = b.x + b.w / 2, z0 = b.z - b.d / 2, z1 = b.z + b.d / 2;
    rects.set([x0, z0, x1, z1], id * 4);
    const i0 = cellOf(x0 - ROAD_REACH, minX, W), i1 = cellOf(x1 + ROAD_REACH, minX, W);
    const j0 = cellOf(z0 - ROAD_REACH, minZ, H), j1 = cellOf(z1 + ROAD_REACH, minZ, H);
    const inL = cellOf(x0, minX, W) + 1, inR = cellOf(x1, minX, W) - 1; // cells wholly inside, by column
    for (let j = j0; j <= j1; j++) {
      const cz0 = minZ + j / per, cz1 = cz0 + 1 / per;
      const rowInside = cz0 >= z0 && cz1 <= z1;
      for (let i = i0; i <= i1; i++) {
        // A cell inside the footprint never shows a surface the footprint borders.
        if (rowInside && i >= inL && i <= inR) { i = inR; continue; }
        const cx0 = minX + i / per, cx1 = cx0 + 1 / per;
        const d = Math.hypot(Math.max(0, x0 - cx1, cx0 - x1), Math.max(0, z0 - cz1, cz0 - z1));
        if (d > ROAD_REACH) continue;
        const base = (j * W + i) * ROAD_SLOTS;
        let worst = base;
        for (let k = base + 1; k < base + ROAD_SLOTS; k++) if (dist[k] > dist[worst]) worst = k;
        if (d < dist[worst]) {
          dist[worst] = d;
          ids[worst] = id;
        }
      }
    }
  });

  const tex = (data, w, h) => {
    const t = new THREE.DataTexture(data, w, h, THREE.RGBAFormat, THREE.FloatType);
    t.minFilter = t.magFilter = THREE.NearestFilter;
    t.generateMipmaps = false;
    t.needsUpdate = true;
    return t;
  };
  u.uRoadIdx.value = tex(ids, W * 2, H);
  u.uRoadRects.value = tex(rects, RECT_TEX_W, rects.length / 4 / RECT_TEX_W);
  u.uRoadGrid.value.set(minX, minZ, per, RECT_TEX_W);
  u.uRoadOn.value = 1;
}

// ------------------------------------------------------------------ sky and water

/** A sky dome drawn behind everything; it follows the camera. */
export function makeSky(uniforms) {
  const mat = new THREE.ShaderMaterial({
    uniforms: { ...uniforms, uTop: { value: new THREE.Color() }, uHorizon: { value: new THREE.Color() } },
    side: THREE.BackSide,
    depthWrite: false,
    vertexShader: `
      varying vec3 vDir;
      void main() {
        vDir = position;
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        gl_Position.z = gl_Position.w; // on the far plane
      }`,
    fragmentShader: NOISE_GLSL + `
      uniform vec3 uTop, uHorizon;
      uniform float uNight, uTime, uStyle;
      varying vec3 vDir;

      // The plane of the galaxy: a great circle the band of light lies along.
      const vec3 GALACTIC = vec3(0.4, 0.31, -0.86);

      /**
       * One layer of stars. The directions are cut into cells and a cell either holds
       * a star at its centre or holds nothing; what was drawn before was the cell
       * itself, which is why the sky was a grid of white squares. Only a few cells in
       * a hundred are lit, so their lattice never reads, and the distance is measured
       * across the line of sight rather than through it, which keeps a star a round
       * dot however the cube grid happens to cross the sphere.
       *
       * color runs from cool blue through white to amber, because a sky of identical
       * white dots reads as dirt on the screen.
       *
       * A star smaller than a pixel would flicker as the view turned, so one that far
       * away is widened to a pixel and dimmed by as much as it was widened: it fades
       * out instead of sparkling. fwidth is taken before anything branches, because a
       * derivative asked for inside a branch is not defined.
       */
      vec3 stars(vec3 d, float scale, float rarity, float size, float glow) {
        float px = length(fwidth(d)) * scale * 0.8;
        vec3 q = d * scale;
        vec3 cell = floor(q);
        float pick = hash13(cell + 5.0);
        if (pick < rarity) return vec3(0.0);
        vec3 off = fract(q) - 0.5;
        off -= d * dot(off, d); // only the part across the view
        float r = length(off);
        // Never wider than a third of a cell: past that the cut at the cell's edge is
        // what shows, which is the grid of squares this was drawn to get rid of. What
        // the widening costs is worked out first, so a capped star still goes out.
        float fade = (size * size) / max(size * size, px * px);
        float w = min(max(size, px), 0.3);
        float core = exp(-(r * r) / (w * w)) * fade;
        float mag = 0.2 + 0.8 * hash13(cell + 3.0);
        // Cooler or warmer than white, either way; a few are decidedly amber.
        float temp = hash13(cell + 29.0);
        vec3 tint = mix(vec3(0.62, 0.74, 1.0), vec3(1.0, 0.84, 0.6), smoothstep(0.35, 0.9, temp));
        tint = mix(vec3(0.95, 0.97, 1.0), tint, 0.8);
        // Slow, per-star, and small: a night sky is not a string of fairy lights.
        float twinkle = 0.9 + 0.1 * sin(uTime * (0.5 + mag) + pick * 39.0);
        return tint * mag * twinkle * (core + glow * exp(-r * r * 70.0) * mag);
      }

      /**
       * Noise over a direction rather than over a plane. Projecting the sky onto one
       * plane pinches everything to a smear at the pole, which is what the nebulae
       * used to do directly overhead; this takes all three projections and weighs
       * each by how square-on the direction is to it, so the structure is the same
       * size wherever it is looked at. Three octaves, not five: at the size of a
       * nebula the last two are below a pixel.
       */
      float skyFbm(vec3 d, float s, float seed) {
        vec3 w = abs(d);
        w /= w.x + w.y + w.z;
        float v = 0.0;
        vec2 a = d.zy * s + seed, b = d.xz * s + seed + 19.0, c = d.xy * s + seed + 43.0;
        float amp = 0.5;
        for (int i = 0; i < 3; i++) {
          v += amp * (w.x * vnoise(a) + w.y * vnoise(b) + w.z * vnoise(c));
          a = a * 2.03 + 17.1; b = b * 2.03 + 17.1; c = c * 2.03 + 17.1;
          amp *= 0.5;
        }
        return v / 0.875; // the three octaves sum to less than one
      }

      // Deep space: the band of the galaxy with the dark lanes across it, two nebulae
      // drifting behind that, and three layers of stars - dense and faint, sparse and
      // bright, and a few near enough to have a halo around them.
      vec3 space(vec3 d) {
        vec3 col = mix(vec3(0.02, 0.017, 0.05), vec3(0.006, 0.006, 0.022), pow(max(d.y, 0.0), 0.6));

        // The band. Away from its plane there is almost nothing; along it, the light
        // of everything too far off to be a star, broken by the dust that crosses it.
        float band = dot(d, normalize(GALACTIC));
        float along = exp(-band * band * 14.0);
        col += vec3(0.16, 0.15, 0.25) * along * (0.35 + 1.05 * smoothstep(0.3, 0.88, skyFbm(d, 2.6, 0.0)));
        col *= 1.0 - 0.6 * along * smoothstep(0.44, 0.9, skyFbm(d, 5.5, 31.0));

        // Two nebulae, drifting. They are patches of color, not a wash: spread over
        // the whole dome they only lift the black to a flat mauve and take the depth
        // out of everything in front of them.
        //
        // One field of noise serves both, the second reading it upside down: where
        // the purple is thickest the teal is absent, which is what two clouds at
        // different distances look like anyway, and the sky is drawn over every
        // pixel on the screen every frame - it is not the place to ask for noise
        // twice to say the same thing.
        float neb = skyFbm(d, 2.4, uTime * 0.01);
        col += vec3(0.24, 0.07, 0.36) * smoothstep(0.55, 0.88, neb) * (0.5 + 0.5 * neb);
        col += vec3(0.05, 0.19, 0.25) * smoothstep(0.58, 0.82, 1.0 - neb);

        // The scales are chosen so a cell is several pixels across at a normal field
        // of view: finer than that and every star is smaller than a pixel, which is
        // how the dense layer ended up a dim grey haze with wedges cut out of it.
        col += stars(d, 85.0, 0.87, 0.17, 0.0) * (0.5 + 1.0 * along);
        col += stars(d, 38.0, 0.955, 0.13, 0.0) * 1.5;
        col += stars(d, 15.0, 0.982, 0.075, 0.25) * 2.2;
        return col;
      }

      void main() {
        vec3 d = normalize(vDir);
        float up = max(d.y, 0.0);
        if (uStyle > 1.5) {
          gl_FragColor = vec4(space(d), 1.0);
          #include <colorspace_fragment>
          return;
        }
        if (uStyle > 0.5) {
          // A workbench: even, dim light from everywhere, nothing overhead to look at.
          vec3 lab = mix(uHorizon, uTop, pow(up, 0.8)) * (0.95 + 0.05 * vnoise(d.xz * 5.0));
          gl_FragColor = vec4(lab, 1.0);
          #include <colorspace_fragment>
          return;
        }
        vec3 col = mix(uHorizon, uTop, pow(up, 0.5));
        if (d.y > 0.0) {
          vec2 p = d.xz / (d.y + 0.12) * 1.4 + vec2(uTime * 0.012, uTime * 0.004);
          float c = smoothstep(0.52, 0.82, fbm(p)) * smoothstep(0.0, 0.2, d.y);
          vec3 cloud = mix(vec3(1.0), vec3(0.05, 0.06, 0.08), uNight) * (0.85 + 0.15 * fbm(p * 3.0));
          col = mix(col, cloud, c * mix(0.9, 0.6, uNight));
          float star = step(0.998, hash13(floor(d * 420.0))) * uNight * (1.0 - c) * smoothstep(0.04, 0.3, d.y);
          col += star * (0.4 + 0.6 * hash13(floor(d * 420.0) + 7.0));
        }
        vec3 sd = normalize(vec3(-0.45, mix(0.42, 0.55, uNight), -0.78));
        float a = dot(d, sd);
        vec3 light = mix(vec3(1.0, 0.92, 0.75), vec3(0.75, 0.78, 0.85), uNight);
        col += light * (smoothstep(0.9986, 0.9991, a) + pow(max(a, 0.0), 48.0) * mix(0.4, 0.06, uNight));
        gl_FragColor = vec4(col, 1.0);
        #include <colorspace_fragment>
      }`,
  });
  const sky = new THREE.Mesh(new THREE.SphereGeometry(10, 32, 16), mat);
  sky.frustumCulled = false;
  sky.renderOrder = -10;
  return sky;
}

/**
 * Water with slow ripples: the planet's surface in walk mode, the sea around the
 * isometric map. Ripples fade to their mean where a pixel covers several of them.
 */
export function waterMaterial(uniforms) {
  const mat = new THREE.MeshBasicMaterial();
  mat.onBeforeCompile = shader => {
    Object.assign(shader.uniforms, uniforms);
    shader.vertexShader = 'varying vec3 vW;\n' + shader.vertexShader.replace('#include <project_vertex>',
      '#include <project_vertex>\nvW = (modelMatrix * vec4(position, 1.0)).xyz;');
    shader.fragmentShader = NOISE_GLSL + `
      uniform float uTime, uNight, uStyle;
      varying vec3 vW;

      /**
       * How bright a star is at this moment, between a half and one. Each has its own
       * phase, taken from the cell it sits in, and its own rate from the layer it
       * belongs to - a field that all pulsed together would read as the screen
       * flickering rather than as stars.
       */
      float breathe(vec2 q, float rate) {
        return 0.75 + 0.25 * sin(uTime * rate + hash12(floor(q)) * 43.0);
      }
    ` + shader.fragmentShader.replace('#include <color_fragment>', `
      #include <color_fragment>
      if (uStyle > 1.5) {
        // Between the platforms there is no sea, only more of the same sky seen the
        // other way. Nothing here is still: the platforms are adrift, and what is
        // behind them moves.
        //
        // Depth is the whole of it. Three layers travel at three speeds and in three
        // directions - the far cloud barely at all, the veil in front of it faster
        // and across it, the stars faster still - which is why it reads as something
        // deep rather than as one sheet of noise sliding sideways. The speeds are in
        // the noise's own coordinates, so the far cloud crosses about a fifth of a
        // map unit a second: a drift, not an animation.
        vec2 far = vW.xz * 0.055 + vec2(uTime * 0.007, uTime * -0.004);
        vec2 veil = vW.zx * 0.1 + vec2(uTime * -0.021, uTime * 0.014);
        float deep = fbm(far);
        diffuseColor.rgb *= 0.42 + 0.42 * vnoise(vW.xz * 0.12 + vec2(uTime * 0.01, 0.0));
        diffuseColor.rgb += vec3(0.13, 0.04, 0.22) * smoothstep(0.52, 0.9, deep) * (0.4 + 0.6 * deep);
        diffuseColor.rgb += vec3(0.02, 0.08, 0.15) * smoothstep(0.62, 0.95, fbm(veil + 23.0));
        // Through the dust, a slow curtain of light: the one thing here bright enough
        // to be seen moving, so it is what makes the rest of it read as moving too.
        // Broad and faint - a sheet drawn across the whole of it, not a cloud, which
        // is the difference between a nebula and mould on the screen.
        float curtain = fbm(vW.xz * 0.045 + vec2(uTime * -0.012, uTime * 0.007) + 61.0);
        diffuseColor.rgb += vec3(0.05, 0.09, 0.19) * smoothstep(0.58, 0.96, curtain);

        // The stars, drifting with the layer they belong to and each breathing at its
        // own rate. One field at one brightness is a speckle; these have a distance.
        vec2 s0 = vW.xz * 34.0 + vec2(uTime * 0.09, uTime * -0.05);
        vec2 s1 = vW.xz * 11.0 + vec2(uTime * -0.14, uTime * 0.08);
        vec2 s2 = vW.zx * 4.5 + vec2(uTime * 0.11, uTime * -0.16) + 7.0;
        diffuseColor.rgb += vec3(0.72, 0.78, 1.0) * starDot(s0, 0.955, 0.16) * 0.8 * breathe(s0, 1.0);
        diffuseColor.rgb += vec3(0.86, 0.9, 1.0) * starDot(s1, 0.985, 0.13) * 1.6 * breathe(s1, 0.7);
        diffuseColor.rgb += vec3(1.0, 0.86, 0.66) * starDot(s2, 0.992, 0.1) * 2.0 * breathe(s2, 0.45);
      } else if (uStyle > 0.5) {
        // Around the board there is only the bench it lies on: an anodized plate,
        // brushed along one axis, with the cutting grid scribed across it. The brush
        // is what the flat grey was missing - a bench with nothing on it but a grid
        // reads as graph paper.
        vec2 t = vW.xz / 0.9;
        vec2 tw = fwidth(t) + 1e-4;
        float grid = max(1.0 - smoothstep(0.012 - tw.x, 0.012 + tw.x, abs(fract(t.x) - 0.5)),
                         1.0 - smoothstep(0.012 - tw.y, 0.012 + tw.y, abs(fract(t.y) - 0.5)));
        float brush = vnoise(vec2(vW.x * 160.0, vW.z * 2.5)) + 0.5 * vnoise(vec2(vW.x * 420.0, vW.z * 1.3));
        float bw = fwidth(vW.x * 160.0);
        brush = mix(brush, 0.75, smoothstep(0.35, 1.2, bw)); // to its mean when minified
        diffuseColor.rgb *= 0.9 + 0.13 * (brush - 0.75) + 0.12 * vnoise(vW.xz * 8.0);
        // Anodizing is never quite even: broad, very slight clouding over the brush.
        diffuseColor.rgb *= 0.96 + 0.08 * fbm(vW.xz * 0.35);
        diffuseColor.rgb *= 1.0 + 0.5 * grid * (1.0 - smoothstep(0.02, 0.08, max(tw.x, tw.y)));
      } else {
        // Open water: a long swell with wind chop riding on it, the crests catching
        // the sky and a little of it showing through where the water is thin. Two
        // octaves traveling at different speeds and angles is what stops it reading
        // as one sheet of noise sliding sideways.
        vec2 w = vW.xz;
        float swell = vnoise(w * 0.7 + vec2(uTime * 0.09, uTime * 0.05))
          + 0.6 * vnoise(w * 1.6 + vec2(uTime * 0.31, -uTime * 0.14));
        float chop = vnoise(w * 4.5 - vec2(uTime * 0.28, uTime * 0.36))
          + 0.5 * vnoise(w * 9.5 + vec2(-uTime * 0.5, uTime * 0.22));
        vec2 fw = fwidth(w * 9.5);
        // A pixel covering many ripples sees their mean; the swell survives longer
        // than the chop does, which is what distance does to water.
        chop = mix(chop / 1.5, 0.5, smoothstep(0.25, 1.0, max(fw.x, fw.y)));
        swell = mix(swell / 1.6, 0.5, smoothstep(0.35, 1.4, max(fw.x, fw.y) * 0.2));
        float r = mix(swell, chop, 0.45);
        diffuseColor.rgb *= 0.76 + 0.46 * r;
        // The crests break rather than glow: a fine sparkle that lives only on the
        // top of a wave. A broad white wash over the peaks, which is what was here,
        // reads as fog lying on the water.
        float sparkle = vnoise(w * 24.0 - vec2(uTime * 0.7, uTime * 0.45));
        sparkle = mix(sparkle, 0.5, smoothstep(0.3, 1.0, fwidth(w.x * 24.0)));
        diffuseColor.rgb += smoothstep(0.64, 0.86, r) * smoothstep(0.54, 0.86, sparkle)
          * mix(0.34, 0.1, uNight);
      }`);
  };
  mat.customProgramCacheKey = () => 'water';
  return mat;
}

// ------------------------------------------------------------------ props

// ------------------------------------------------------------------ blocks and ramps

/** Terraces (city blocks, not a file's symbol plot) with the boxes standing on them. */
function blocks(boxes) {
  const byNode = new Map();
  for (const b of boxes) if (b.kind === 'terrace' && b.node.kind !== 'file') byNode.set(b.node.id, b);
  const kids = new Map([...byNode.values()].map(t => [t, []]));
  for (const b of boxes) {
    if (b.kind === 'land') continue;
    const t = b.node.parentNode && byNode.get(b.node.parentNode.id);
    if (t) kids.get(t).push(b);
  }
  return kids;
}

// A ramp runs along one side of a nested terrace, in the street beside it, from the
// street's level at one corner up to the terrace's top: RAMP_W wide, at most RAMP_MAX
// long, the last RAMP_LANDING of it level. From the landing a driveway, DRIVE deep,
// crosses the terrace's sidewalk into its ring road; at the foot an apron, APRON
// long, replaces the street's curb. The far end leaves room for the stairs (the
// stairs shader).
const RAMP_W = 0.2, RAMP_MAX = 2.4, RAMP_MIN_SIDE = 1.7, RAMP_START = 0.1, RAMP_CLEAR = 0.1;
const RAMP_LANDING = 0.35, DRIVE = 0.14, APRON = RAMP_START + 0.07;

const rampCache = new WeakMap();

/**
 * The ramps of a layout: for every nested terrace, one on the side with the most room
 * beside it. Each ramp: {origin: [x, z] (low end, at the wall), u: direction up the
 * ramp, n: away from the wall, len, rise (the sloped part's length), y0, y1 (low and
 * high surface), x0, z0, x1, z1 (footprint), drive (the driveway's footprint on the
 * terrace)}. Cached per boxes array: scene and walker share it.
 */
export function rampsFor(boxes) {
  if (rampCache.has(boxes)) return rampCache.get(boxes);
  const ramps = [];
  for (const [t, kids] of blocks(boxes)) {
    const tx0 = t.x - t.w / 2, tx1 = t.x + t.w / 2, tz0 = t.z - t.d / 2, tz1 = t.z + t.d / 2;
    for (const c of kids) {
      if (c.kind !== 'terrace' || c.node.kind === 'file') continue;
      let best = null;
      for (const side of sides(c)) {
        if (side.len < RAMP_MIN_SIDE) continue;
        const len = Math.min(RAMP_MAX, side.len - 0.8);
        const a = -side.len / 2 + RAMP_START;
        const origin = [side.mid[0] + side.u[0] * a, side.mid[1] + side.u[1] * a];
        const far = [origin[0] + side.u[0] * len + side.n[0] * RAMP_W, origin[1] + side.u[1] * len + side.n[1] * RAMP_W];
        const r = {
          x0: Math.min(origin[0], far[0]), x1: Math.max(origin[0], far[0]),
          z0: Math.min(origin[1], far[1]), z1: Math.max(origin[1], far[1]),
        };
        let clear = Math.min(r.x0 - tx0, tx1 - r.x1, r.z0 - tz0, tz1 - r.z1);
        for (const k of kids) if (k !== c) clear = Math.min(clear, rectDist(r, k));
        if (clear >= RAMP_CLEAR && (!best || clear > best.clear)) {
          best = { ...r, clear, origin, u: side.u, n: side.n, len, rise: len - RAMP_LANDING, y0: t.y + t.h, y1: c.y + c.h };
          const d0 = [origin[0] + side.u[0] * best.rise, origin[1] + side.u[1] * best.rise];
          const d1 = [origin[0] + side.u[0] * len - side.n[0] * DRIVE, origin[1] + side.u[1] * len - side.n[1] * DRIVE];
          best.drive = { x0: Math.min(d0[0], d1[0]), x1: Math.max(d0[0], d1[0]), z0: Math.min(d0[1], d1[1]), z1: Math.max(d0[1], d1[1]) };
        }
      }
      if (best) ramps.push(best);
    }
  }
  rampCache.set(boxes, ramps);
  return ramps;
}

/** The ramp surface's height at (x, z), or -Infinity off the ramp. */
export function rampHeight(r, x, z) {
  if (x < r.x0 || x > r.x1 || z < r.z0 || z > r.z1) return -Infinity;
  const s = ((x - r.origin[0]) * r.u[0] + (z - r.origin[1]) * r.u[1]) / r.rise;
  return r.y0 + (r.y1 - r.y0) * Math.min(1, Math.max(0, s));
}

// A box's four sides: outward normal n, direction u of increasing position along the
// face as the shaders measure it (cityColor's u), the side's centre and length.
function sides(b) {
  const hw = b.w / 2, hd = b.d / 2;
  return [
    { n: [1, 0], u: [0, 1], mid: [b.x + hw, b.z], len: b.d },
    { n: [-1, 0], u: [0, -1], mid: [b.x - hw, b.z], len: b.d },
    { n: [0, 1], u: [-1, 0], mid: [b.x, b.z + hd], len: b.w },
    { n: [0, -1], u: [1, 0], mid: [b.x, b.z - hd], len: b.w },
  ];
}

// Distance between two footprints ({x0, z0, x1, z1} or a box), 0 when they overlap.
function rectDist(a, b) {
  const bx0 = b.x0 ?? b.x - b.w / 2, bx1 = b.x1 ?? b.x + b.w / 2, bz0 = b.z0 ?? b.z - b.d / 2, bz1 = b.z1 ?? b.z + b.d / 2;
  return Math.hypot(Math.max(0, a.x0 - bx1, bx0 - a.x1), Math.max(0, a.z0 - bz1, bz0 - a.z1));
}

// ------------------------------------------------------------------ bridges

// Islands are separated from the mainland by water, which a walker cannot cross, so
// every island is linked by a bridge: a deck DECK_W wide, arched by DECK_RISE, with
// railings and piers. The links form a spanning tree rooted at the mainland, each
// island joined to the nearest shore already reachable, so every island can be walked
// to. Shores (the land boxes) all have their tops at the same height.
const DECK_W = 0.8, DECK_RISE = 0.25, DECK_T = 0.06, RAIL_H = 0.14, PIER_EVERY = 1.8, DECK_OVERLAP = 0.25;

const bridgeCache = new WeakMap();

/**
 * The bridges of a layout: {a, b: the two shores, axis: 'x'|'z' (the span's
 * direction), across: the deck's centre on the other axis, from, to: the span's ends
 * along the axis, y: the shores' level}. Cached per boxes array, like rampsFor.
 */
export function bridgesFor(boxes) {
  if (bridgeCache.has(boxes)) return bridgeCache.get(boxes);
  const lands = boxes.filter(b => b.kind === 'land');
  const bridges = [];
  if (lands.length > 1) {
    // The mainland is the largest shore; islands join the nearest shore already linked.
    const linked = [lands.reduce((a, b) => (a.w * a.d >= b.w * b.d ? a : b))];
    const rest = lands.filter(l => l !== linked[0]);
    while (rest.length) {
      let best = null;
      for (const a of linked) {
        for (const b of rest) {
          const d = rectDist(rect(a), b);
          if (!best || d < best.d) best = { a, b, d };
        }
      }
      const span = bridgeBetween(best.a, best.b);
      if (span) bridges.push(span);
      linked.push(best.b);
      rest.splice(rest.indexOf(best.b), 1);
    }
  }
  bridgeCache.set(boxes, bridges);
  return bridges;
}

/** A bridge deck's footprint, for indexing it like a box. */
export function bridgeBounds(r) {
  const half = DECK_W / 2;
  return r.axis === 'x'
    ? { x0: r.from, x1: r.to, z0: r.across - half, z1: r.across + half }
    : { x0: r.across - half, x1: r.across + half, z0: r.from, z1: r.to };
}

/** The deck's height at (x, z), or -Infinity beside the bridge. */
export function bridgeHeight(r, x, z) {
  const along = r.axis === 'x' ? x : z, across = r.axis === 'x' ? z : x;
  if (along < r.from || along > r.to || Math.abs(across - r.across) > DECK_W / 2) return -Infinity;
  const t = (along - r.from) / Math.max(1e-6, r.to - r.from);
  return r.y + DECK_RISE * Math.sin(Math.PI * t);
}

// The span between two shores: across the smaller gap, centred on the stretch where
// they face each other (or on the nearer end when they only overlap diagonally).
function bridgeBetween(a, b) {
  const ra = rect(a), rb = rect(b);
  const gapX = Math.max(rb.x0 - ra.x1, ra.x0 - rb.x1), gapZ = Math.max(rb.z0 - ra.z1, ra.z0 - rb.z1);
  const axis = gapX > gapZ ? 'x' : 'z';
  const [g0, g1] = axis === 'x' ? [ra.x1, rb.x0] : [ra.z1, rb.z0];
  const forward = axis === 'x' ? ra.x1 <= rb.x0 : ra.z1 <= rb.z0;
  const from = forward ? g0 : (axis === 'x' ? rb.x1 : rb.z1);
  const to = forward ? g1 : (axis === 'x' ? ra.x0 : ra.z0);
  if (to - from < 0.2) return null; // touching shores need no bridge
  const [lo, hi] = axis === 'x'
    ? [Math.max(ra.z0, rb.z0), Math.min(ra.z1, rb.z1)]
    : [Math.max(ra.x0, rb.x0), Math.min(ra.x1, rb.x1)];
  const across = lo < hi
    ? (lo + hi) / 2
    : clampTo(axis === 'x' ? [ra.z0, ra.z1] : [ra.x0, ra.x1], axis === 'x' ? (rb.z0 + rb.z1) / 2 : (rb.x0 + rb.x1) / 2);
  return { a, b, axis, across, from: from - DECK_OVERLAP, to: to + DECK_OVERLAP, y: a.y + a.h };
}

const rect = b => ({ x0: b.x - b.w / 2, x1: b.x + b.w / 2, z0: b.z - b.d / 2, z1: b.z + b.d / 2 });
const clampTo = ([lo, hi], v) => Math.min(hi - DECK_W, Math.max(lo + DECK_W, v));

// Bridge geometry: the arched deck (split along its length so it bends with the
// planet), its sides and railings, and a pier every PIER_EVERY down to the water.
// aRamp, as for ramps: across and along in world units, 3 on the deck (a road with a
// centre line), 0 on everything else.
function bridgeGeometry(bridges) {
  const pos = [], ramp = [], shade = [], index = [];
  const quad = (a, b, c, d, ra, rb, rc, rd, k) => {
    const i = pos.length / 3;
    pos.push(...a, ...b, ...c, ...d);
    ramp.push(...ra, ...rb, ...rc, ...rd);
    for (let j = 0; j < 4; j++) shade.push(k, k, k);
    index.push(i, i + 1, i + 2, i, i + 2, i + 3);
  };
  const box = (cx, cz, y0, y1, w, d, k) => {
    for (const [sx, sz] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
      const hw = w / 2, hd = d / 2;
      const p = (dx, dz, y) => [cx + dx, y, cz + dz];
      const [ax, az, bx, bz] = sx ? [sx * hw, -hd, sx * hw, hd] : [-hw, sz * hd, hw, sz * hd];
      quad(p(ax, az, y0), p(bx, bz, y0), p(bx, bz, y1), p(ax, az, y1),
        [0, 0, 0], [w + d, 0, 0], [w + d, y1 - y0, 0], [0, y1 - y0, 0], k);
    }
  };
  for (const r of bridges) {
    const len = r.to - r.from;
    const at = (t, off, dy) => {
      const along = r.from + len * t, y = r.y + DECK_RISE * Math.sin(Math.PI * t) + dy;
      return r.axis === 'x' ? [along, y, r.across + off] : [r.across + off, y, along];
    };
    const n = Math.max(3, Math.ceil(len / 0.5));
    for (let i = 0; i < n; i++) {
      const t0 = i / n, t1 = (i + 1) / n, a0 = len * t0, a1 = len * t1;
      const hw = DECK_W / 2;
      quad(at(t0, -hw, 0), at(t1, -hw, 0), at(t1, hw, 0), at(t0, hw, 0),
        [0, a0, 3], [0, a1, 3], [DECK_W, a1, 3], [DECK_W, a0, 3], 1);          // deck
      for (const side of [-1, 1]) {
        quad(at(t0, side * hw, -DECK_T), at(t1, side * hw, -DECK_T), at(t1, side * hw, 0), at(t0, side * hw, 0),
          [a0, 0, 0], [a1, 0, 0], [a1, DECK_T, 0], [a0, DECK_T, 0], 0.62);     // the deck's edge
        quad(at(t0, side * hw, 0), at(t1, side * hw, 0), at(t1, side * hw, RAIL_H), at(t0, side * hw, RAIL_H),
          [a0, 0, 0], [a1, 0, 0], [a1, RAIL_H, 0], [a0, RAIL_H, 0], 0.9);      // railing
      }
    }
    for (let t = PIER_EVERY / 2; t < len; t += PIER_EVERY) {
      const [x, y, z] = at(t / len, 0, -DECK_T);
      box(x, z, r.y - 0.45, y, 0.18, 0.18, 0.7);
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
  geo.setAttribute('aRamp', new THREE.Float32BufferAttribute(ramp, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(shade, 3));
  geo.setIndex(index);
  return geo;
}

// Ramp geometry: the roadway (split along its length so it bends with the planet),
// its outer wall with a parapet, the wall and barrier at its high end, the driveway
// onto the terrace and the apron at its foot. aRamp: across and along the roadway in
// world units, and 1 on the sloped roadway (markings), 2 on plain asphalt, 0 on walls.
function rampGeometry(ramps) {
  const pos = [], ramp = [], shade = [], index = [];
  const quad = (a, b, c, d, ra, rb, rc, rd, k) => {
    const i = pos.length / 3;
    pos.push(...a, ...b, ...c, ...d);
    ramp.push(...ra, ...rb, ...rc, ...rd);
    for (let j = 0; j < 4; j++) shade.push(k, k, k);
    index.push(i, i + 1, i + 2, i, i + 2, i + 3);
  };
  const LIFT = 0.004, PARAPET = 0.035;
  for (const r of ramps) {
    const at = (s, w, y) => [r.origin[0] + r.u[0] * s + r.n[0] * w, y, r.origin[1] + r.u[1] * s + r.n[1] * w];
    const y = s => r.y0 + (r.y1 - r.y0) * Math.min(1, s / r.rise) + LIFT;
    const n = Math.max(2, Math.ceil(r.rise / 0.25));
    const cuts = [...Array.from({ length: n + 1 }, (_, i) => r.rise * i / n), r.len];
    for (let i = 0; i + 1 < cuts.length; i++) {
      const s0 = cuts[i], s1 = cuts[i + 1], k = s0 < r.rise ? 1 : 2;
      quad(at(s0, 0, y(s0)), at(s1, 0, y(s1)), at(s1, RAMP_W, y(s1)), at(s0, RAMP_W, y(s0)),
        [0, s0, k], [0, s1, k], [RAMP_W, s1, k], [RAMP_W, s0, k], 1);
      // Outer wall, up to the parapet's top, and the parapet's top.
      quad(at(s0, RAMP_W, r.y0), at(s1, RAMP_W, r.y0), at(s1, RAMP_W, y(s1) + PARAPET), at(s0, RAMP_W, y(s0) + PARAPET),
        [s0, 0, 0], [s1, 0, 0], [s1, y(s1) + PARAPET - r.y0, 0], [s0, y(s0) + PARAPET - r.y0, 0], 0.68);
      quad(at(s0, RAMP_W - 0.02, y(s0) + PARAPET), at(s1, RAMP_W - 0.02, y(s1) + PARAPET), at(s1, RAMP_W, y(s1) + PARAPET), at(s0, RAMP_W, y(s0) + PARAPET),
        [s0, 0, 0], [s1, 0, 0], [s1, 0.02, 0], [s0, 0.02, 0], 0.95);
      quad(at(s0, RAMP_W - 0.02, y(s0)), at(s1, RAMP_W - 0.02, y(s1)), at(s1, RAMP_W - 0.02, y(s1) + PARAPET), at(s0, RAMP_W - 0.02, y(s0) + PARAPET),
        [s0, 0, 0], [s1, 0, 0], [s1, PARAPET, 0], [s0, PARAPET, 0], 0.8);
    }
    // The high end: its wall down to the street, and a barrier: traffic turns onto
    // the terrace here.
    quad(at(r.len, 0, r.y0), at(r.len, RAMP_W, r.y0), at(r.len, RAMP_W, y(r.len) + PARAPET), at(r.len, 0, y(r.len) + PARAPET),
      [0, 0, 0], [RAMP_W, 0, 0], [RAMP_W, r.y1 - r.y0 + PARAPET, 0], [0, r.y1 - r.y0 + PARAPET, 0], 0.75);
    // Driveway across the terrace's sidewalk, and the apron over the street's curb.
    quad(at(r.rise, -DRIVE, y(r.len)), at(r.len, -DRIVE, y(r.len)), at(r.len, 0, y(r.len)), at(r.rise, 0, y(r.len)),
      [-DRIVE, r.rise, 2], [-DRIVE, r.len, 2], [0, r.len, 2], [0, r.rise, 2], 1);
    quad(at(-APRON, 0, r.y0 + LIFT), at(0, 0, r.y0 + LIFT), at(0, RAMP_W, r.y0 + LIFT), at(-APRON, RAMP_W, r.y0 + LIFT),
      [0, -APRON, 2], [0, 0, 2], [RAMP_W, 0, 2], [RAMP_W, -APRON, 2], 1);
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
  geo.setAttribute('aRamp', new THREE.Float32BufferAttribute(ramp, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(shade, 3));
  geo.setIndex(index);
  return geo;
}

// The ramps' and bridges' look on top of a bendable material: asphalt with edge lines
// and chevrons pointing uphill, concrete walls - or, in the other styles, a copper
// track between the parts, or a lit gangway between the platforms.
function rampMaterial(bendable) {
  const mat = bendable(new THREE.MeshBasicMaterial({ vertexColors: true, side: THREE.DoubleSide }));
  const bend = mat.onBeforeCompile;
  mat.onBeforeCompile = (shader, renderer) => {
    bend(shader, renderer);
    shader.vertexShader = 'attribute vec3 aRamp;\nvarying vec3 vRamp;\n' +
      shader.vertexShader.replace('#include <begin_vertex>', '#include <begin_vertex>\nvRamp = aRamp;');
    shader.fragmentShader = NOISE_GLSL + `
      uniform float uStyle;
      varying vec3 vRamp;
      float band(float t, float lo, float hi, float w) { return smoothstep(lo - w, lo + w, t) - smoothstep(hi - w, hi + w, t); }
    ` + shader.fragmentShader.replace('#include <color_fragment>', `
      #include <color_fragment>
      vec2 q = vRamp.xy;
      vec3 c;
      if (uStyle > 0.5) {
        // A track across the board, or a lit gangway out in the dark: the same deck,
        // with an edge and a line down the middle, in the style's own materials.
        float w = fwidth(q.x) + 1e-4;
        float wide = vRamp.z > 2.5 ? ${DECK_W} : ${RAMP_W};
        float mid = 1.0 - smoothstep(0.012 - w, 0.012 + w, abs(q.x - wide * 0.5));
        float edge = band(q.x, 0.0, 0.022, w) + band(q.x, wide - 0.022, wide, w);
        if (uStyle > 1.5) {
          c = vec3(0.05, 0.045, 0.1) * (0.8 + 0.4 * vnoise(q * 26.0));
          c += vec3(0.25, 0.8, 0.95) * (mid * 0.9 + edge * 0.5);
        } else {
          c = vec3(0.05, 0.16, 0.09) * (0.85 + 0.3 * vnoise(q * 26.0));
          c = mix(c, vec3(0.5, 0.3, 0.11), mid + edge * 0.8);
        }
      } else if (vRamp.z > 2.5) {
        // A bridge deck: asphalt with a dashed centre line and edge lines.
        float w = fwidth(q.x) + 1e-4, wa = fwidth(q.y) + 1e-4;
        c = vec3(0.045, 0.047, 0.052) * (0.8 + 0.4 * vnoise(q * 24.0));
        c = mix(c, vec3(0.7), band(q.x, 0.03, 0.05, w) + band(q.x, ${DECK_W - 0.05}, ${DECK_W - 0.03}, w));
        c = mix(c, vec3(0.78, 0.6, 0.12), band(fract(q.y / 0.5), 0.0, 0.5, wa / 0.5) * band(q.x, ${DECK_W / 2 - 0.012}, ${DECK_W / 2 + 0.012}, w));
      } else if (vRamp.z > 1.5) {
        c = vec3(0.04, 0.042, 0.047) * (0.8 + 0.4 * vnoise(q * 30.0));
      } else if (vRamp.z > 0.5) {
        float w = fwidth(q.x) + 1e-4;
        c = vec3(0.04, 0.042, 0.047) * (0.8 + 0.4 * vnoise(q * 30.0));
        c = mix(c, vec3(0.75), band(q.x, 0.012, 0.022, w) + band(q.x, ${RAMP_W - 0.042}, ${RAMP_W - 0.032}, w));
        float ch = fract((q.y + abs(q.x - ${RAMP_W / 2}) * 0.8) / 0.55); // tips uphill
        c = mix(c, vec3(0.85, 0.85, 0.8), band(ch, 0.0, 0.1, fwidth(ch) + 1e-4) * step(abs(q.x - ${RAMP_W / 2}), ${RAMP_W / 4}));
      } else {
        c = vec3(0.5, 0.49, 0.47) * (0.85 + 0.3 * vnoise(q * vec2(20.0, 40.0)));
      }
      diffuseColor.rgb *= c * 2.2;`);
  };
  mat.customProgramCacheKey = () => 'bend-ramp';
  return mat;
}

// ------------------------------------------------------------------ props

// Lamps stand on the sidewalk along each terrace's edge (the street shader's SIDEWALK).
const TREE_SPACING = 1.1, LAMP_SPACING = 1.6, LAMP_INSET = 0.035, SHORE_INSET = 0.6;
// Parks: planted where the street shader draws lawn (farther than CARRIAGE + SIDEWALK
// from every obstacle, with a margin), off the gravel paths every PARK_PATHS.
const PARK_CLEAR = 0.72, PARK_PATHS = 2.2, PATH_CLEAR = 0.16, MAX_PARK_SAMPLES = 60000;

/**
 * What stands on the map: along every shore and in every park a tall prop and a low
 * one, along every terrace's edge a light, and the ramps and bridges - as meshes that
 * bend like the map. What those props are is the style's business (PROPS): trees,
 * bushes and street lamps in a city; capacitors, resistors and LEDs on a board;
 * crystals, glowing rubble and beacons in a galaxy. Returns a Group; the lights glow
 * at night (setNight).
 */
export function makeProps(boxes, bendable, style = 'city') {
  const set = dressed(style);
  const trees = [], bushes = [], lamps = [];
  const ramps = rampsFor(boxes), bridges = bridgesFor(boxes);
  const inDrive = (x, z) => ramps.some(r => r.drive && x > r.drive.x0 - 0.2 && x < r.drive.x1 + 0.2 && z > r.drive.z0 - 0.2 && z < r.drive.z1 + 0.2);
  const tree = (x, z, y, r) => trees.push({ x, z, y, r, s: 0.7 + 0.6 * r, kind: Math.floor(rand(z * 3.1, x) * 3) });
  const bush = (x, z, y, r) => bushes.push({ x, z, y, r, s: 0.7 + 0.7 * r });
  for (const b of boxes) {
    const top = b.y + b.h;
    if (b.kind === 'land') {
      around(b, SHORE_INSET, TREE_SPACING, (x, z) => {
        const r = rand(x, z);
        if (r > 0.2) tree(x + (rand(z, x) - 0.5) * 0.3, z + (r - 0.5) * 0.3, top, r);
      });
      for (const inset of [0.28, 0.98]) {
        around(b, inset, 0.42, (x, z) => {
          const r = rand(x + inset, z);
          if (r > 0.45) bush(x + (rand(z, x + 1) - 0.5) * 0.12, z + (r - 0.5) * 0.12, top, r);
        });
      }
    } else if (b.kind === 'terrace' && b.node.kind !== 'file' && Math.min(b.w, b.d) > 2 * LAMP_INSET + 0.2) {
      around(b, LAMP_INSET, LAMP_SPACING, (x, z) => inDrive(x, z) || lamps.push({ x, z, y: top }));
    }
  }
  plantParks(boxes, tree, bush);

  const group = new THREE.Group();
  const m = new THREE.Matrix4(), q = new THREE.Quaternion(), p = new THREE.Vector3(), s = new THREE.Vector3();
  const c = new THREE.Color();
  const add = (geo, color, items, place, tint) => {
    const mesh = new THREE.InstancedMesh(geo, bendable(new THREE.MeshBasicMaterial({ color, vertexColors: true })), Math.max(1, items.length));
    mesh.count = items.length;
    items.forEach((it, i) => {
      mesh.setMatrixAt(i, place(it, m));
      if (tint) mesh.setColorAt(i, tint(it, c));
    });
    mesh.frustumCulled = false;
    mesh.userData.day = color;
    group.add(mesh);
    return mesh;
  };
  const plantAt = (it, m) => m.compose(p.set(it.x, it.y, it.z), q.setFromAxisAngle(UP, it.r * 6.28), s.setScalar(it.s));
  const lampAt = (it, m) => m.compose(p.set(it.x, it.y, it.z), q.identity(), s.setScalar(1));
  set.species.forEach((t, kind) => {
    const these = trees.filter(it => it.kind === kind);
    add(t.stem, set.stem, these, plantAt);
    add(t.head, '#ffffff', these, plantAt, set.tint(t.hue));
  });
  add(set.low, '#ffffff', bushes, plantAt, set.tint(set.lowHue));
  add(set.pole, set.poleColor, lamps, lampAt);
  group.userData.heads = add(set.lampHead, set.headColor, lamps, lampAt);
  group.userData.night = set.headNight;
  // What a walker cannot walk through. A trunk, a capacitor's case, a crystal and a
  // lamp post are all a circle standing on a spot; a bush is something to walk over.
  group.userData.obstacles = [
    ...trees.map(it => ({ x: it.x, z: it.z, y: it.y, r: set.solid * it.s })),
    ...lamps.map(it => ({ x: it.x, z: it.z, y: it.y, r: set.post })),
  ];
  for (const geo of [ramps.length && rampGeometry(ramps), bridges.length && bridgeGeometry(bridges)]) {
    if (!geo) continue;
    const mesh = new THREE.Mesh(geo, rampMaterial(bendable));
    mesh.frustumCulled = false;
    mesh.userData.day = '#ffffff';
    group.add(mesh);
  }
  return group;
}

// Samples each block's lawn (the park the street shader draws) on a jittered grid
// and plants trees and bushes there.
function plantParks(boxes, tree, bush) {
  const all = blocks(boxes);
  const area = [...all.keys()].reduce((a, t) => a + t.w * t.d, 0);
  const step = Math.max(0.6, Math.sqrt(area / MAX_PARK_SAMPLES));
  const offPath = v => Math.abs(((v / PARK_PATHS) % 1 + 1) % 1 - 0.5) * PARK_PATHS > PATH_CLEAR;
  for (const [t, kids] of all) {
    const top = t.y + t.h;
    const x0 = t.x - t.w / 2 + PARK_CLEAR, x1 = t.x + t.w / 2 - PARK_CLEAR;
    const z0 = t.z - t.d / 2 + PARK_CLEAR, z1 = t.z + t.d / 2 - PARK_CLEAR;
    if (x1 <= x0 || z1 <= z0) continue;
    // Kids by grid cell, for the distance test.
    const cell = 1.5, grid = new Map();
    for (const k of kids) {
      for (let i = Math.floor((k.x - k.w / 2) / cell); i <= Math.floor((k.x + k.w / 2) / cell); i++) {
        for (let j = Math.floor((k.z - k.d / 2) / cell); j <= Math.floor((k.z + k.d / 2) / cell); j++) {
          const key = i + ',' + j;
          if (!grid.has(key)) grid.set(key, []);
          grid.get(key).push(k);
        }
      }
    }
    const clear = (x, z) => {
      const i = Math.floor(x / cell), j = Math.floor(z / cell);
      for (let di = -1; di <= 1; di++) for (let dj = -1; dj <= 1; dj++) {
        for (const k of grid.get(i + di + ',' + (j + dj)) || []) {
          if (rectDist({ x0: x, x1: x, z0: z, z1: z }, k) < PARK_CLEAR) return false;
        }
      }
      return true;
    };
    for (let z = z0; z <= z1; z += step) {
      for (let x = x0; x <= x1; x += step) {
        const px = x + (rand(x, z) - 0.5) * step * 0.8, pz = z + (rand(z, x) - 0.5) * step * 0.8;
        if (px < x0 || px > x1 || pz < z0 || pz > z1 || !offPath(px) || !offPath(pz) || !clear(px, pz)) continue;
        const r = rand(px * 1.7, pz * 2.3);
        if (r < 0.16) tree(px, pz, top, r / 0.16);
        else if (r < 0.5) bush(px, pz, top, (r - 0.16) / 0.34);
      }
    }
  }
}

/** Day or night for the props: the lights come on, everything else darkens. */
export function setNight(group, night) {
  for (const mesh of group.children) {
    if (mesh === group.userData.heads && night) mesh.material.color.set(group.userData.night);
    else mesh.material.color.set(mesh.userData.day).multiplyScalar(night ? 0.4 : 1);
  }
}

const UP = new THREE.Vector3(0, 1, 0);

// Calls fn at points spaced along a rectangle inset from a box's edges.
function around(b, inset, spacing, fn) {
  const x0 = b.x - b.w / 2 + inset, x1 = b.x + b.w / 2 - inset, z0 = b.z - b.d / 2 + inset, z1 = b.z + b.d / 2 - inset;
  if (x1 <= x0 || z1 <= z0) return;
  const side = (ax, az, bx, bz) => {
    const n = Math.max(1, Math.round(Math.hypot(bx - ax, bz - az) / spacing));
    for (let i = 0; i < n; i++) fn(ax + (bx - ax) * i / n, az + (bz - az) * i / n, i);
  };
  side(x0, z0, x1, z0); side(x1, z0, x1, z1); side(x1, z1, x0, z1); side(x0, z1, x0, z0);
}

// Deterministic 0..1 from a position, so props stay put across relayouts.
function rand(x, z) {
  const v = Math.sin(x * 12.9898 + z * 78.233) * 43758.5453;
  return v - Math.floor(v);
}

// Geometries with vertex colors as fixed shading: lighter facing up and towards the
// light, darker towards the base (self-shadowing), with a little per-vertex jitter so
// foliage does not look faceted.
function shaded(geo, jitter = 0) {
  geo = geo.index ? geo.toNonIndexed() : geo;
  geo.computeVertexNormals();
  geo.computeBoundingBox();
  const { min, max } = geo.boundingBox;
  const n = geo.getAttribute('normal'), pos = geo.getAttribute('position'), col = [];
  for (let i = 0; i < n.count; i++) {
    const up = (pos.getY(i) - min.y) / Math.max(1e-6, max.y - min.y);
    const k = (0.5 + 0.4 * Math.max(0, n.getY(i)) + 0.12 * n.getX(i) - 0.06 * n.getZ(i)) * (0.72 + 0.28 * up)
      * (1 + jitter * (rand(pos.getX(i) * 7.1, pos.getZ(i) * 5.3 + pos.getY(i)) - 0.5));
    col.push(k, k, k);
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(col, 3));
  return geo;
}

// One non-indexed geometry from several (position and color only, as shaded makes them).
function merge(geos) {
  const out = new THREE.BufferGeometry();
  for (const name of ['position', 'color']) {
    const parts = geos.map(g => g.getAttribute(name).array);
    const all = new Float32Array(parts.reduce((a, p) => a + p.length, 0));
    let o = 0;
    for (const p of parts) { all.set(p, o); o += p.length; }
    out.setAttribute(name, new THREE.BufferAttribute(all, 3));
  }
  return out;
}

const blob = (r, x, y, z, sy = 1) => shaded(new THREE.IcosahedronGeometry(r, 1).scale(1, sy, 1).translate(x, y, z), 0.25);
const trunk = (h, r) => shaded(new THREE.CylinderGeometry(r * 0.7, r, h, 6).translate(0, h / 2, 0));

// Tree species: a broadleaf with a crown of several blobs, a conifer of stacked cones,
// and a slender poplar. hue: their foliage's base hue.
const TREES = [
  {
    trunk: trunk(0.3, 0.04), hue: 0.24,
    crown: merge([blob(0.19, 0, 0.47, 0), blob(0.14, 0.12, 0.4, 0.05), blob(0.14, -0.1, 0.42, -0.07), blob(0.12, 0.02, 0.6, -0.04)]),
  },
  {
    trunk: trunk(0.14, 0.035), hue: 0.3,
    crown: merge([0, 1, 2].map(i => shaded(new THREE.ConeGeometry(0.2 - i * 0.05, 0.3, 8).translate(0, 0.25 + i * 0.16, 0), 0.2))),
  },
  { trunk: trunk(0.2, 0.03), hue: 0.21, crown: merge([blob(0.12, 0, 0.5, 0, 2.6)]) },
];
const BUSH = merge([blob(0.085, 0, 0.055, 0, 0.8), blob(0.065, 0.07, 0.04, 0.03, 0.8), blob(0.06, -0.06, 0.04, -0.04, 0.8)]);
const POLE = shaded(new THREE.CylinderGeometry(0.012, 0.018, 0.6, 5).translate(0, 0.3, 0));
const HEAD = shaded(new THREE.BoxGeometry(0.07, 0.025, 0.05).translate(0, 0.61, 0));

// ------------------------------------------------------------------ the other styles' props

const cyl = (r0, r1, h, y, seg = 10) => shaded(new THREE.CylinderGeometry(r0, r1, h, seg).translate(0, y + h / 2, 0));
const legs = (h, dx) => merge([
  shaded(new THREE.CylinderGeometry(0.008, 0.008, h, 5).translate(dx, h / 2, 0)),
  shaded(new THREE.CylinderGeometry(0.008, 0.008, h, 5).translate(-dx, h / 2, 0)),
]);

// Parts soldered to the board. A through-hole electrolytic capacitor with its vent
// cross and polarity stripe, a small transistor in a TO-92 case, and a wound inductor.
const PARTS = [
  {
    hue: 0.62, // the dark blue a capacitor's sleeve usually is
    stem: legs(0.1, 0.022),
    head: merge([
      cyl(0.085, 0.085, 0.38, 0.1),
      shaded(new THREE.CylinderGeometry(0.087, 0.087, 0.02, 12).translate(0, 0.475, 0)),  // the crimp
      shaded(new THREE.BoxGeometry(0.03, 0.004, 0.17).translate(0, 0.487, 0)),            // the vent cross
      shaded(new THREE.BoxGeometry(0.17, 0.004, 0.03).translate(0, 0.487, 0)),
    ]),
  },
  {
    hue: 0.0,
    stem: legs(0.08, 0.016),
    head: merge([
      shaded(new THREE.CylinderGeometry(0.075, 0.075, 0.16, 12, 1, false, -Math.PI / 2, Math.PI).translate(0, 0.16, 0)),
      shaded(new THREE.BoxGeometry(0.15, 0.16, 0.03).translate(0, 0.16, 0.012)),
    ]),
  },
  {
    hue: 0.09,
    stem: legs(0.07, 0.03),
    head: merge([
      cyl(0.06, 0.06, 0.24, 0.07, 8),
      ...[0, 1, 2, 3].map(i => shaded(new THREE.TorusGeometry(0.065, 0.014, 5, 10).rotateX(Math.PI / 2).translate(0, 0.11 + i * 0.055, 0))),
    ]),
  },
];
// A surface-mount resistor: a body with tin ends, lying on its pads.
const SMD = merge([
  shaded(new THREE.BoxGeometry(0.13, 0.05, 0.07).translate(0, 0.025, 0)),
  shaded(new THREE.BoxGeometry(0.03, 0.052, 0.072).translate(0.055, 0.026, 0)),
  shaded(new THREE.BoxGeometry(0.03, 0.052, 0.072).translate(-0.055, 0.026, 0)),
]);
const LED_LEGS = merge([
  shaded(new THREE.CylinderGeometry(0.009, 0.009, 0.42, 5).translate(0.016, 0.21, 0)),
  shaded(new THREE.CylinderGeometry(0.009, 0.009, 0.36, 5).translate(-0.016, 0.18, 0)),
]);
const LED = merge([
  shaded(new THREE.CylinderGeometry(0.038, 0.042, 0.08, 10).translate(0, 0.46, 0)),
  shaded(new THREE.SphereGeometry(0.038, 10, 6, 0, Math.PI * 2, 0, Math.PI / 2).translate(0, 0.54, 0)),
]);

// What grows out in the dark: spires of crystal, and rubble that still glows.
const shard = (r, h, x, y, z, tilt = 0) => shaded(
  new THREE.ConeGeometry(r, h, 5).rotateZ(tilt).translate(x, y + h / 2, z), 0.15);
const CRYSTALS = [
  { hue: 0.72, stem: shard(0.06, 0.1, 0, 0, 0), head: merge([shard(0.09, 0.62, 0, 0.04, 0), shard(0.05, 0.3, 0.08, 0.02, 0.05, 0.3)]) },
  {
    hue: 0.52, stem: shard(0.07, 0.08, 0, 0, 0),
    head: merge([shard(0.07, 0.34, 0.06, 0.02, 0.02, 0.35), shard(0.06, 0.44, -0.05, 0.02, -0.03, -0.28), shard(0.05, 0.26, 0.01, 0.02, -0.08, 0.12)]),
  },
  { hue: 0.85, stem: shard(0.09, 0.07, 0, 0, 0), head: merge([shard(0.13, 0.28, 0, 0.03, 0)]) },
];
const RUBBLE = merge([blob(0.07, 0, 0.045, 0, 0.7), blob(0.05, 0.06, 0.03, 0.02, 0.7), blob(0.045, -0.05, 0.03, -0.04, 0.7)]);
const BEACON_STEM = shaded(new THREE.CylinderGeometry(0.008, 0.016, 0.44, 5).translate(0, 0.22, 0));
const BEACON = merge([
  shaded(new THREE.IcosahedronGeometry(0.055, 1).translate(0, 0.52, 0)),
  shaded(new THREE.TorusGeometry(0.075, 0.006, 5, 14).rotateX(Math.PI / 2).translate(0, 0.52, 0)),
]);

// Each style's props, in the same four roles: a tall one for shores and parks, a low
// one beside it, and a light with its stem for the terrace edges.
const PROPS = {
  city: {
    species: TREES, stem: '#5a4030', low: BUSH, lowHue: 0.25,
    pole: POLE, poleColor: '#3a3d42', lampHead: HEAD, headColor: '#8a8d92', headNight: '#ffd28a',
    solid: 0.055, post: 0.03,
    tint: hue => (it, c) => c.setHSL(hue + it.r * 0.07, 0.5 + 0.2 * it.r, 0.2 + it.r * 0.1),
  },
  circuit: {
    species: PARTS, stem: '#b9bec6', low: SMD, lowHue: 0.09,
    pole: LED_LEGS, poleColor: '#b9bec6', lampHead: LED, headColor: '#c94a3a', headNight: '#ff6a52',
    solid: 0.09, post: 0.03,
    // Parts are made in a handful of colors, not a spectrum: a little jitter around
    // the one the part type is usually sold in.
    tint: hue => (it, c) => c.setHSL(hue + (it.r - 0.5) * 0.04, hue < 0.05 ? 0.05 : 0.55, 0.12 + it.r * 0.12),
  },
  galaxy: {
    species: CRYSTALS, stem: '#2b2540', low: RUBBLE, lowHue: 0.68,
    pole: BEACON_STEM, poleColor: '#2b2540', lampHead: BEACON, headColor: '#4fd0e8', headNight: '#9df0ff',
    solid: 0.085, post: 0.025,
    tint: hue => (it, c) => c.setHSL(hue + it.r * 0.12, 0.6 + 0.25 * it.r, 0.3 + it.r * 0.22),
  },
};

// The city's plants, and the galaxy's rubble, are models when props.js has them: the
// same trunk-and-crown shape as the blobs above, but a shape somebody drew. They are
// shaded here like everything else, so they take the map's flat paint and its
// per-instance tint and read as part of it rather than as an import.
//
// Built once, the first time makeProps runs after the models land.
let modelled = null;

const MODELS = ['tree1_stem', 'tree1_head', 'tree2_stem', 'tree2_head',
  'tree3_stem', 'tree3_head', 'bush_head', 'rock_head'];

function dressed(style) {
  const set = PROPS[style] || PROPS.city;
  const got = plants();
  // A file that loaded but is missing a part would be worse than no file at all.
  if (!got || MODELS.some(name => !got.has(name))) return set;
  modelled ||= {
    city: {
      ...PROPS.city,
      species: [1, 2, 3].map((n, i) => ({
        hue: PROPS.city.species[i].hue,
        stem: shaded(got.get(`tree${n}_stem`)),
        head: shaded(got.get(`tree${n}_head`), 0.16),
      })),
      low: shaded(got.get('bush_head'), 0.16),
    },
    galaxy: { ...PROPS.galaxy, low: shaded(got.get('rock_head'), 0.12) },
  };
  return modelled[style] || set;
}
