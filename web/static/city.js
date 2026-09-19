// The city look, all procedural (no image assets): shader "textures" for the boxes -
// building facades and roofs, a street network on every terrace, stairs between
// terrace levels, grassy shores - plus trees, bushes, street lamps and ramps, on the
// isometric map and in walk mode alike; walk mode adds a sky dome and rippling water.
// The textures are unlit and antialiased by pixel footprint, so detail fades out
// when zoomed out rather than flickering.
//
// Streets are the free space of a terrace top: everything not covered by a child
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

// The top of a terrace: streets between its children. lp: position from the base
// centre, sz: the terrace's size.
vec3 streets(vec3 base, vec3 lp, vec3 sz) {
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
  float d1 = d[a];
  vec2 n1 = n[a];
  // The nearest obstacle facing it across the street, if the street is straight here.
  int b = -1;
  float d2 = 1e9;
  for (int j = 0; j < 12; j++) if (dot(n[j], n1) < -0.95 && d[j] < d2) { d2 = d[j]; b = j; }

  float w = fwidth(d1) + 1e-4;
  vec3 c = asphalt(p);
  if (b >= 0 && d1 + d2 > 2.0 * CARRIAGE) b = -1; // a park between: two streets, not one
  if (b < 0 && d1 < CARRIAGE) {
    // A street along a park: the dashed line on its middle.
    float along = abs(n1.x) < 0.5 ? p.x : p.y, wa = fwidth(along) + 1e-4;
    float mid = (SIDEWALK + CARRIAGE) * 0.5;
    float dash = band(fract(along / 0.3), 0.0, 0.5, wa / 0.3) * band(d1, mid - 0.008, mid + 0.008, w);
    c = mix(c, vec3(0.78, 0.6, 0.12), dash * (1.0 - smoothstep(0.02, 0.06, wa)));
  }
  if (b >= 0) {
    float width = d1 + d2, s = (d2 - d1) * 0.5; // s: distance from the centre line
    bool xRoad = abs(n1.x) < 0.5;               // the street runs along x
    float along = xRoad ? p.x : p.y, across = xRoad ? p.y : p.x;
    vec2 e = vec2(max(ext[a].x, ext[b].x), min(ext[a].y, ext[b].y)); // where both sides face each other
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

vec3 cityTexture(vec3 base) {
  vec3 n = normalize(vObjN);
  float k = floor(vKind + 0.5);

  vec3 lp = vLP, sz = vSize;
  if (n.y > 0.5) {
    float ex = sz.x * 0.5 - abs(lp.x), ez = sz.z * 0.5 - abs(lp.z);
    float e = min(ex, ez);
    vec2 p = lp.xz + vSeed;
    if (k < 0.5) return grass(base, p) * tint(base, uLandRef);
    if (k < 1.5) return streets(base, lp, sz);
    if (k > 5.5) return paving(base, p);
    return roof(base, lp, sz, e);
  }
  if (n.y < -0.5) return base;
  bool sideX = abs(n.x) > 0.5;
  float u = sideX ? lp.z * sign(n.x) : -lp.x * sign(n.z);
  float faceW = sideX ? sz.z : sz.x;
  // Keep the per-face shade the flat colors carry.
  float shade = sideX ? 0.62 : 0.78;
  if (k < 0.5) return retaining(vec3(0.3, 0.24, 0.16), vec2(u, lp.y), 0.0) * shade / 0.7;
  if (k > 5.5) return retaining(base, vec2(u, lp.y), 0.35);
  if (k < 1.5) return stairs(retaining(base, vec2(u, lp.y), 0.35), u, faceW, lp.y, sz.y);
  return facade(base, u, faceW, lp.y, sz.y, vSeed + n.xz * 3.1);
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
      uniform float uNight, uTime;
      varying vec3 vDir;
      void main() {
        vec3 d = normalize(vDir);
        float up = max(d.y, 0.0);
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
    shader.fragmentShader = NOISE_GLSL + 'uniform float uTime, uNight;\nvarying vec3 vW;\n' + shader.fragmentShader.replace('#include <color_fragment>', `
      #include <color_fragment>
      float r = 0.5 * vnoise(vW.xz * 1.6 + vec2(uTime * 0.35, uTime * 0.2)) + 0.5 * vnoise(vW.xz * 4.5 - uTime * 0.3);
      vec2 fw = fwidth(vW.xz * 4.5);
      r = mix(r, 0.5, smoothstep(0.3, 1.0, max(fw.x, fw.y)));
      diffuseColor.rgb *= 0.8 + 0.4 * r;
      diffuseColor.rgb += smoothstep(0.8, 0.95, r) * mix(0.1, 0.03, uNight);`);
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

// The ramps' look on top of a bendable material: asphalt with edge lines and chevrons
// pointing uphill, concrete walls.
function rampMaterial(bendable) {
  const mat = bendable(new THREE.MeshBasicMaterial({ vertexColors: true, side: THREE.DoubleSide }));
  const bend = mat.onBeforeCompile;
  mat.onBeforeCompile = (shader, renderer) => {
    bend(shader, renderer);
    shader.vertexShader = 'attribute vec3 aRamp;\nvarying vec3 vRamp;\n' +
      shader.vertexShader.replace('#include <begin_vertex>', '#include <begin_vertex>\nvRamp = aRamp;');
    shader.fragmentShader = NOISE_GLSL + `
      varying vec3 vRamp;
      float band(float t, float lo, float hi, float w) { return smoothstep(lo - w, lo + w, t) - smoothstep(hi - w, hi + w, t); }
    ` + shader.fragmentShader.replace('#include <color_fragment>', `
      #include <color_fragment>
      vec2 q = vRamp.xy;
      vec3 c;
      if (vRamp.z > 2.5) {
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
 * Trees and bushes along every shore and in parks, lamps along every terrace's edge,
 * and the ramps, as meshes that bend like the map. Returns a Group; lamps' heads glow
 * at night (setNight).
 */
export function makeProps(boxes, bendable) {
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
  const leaves = hue => (it, c) => c.setHSL(hue + it.r * 0.07, 0.5 + 0.2 * it.r, 0.2 + it.r * 0.1);
  TREES.forEach((t, kind) => {
    const these = trees.filter(it => it.kind === kind);
    add(t.trunk, '#5a4030', these, plantAt);
    add(t.crown, '#ffffff', these, plantAt, leaves(t.hue));
  });
  add(BUSH, '#ffffff', bushes, plantAt, leaves(0.25));
  add(POLE, '#3a3d42', lamps, lampAt);
  group.userData.heads = add(HEAD, '#8a8d92', lamps, lampAt);
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

/** Day or night for the props: lamp heads glow, foliage darkens. */
export function setNight(group, night) {
  for (const mesh of group.children) {
    if (mesh === group.userData.heads && night) mesh.material.color.set('#ffd28a');
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
