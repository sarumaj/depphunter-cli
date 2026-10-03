// What the walker's clothes are made of, as it looks: a fabric's weave, knit or quilting
// in its shading and in the light off it, so a sleeve reads as cloth over an arm and not
// as an arm painted another color. And the sleeves and gloves themselves, cut from the
// arm's own mesh as layers over it, with an edge where they end.
//
// The pattern is worked out per pixel from where on the model a point is - the hands
// and legs carry no texture coordinates - and fades to its average where it would be
// finer than a pixel. Each vertex says which fabric it is (`cloth`, one of CLOTH).
//
// Implements: REQ-WALK-059

import * as THREE from '../vendor/three.module.min.js';

/** The fabrics: bare skin, a T-shirt's knit, a twill (denim, a coverall), rubber, a spacesuit's quilting, leather. */
export const CLOTH = { skin: 0, knit: 1, twill: 2, rubber: 3, suit: 4, leather: 5 };

const GLSL = /* glsl */ `
varying vec3 vClothAt;
varying float vCloth;
float clothHash(vec3 p) { return fract(sin(dot(p, vec3(12.9898, 78.233, 37.719))) * 43758.5453); }
float clothNoise(vec3 p) {
  vec3 i = floor(p), f = fract(p);
  f = f * f * (3.0 - 2.0 * f);
  return mix(mix(mix(clothHash(i), clothHash(i + vec3(1, 0, 0)), f.x), mix(clothHash(i + vec3(0, 1, 0)), clothHash(i + vec3(1, 1, 0)), f.x), f.y),
    mix(mix(clothHash(i + vec3(0, 0, 1)), clothHash(i + vec3(1, 0, 1)), f.x), mix(clothHash(i + vec3(0, 1, 1)), clothHash(i + vec3(1, 1, 1)), f.x), f.y), f.z);
}
// The fabric's relief at p (0..1): its weave, knit or seams; 0.5 where there is none.
float clothHeight(vec3 p, float kind) {
  float fine = 1.0 - smoothstep(0.04, 0.25, length(fwidth(p))); // gone before a repeat is a few pixels
  float h = 0.5;
  if (kind > 0.5 && kind < 1.5) h = 0.5 + 0.5 * sin(6.2832 * (p.x + p.z)) * sin(6.2832 * p.y * 1.5);  // knit
  else if (kind < 2.5) h = 0.5 + 0.5 * sin(6.2832 * (p.y * 1.6 + 0.8 * (p.x + p.z)));                 // twill
  else if (kind < 3.5) h = 0.5 + 0.2 * (clothNoise(p * 0.4) - 0.5);                                   // rubber
  else if (kind < 4.5) {                                                                              // quilting
    vec3 q = 0.5 - abs(fract(p * 0.06) - 0.5); // how far from the nearest seam, each way
    h = 0.75 + 0.1 * clothNoise(p * 0.2) - 0.45 * (1.0 - smoothstep(0.0, 0.035, min(q.y, min(q.x, q.z))));
  } else if (kind < 5.5) h = 0.5 + 0.25 * (clothNoise(p * 0.8) - 0.5);                               // leather
  return mix(0.5, h, fine);
}
// A fabric's color, shaded by its relief and a little uneven, as dyed cloth is.
vec3 clothTone(vec3 rgb, vec3 p, float kind, float h) {
  if (kind < 0.5) return rgb;
  float uneven = 0.94 + 0.12 * clothNoise(p * 0.15);
  return rgb * uneven * mix(0.92, 1.04, h);
}
// The normal bent by the relief, as a bump map would bend it.
vec3 clothBump(vec3 at, vec3 n, float h, float amount) {
  vec3 sx = dFdx(at), sy = dFdy(at);
  vec3 r1 = cross(sy, n), r2 = cross(n, sx);
  float det = dot(sx, r1);
  vec2 dh = vec2(dFdx(h), dFdy(h)) * amount;
  vec3 grad = sign(det) * (dh.x * r1 + dh.y * r2);
  return normalize(abs(det) * n - grad);
}
`;

/**
 * Makes `material`, a lit one, draw fabric: its `cloth` attribute picks the fabric and
 * `scale` is how many repeats of a weave fit in one unit of the model. The relief goes
 * into its normals, and it is matte where it is cloth.
 */
