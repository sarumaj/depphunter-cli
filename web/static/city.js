// The city look of walk mode, all procedural (no image assets): a sky dome, rippling
// water, and shader "textures" for the boxes — building facades with windows and
// roofs, terraces as city blocks ringed by roads, grassy shores — plus trees and
// street lamps. The isometric map is untouched: every effect is gated on uBend.
//
// Colours stay data first: facades and roofs are modulated from the box's own
// colour (language, size, history, dimming, hover), never replaced.

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
attribute vec3 aBoxCenter; // base centre, for non-instanced boxes
attribute vec3 aBoxSize;
varying vec3 vLP;          // position relative to the box's base centre, world units
varying vec3 vObjN;
// Per box. Flat: interpolation noise in a seed, run through a hash, speckles windows.
flat varying vec3 vSize;
flat varying float vKind;
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
`;

/** Fragment declarations: surface functions, all in linear colour. */
export const CITY_FRAG_HEAD = NOISE_GLSL + `
uniform float uBend;
uniform float uNight;
varying vec3 vLP;
varying vec3 vObjN;
flat varying vec3 vSize;
flat varying float vKind;
flat varying vec2 vSeed;

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

vec3 grass(vec3 base, vec2 p) {
  vec3 g = mix(vec3(0.07, 0.16, 0.035), vec3(0.16, 0.28, 0.06), vnoise(p * 2.5));
  g *= 0.8 + 0.4 * vnoise(p * 22.0);
  return mix(g, base, 0.12) * dark(0.35);
}

// A terrace top is a city block: a ring road with a dashed centre line between two
// curbs, and paved sidewalk inside where the buildings stand.
vec3 street(vec3 base, float e, float along, vec2 p) {
  float w = fwidth(e) + 1e-4, wa = fwidth(along) + 1e-4;
  vec3 c = vec3(0.028, 0.03, 0.034) * (0.75 + 0.5 * vnoise(p * 28.0));
  float dash = band(fract(along / 0.34), 0.08, 0.6, wa / 0.34) * band(e, 0.262, 0.288, w);
  c = mix(c, vec3(0.75, 0.55, 0.1), dash * (1.0 - smoothstep(0.015, 0.05, w)));
  c = mix(c, vec3(0.4, 0.4, 0.38), 1.0 - smoothstep(0.05 - w, 0.05 + w, e));
  c = mix(c, base, 0.1); // the block keeps its tint, and hover shows
  c *= dark(0.5);
  return mix(c, paving(base, p), smoothstep(0.47 - w, 0.47 + w, e));
}

vec3 roof(vec3 base, vec3 lp, vec3 sz, float e) {
  float w = fwidth(e) + 1e-4;
  vec3 c = base * 0.8 * (0.9 + 0.2 * vnoise(lp.xz * 16.0 + vSeed));
  c = mix(c, base * 1.08, 1.0 - smoothstep(0.035 - w, 0.035 + w, e)); // parapet
  vec2 at = (vec2(hash12(vSeed), hash12(vSeed + 5.3)) - 0.5) * sz.xz * 0.35;
  vec2 q = abs(lp.xz - at);
  float s = min(sz.x, sz.z) * 0.15;
  float unit = 1.0 - smoothstep(s - w, s + w, max(q.x, q.y)); // a rooftop plant room
  c = mix(c, vec3(0.3, 0.31, 0.33), unit * 0.85);
  return c * dark(0.55);
}

