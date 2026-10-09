// What the props are made of: geometry with its shading baked into vertex colors, so
// the map's unlit materials still show which way a face is turned, and the handful of
// shapes the amenities are built from.

import * as THREE from '../vendor/three.module.min.js';

// Deterministic 0..1 from a position, so props stay put across relayouts.
export function rand(x, z) {
  const v = Math.sin(x * 12.9898 + z * 78.233) * 43758.5453;
  return v - Math.floor(v);
}

// Geometries with vertex colors as fixed shading: lighter facing up and towards the
// light, darker towards the base (self-shadowing), with a little per-vertex jitter so
// foliage does not look faceted. A `round` geometry keeps its own smooth normals, so a
// post or a bar shades round and not in facets.
// Implements: REQ-CITY-022
export function shaded(geo, jitter = 0, round = false) {
  geo = geo.index ? geo.toNonIndexed() : geo;
  if (!round) geo.computeVertexNormals();
  geo.computeBoundingBox();
  const { min, max } = geo.boundingBox;
  const n = geo.getAttribute('normal'), position = geo.getAttribute('position'), colors = [];
  for (let i = 0; i < n.count; i++) {
    const up = (position.getY(i) - min.y) / Math.max(1e-6, max.y - min.y);
    const k = (0.5 + 0.4 * Math.max(0, n.getY(i)) + 0.12 * n.getX(i) - 0.06 * n.getZ(i)) * (0.72 + 0.28 * up)
      * (1 + jitter * (rand(position.getX(i) * 7.1, position.getZ(i) * 5.3 + position.getY(i)) - 0.5));
    colors.push(k, k, k);
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  return geo;
}

// One non-indexed geometry from several (position and color only, as shaded makes them).
export function merge(geos) {
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

// ------------------------------------------------------------------ painted shapes

// Shaded, then tinted one color.
export const painted = (geo, hex, jitter = 0, round = false) => {
  geo = shaded(geo, jitter, round);
  const c = new THREE.Color(hex), colors = geo.getAttribute('color');
  for (let i = 0; i < colors.count; i++) colors.setXYZ(i, colors.getX(i) * c.r, colors.getY(i) * c.g, colors.getZ(i) * c.b);
  return geo;
};
// Flat on the ground: a patch of surface, a painted line, a painted ring. A patch laid
// on another (INLAY) lies between the two, not in the same plane flickering through it.
export const LIFT = 0.006, INLAY = 0.0075, PAINT = 0.009;
// Cut finely enough to follow the ground as the walk view bends it, and not lie under it.
const cuts = length => Math.max(1, Math.ceil(length / 0.1));
export const patch = (w, d, hex, x = 0, z = 0, y = LIFT) => painted(new THREE.PlaneGeometry(w, d, cuts(w), cuts(d)).rotateX(-Math.PI / 2).translate(x, y, z), hex);
export const stripe = (x0, z0, x1, z1, hex, width = 0.018) => {
  const len = Math.hypot(x1 - x0, z1 - z0);
  return painted(new THREE.PlaneGeometry(len, width, cuts(len), 1).rotateX(-Math.PI / 2).rotateY(-Math.atan2(z1 - z0, x1 - x0))
    .translate((x0 + x1) / 2, PAINT, (z0 + z1) / 2), hex);
};
export const outline = (w, d, hex, x = 0, z = 0, width) => merge([
  stripe(x - w / 2, z - d / 2, x + w / 2, z - d / 2, hex, width), stripe(x - w / 2, z + d / 2, x + w / 2, z + d / 2, hex, width),
  stripe(x - w / 2, z - d / 2, x - w / 2, z + d / 2, hex, width), stripe(x + w / 2, z - d / 2, x + w / 2, z + d / 2, hex, width),
]);
export const ring = (r, hex, x = 0, z = 0, start = 0, sweep = Math.PI * 2, width = 0.018) => painted(
  new THREE.RingGeometry(r - width / 2, r + width / 2, 28, 1, start, sweep).rotateX(-Math.PI / 2).translate(x, PAINT, z), hex);
// Standing: a post, a bar between two points, a block.
export const post = (x, z, h, r, hex, y = 0) => painted(new THREE.CylinderGeometry(r, r, h, 10).translate(x, y + h / 2, z), hex, 0, true);
export const bar = (a, b, r, hex) => {
  const from = new THREE.Vector3(...a), to = new THREE.Vector3(...b), len = from.distanceTo(to);
  const geo = new THREE.CylinderGeometry(r, r, len, 8);
  geo.applyQuaternion(new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 1, 0), to.clone().sub(from).normalize()));
  return painted(geo.translate((a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2), hex, 0, true);
};
export const block = (w, h, d, x, y, z, hex) => painted(new THREE.BoxGeometry(w, h, d).translate(x, y + h / 2, z), hex);
// The halo round whatever on one lights itself, as an LED's (GLOW): a shell at each
// point, for each [radius, strength].
export const lit = (points, shells) => shells.map(([r, opacity]) => [
  merge(points.map(([x, y, z]) => shaded(new THREE.SphereGeometry(r, 9, 6).translate(x, y, z)))), opacity]);

// ------------------------------------------------------------------ contact shadows

// Where something stands on the ground, the ground is darker - most under its middle,
// fading out to nothing at its rim. There are no lights in the map and so no shadow
// maps (cityglsl.js): a shadow is a disc multiplied over whatever it lies on, white
// leaving it be and gray darkening it, and over every painted patch, line and ring.
export const SHADOW_LIFT = 0.011;
// How dark a shadow is under its middle, as what the ground is multiplied by.
export const SHADOW_DARK = 0.55;

/**
 * A shadow on the ground: a disc `rx` by `rz` across its middle at (x, z), of rings
 * whose vertex colors fall off from `dark` in the middle to white at the rim - in
 * linear color, which the output turns back into `dark` on screen.
 */
export function shadowSpot(rx = 1, rz = rx, x = 0, z = 0, dark = SHADOW_DARK) {
  const RINGS = [[0, 1], [0.3, 0.95], [0.55, 0.75], [0.78, 0.38], [1, 0]], SEGMENTS = 24;
  const at = (ring, k) => {
    const [r] = RINGS[ring], a = k / SEGMENTS * Math.PI * 2;
    return [x + Math.cos(a) * r * rx, SHADOW_LIFT, z + Math.sin(a) * r * rz];
  };
  const shade = ring => Math.pow(1 - (1 - dark) * RINGS[ring][1], 2.2);
  const positions = [], colors = [];
  const vertex = (ring, k) => {
    positions.push(...at(ring, k));
    const c = shade(ring);
    colors.push(c, c, c);
  };
  for (let ring = 0; ring + 1 < RINGS.length; ring++) {
    for (let k = 0; k < SEGMENTS; k++) {
      vertex(ring, k); vertex(ring + 1, k + 1); vertex(ring + 1, k);
      if (ring) { vertex(ring, k); vertex(ring, k + 1); vertex(ring + 1, k + 1); }
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  return geo;
}

/**
 * Makes `material` (bendable or not, with vertex colors) draw a shadow: multiplied over
 * what is drawn already, writing no depth, and drawn over the surface it lies in. Fog
 * fades it to white - to nothing - rather than to the fog's color, which multiplied
 * over the ground would tint it.
 */
export function shadowMaterial(material) {
  const before = material.onBeforeCompile, key = material.customProgramCacheKey;
  material.onBeforeCompile = (shader, renderer) => {
    before?.call(material, shader, renderer);
    shader.fragmentShader = shader.fragmentShader.replace('#include <fog_fragment>', `
#ifdef USE_FOG
  #ifdef FOG_EXP2
    float fogFactor = 1.0 - exp(-fogDensity * fogDensity * vFogDepth * vFogDepth);
  #else
    float fogFactor = smoothstep(fogNear, fogFar, vFogDepth);
  #endif
  gl_FragColor.rgb = mix(gl_FragColor.rgb, vec3(1.0), fogFactor);
#endif`);
  };
  material.customProgramCacheKey = () => `${key.call(material)}-shadow`;
  return Object.assign(material, {
    transparent: true, depthWrite: false, blending: THREE.MultiplyBlending, side: THREE.DoubleSide,
    polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2,
  });
}