export function fabric(material, scale) {
  material.onBeforeCompile = shader => {
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', '#include <common>\nattribute float cloth;\nvarying vec3 vClothAt;\nvarying float vCloth;')
      .replace('#include <begin_vertex>', `#include <begin_vertex>\nvClothAt = position * ${scale.toFixed(2)};\nvCloth = cloth;`);
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', `#include <common>\n${GLSL}`)
      .replace('#include <color_fragment>', '#include <color_fragment>\nfloat clothH = clothHeight(vClothAt, vCloth);\ndiffuseColor.rgb = clothTone(diffuseColor.rgb, vClothAt, vCloth, clothH);')
      .replace('#include <normal_fragment_maps>', '#include <normal_fragment_maps>\nif (vCloth > 0.5) normal = clothBump(-vViewPosition, normal, clothH, 0.6);')
      .replace('#include <roughnessmap_fragment>', '#include <roughnessmap_fragment>\nif (vCloth > 0.5) roughnessFactor = vCloth > 2.5 && vCloth < 3.5 ? 0.55 : 0.95;')
      .replace('#include <specularmap_fragment>', '#include <specularmap_fragment>\nif (vCloth > 0.5) specularStrength = vCloth > 2.5 && vCloth < 3.5 ? 0.5 : 0.04;');
  };
  material.customProgramCacheKey = () => `fabric-${scale}`;
  return material;
}

/**
 * A layer of clothing over a skinned mesh's `geometry` (indexed or not, in its bind
 * pose): the part of it between `from` and `to` along z, cut straight across there,
 * stood off it by `inflate` along its normals, in `color` and of fabric `kind` - with
 * a rim at each cut, from the layer's surface back down to the skin, which is what
 * makes a sleeve's end a sleeve's end. Skinned as the mesh is; at a cut, each new point
 * goes with the nearer of the two it was cut between.
 */
export function layer(geometry, { from = -Infinity, to = Infinity, inflate, color, kind }) {
  const position = geometry.getAttribute('position'), normal = geometry.getAttribute('normal');
  const skinIndex = geometry.getAttribute('skinIndex'), skinWeight = geometry.getAttribute('skinWeight');
  const index = geometry.index;
  const vertex = k => ({
    p: new THREE.Vector3().fromBufferAttribute(position, k), n: new THREE.Vector3().fromBufferAttribute(normal, k),
    i: [skinIndex.getX(k), skinIndex.getY(k), skinIndex.getZ(k), skinIndex.getW(k)],
    w: [skinWeight.getX(k), skinWeight.getY(k), skinWeight.getZ(k), skinWeight.getW(k)], cut: 0,
  });
  const between = (a, b, t, plane) => ({
    p: a.p.clone().lerp(b.p, t), n: a.n.clone().lerp(b.n, t).normalize(),
    i: t < 0.5 ? a.i : b.i, w: t < 0.5 ? a.w : b.w, cut: plane,
  });
  // Keeps the part of a polygon on the side of a plane where `side` is not negative.
  const clip = (poly, side, plane) => {
    const out = [];
    for (let k = 0; k < poly.length; k++) {
      const a = poly[k], b = poly[(k + 1) % poly.length], da = side(a), db = side(b);
      if (da >= 0) out.push(a);
      if ((da >= 0) !== (db >= 0)) out.push(between(a, b, da / (da - db), plane));
    }
    return out;
  };
  const p = [], n = [], si = [], sw = [];
  const put = (v, lift, normalOut = v.n) => {
    p.push(v.p.x + v.n.x * lift, v.p.y + v.n.y * lift, v.p.z + v.n.z * lift);
    n.push(normalOut.x, normalOut.y, normalOut.z);
    si.push(...v.i);
    sw.push(...v.w);
  };
  const ends = { 1: new THREE.Vector3(0, 0, -1), 2: new THREE.Vector3(0, 0, 1) };
  const count = index ? index.count : position.count;
  for (let t = 0; t < count; t += 3) {
    let poly = [0, 1, 2].map(k => vertex(index ? index.getX(t + k) : t + k));
    if (Number.isFinite(from)) poly = clip(poly, v => v.p.z - from, 1);
    if (Number.isFinite(to)) poly = clip(poly, v => to - v.p.z, 2);
    if (poly.length < 3) continue;
    for (let k = 1; k + 1 < poly.length; k++) for (const v of [poly[0], poly[k], poly[k + 1]]) put(v, inflate);
    // The rim, along each edge lying on a cut.
    for (let k = 0; k < poly.length; k++) {
      const a = poly[k], b = poly[(k + 1) % poly.length];
      if (!a.cut || a.cut !== b.cut) continue;
      const out = ends[a.cut];
      put(a, inflate, out); put(b, inflate, out); put(b, -0.0005, out);
      put(a, inflate, out); put(b, -0.0005, out); put(a, -0.0005, out);
    }
  }
  const out = new THREE.BufferGeometry();
  out.setAttribute('position', new THREE.Float32BufferAttribute(p, 3));
  out.setAttribute('normal', new THREE.Float32BufferAttribute(n, 3));
  out.setAttribute('skinIndex', new THREE.Uint16BufferAttribute(si, 4));
  out.setAttribute('skinWeight', new THREE.Float32BufferAttribute(sw, 4));
  const c = new THREE.Color(color), vertices = p.length / 3;
  out.setAttribute('color', new THREE.Float32BufferAttribute(Array.from({ length: vertices }, () => [c.r, c.g, c.b]).flat(), 3));
  out.setAttribute('cloth', new THREE.Float32BufferAttribute(new Array(vertices).fill(kind), 1));
  return out;
}