// A facade: whole window bays across the face, 0.3-unit storeys, a shopfront on the
// ground floor and a cornice on top. Lit windows glow, more of them at night.
vec3 facade(vec3 base, float u, float faceW, float v, float h, vec2 seed) {
  float bays = max(1.0, floor(faceW / 0.24));
  vec2 cell = vec2((u + faceW * 0.5) / (faceW / bays), v / 0.3);
  vec2 f = fract(cell), w = fwidth(cell) + 1e-4;
  float far = smoothstep(0.3, 0.7, max(w.x, w.y));
  float id = hash12(floor(cell) + seed * 1.37);
  float ground = 1.0 - step(1.0, cell.y);
  float top = step(h - 0.07, v);
  float win = band(f.x, 0.2, 0.8, w.x) * band(f.y, 0.3, 0.82, w.y) * (1.0 - ground) * (1.0 - top);
  float shop = band(f.x, 0.06, 0.94, w.x) * band(f.y, 0.08, 0.72, w.y) * ground * (1.0 - top);
  float litShare = mix(0.1, 0.55, uNight);
  float lit = step(1.0 - litShare, id);
  vec3 glass = mix(vec3(0.16, 0.22, 0.3), vec3(0.01, 0.014, 0.02), uNight) * (0.7 + 0.6 * hash12(floor(cell) + seed));
  vec3 warm = vec3(1.0, 0.7, 0.32) * (0.55 + 0.45 * id);
  vec3 wall = base * (0.92 + 0.12 * vnoise(vec2(u, v) * 26.0)) * dark(0.5);
  wall = mix(wall, wall * 1.2, top);
  wall *= 1.0 - 0.18 * band(f.y, 0.94, 1.0, w.y) * (1.0 - far); // floor lines
  vec3 c = mix(wall, mix(glass, warm, lit), win);
  c = mix(c, mix(glass * 1.4, warm, uNight * 0.8), shop);
  vec3 avg = mix(wall, mix(glass, warm, litShare), mix(0.3, 0.45, uNight));
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

vec3 cityColor(vec3 base) {
  vec3 n = normalize(vObjN);
  float k = floor(vKind + 0.5);
  vec3 lp = vLP, sz = vSize;
  if (n.y > 0.5) {
    float ex = sz.x * 0.5 - abs(lp.x), ez = sz.z * 0.5 - abs(lp.z);
    float e = min(ex, ez);
    vec2 p = lp.xz + vSeed;
    if (k < 0.5) return grass(base, p);
    if (k < 1.5) return street(base, e, ex < ez ? lp.z : lp.x, p);
    if (k > 5.5) return paving(base, p);
    return roof(base, lp, sz, e);
  }
  if (n.y < -0.5) return base;
  bool sideX = abs(n.x) > 0.5;
  float u = sideX ? lp.z * sign(n.x) : -lp.x * sign(n.z);
  float faceW = sideX ? sz.z : sz.x;
  // Keep the per-face shade the flat colours carry.
  float shade = sideX ? 0.62 : 0.78;
  if (k < 0.5) return retaining(vec3(0.3, 0.24, 0.16), vec2(u, lp.y), 0.0) * shade / 0.7;
  if (k < 1.5 || k > 5.5) return retaining(base, vec2(u, lp.y), 0.35);
  return facade(base, u, faceW, lp.y, sz.y, vSeed + n.xz * 3.1);
}
`;

export const CITY_FRAG_BODY = `
if (uBend > 0.5) diffuseColor.rgb = cityColor(diffuseColor.rgb);
`;

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

/** Patches the planet's material with slow ripples. */
export function waterMaterial(uniforms) {
  const mat = new THREE.MeshBasicMaterial();
  mat.onBeforeCompile = shader => {
    Object.assign(shader.uniforms, uniforms);
    shader.vertexShader = 'varying vec3 vW;\n' + shader.vertexShader.replace('#include <project_vertex>',
      '#include <project_vertex>\nvW = (modelMatrix * vec4(position, 1.0)).xyz;');
    shader.fragmentShader = NOISE_GLSL + 'uniform float uTime, uNight;\nvarying vec3 vW;\n' + shader.fragmentShader.replace('#include <color_fragment>', `
      #include <color_fragment>
      float r = 0.5 * vnoise(vW.xz * 1.6 + vec2(uTime * 0.35, uTime * 0.2)) + 0.5 * vnoise(vW.xz * 4.5 - uTime * 0.3);
      diffuseColor.rgb *= 0.8 + 0.4 * r;
      diffuseColor.rgb += smoothstep(0.8, 0.95, r) * mix(0.1, 0.03, uNight);`);
  };
  mat.customProgramCacheKey = () => 'water';
  return mat;
}

// ------------------------------------------------------------------ props

const TREE_SPACING = 1.1, LAMP_SPACING = 1.6, LAMP_INSET = 0.5, SHORE_INSET = 0.6;

/**
 * Trees along every shore and lamps along every block's road, as instanced meshes
 * that bend like the map. Returns a Group; lamps' heads glow at night (setNight).
 */
export function makeProps(boxes, bendable) {
  const trees = [], lamps = [];
  for (const b of boxes) {
    const top = b.y + b.h;
    if (b.kind === 'land') {
      around(b, SHORE_INSET, TREE_SPACING, (x, z, i) => {
        const r = rand(x, z);
        if (r > 0.2) trees.push({ x: x + (rand(z, x) - 0.5) * 0.3, z: z + (r - 0.5) * 0.3, y: top, s: 0.7 + 0.6 * r, r });
      });
    } else if (b.kind === 'terrace' && b.node.kind !== 'file' && Math.min(b.w, b.d) > 2 * LAMP_INSET + 0.2) {
      around(b, LAMP_INSET, LAMP_SPACING, (x, z) => lamps.push({ x, z, y: top }));
    }
  }
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
  const treeAt = (it, m) => m.compose(p.set(it.x, it.y, it.z), q.setFromAxisAngle(UP, it.r * 6.28), s.setScalar(it.s));
  const lampAt = (it, m) => m.compose(p.set(it.x, it.y, it.z), q.identity(), s.setScalar(1));
  add(TRUNK, '#5a4030', trees, treeAt);
  add(CROWN, '#ffffff', trees, treeAt, (it, c) => c.setHSL(0.24 + it.r * 0.08, 0.55, 0.22 + it.r * 0.1));
  add(POLE, '#3a3d42', lamps, lampAt);
  group.userData.heads = add(HEAD, '#8a8d92', lamps, lampAt);
  return group;
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

// Geometries with vertex colours as fixed shading: lighter facing up.
function shaded(geo) {
  geo = geo.index ? geo.toNonIndexed() : geo;
  geo.computeVertexNormals();
  const n = geo.getAttribute('normal'), col = [];
  for (let i = 0; i < n.count; i++) {
    const k = 0.55 + 0.45 * Math.max(0, n.getY(i)) + 0.1 * n.getX(i);
    col.push(k, k, k);
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(col, 3));
  return geo;
}
const TRUNK = shaded(new THREE.CylinderGeometry(0.03, 0.045, 0.28, 5).translate(0, 0.14, 0));
const CROWN = shaded(new THREE.IcosahedronGeometry(0.2, 0).scale(1, 1.25, 1).translate(0, 0.45, 0));
const POLE = shaded(new THREE.CylinderGeometry(0.012, 0.018, 0.6, 5).translate(0, 0.3, 0));
const HEAD = shaded(new THREE.BoxGeometry(0.07, 0.025, 0.05).translate(0, 0.61, 0));
