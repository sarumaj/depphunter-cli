// What the walker wears for a tool that is not only held: the jet backpack on the back,
// the parachute's container on its harness, and a skimmer's float under each foot.
// The view sees only the part of each that reaches round into it (tools.js); seen from
// behind, or looking down at the feet, the body wears the rest (body.js).
//
// Built in the body's own frame as it stands at rest - feet at 0, facing -z, its units
// the body's, an eye 0.45 over the ground - for body.js to hang on its bones.
//
// Implements: REQ-WALK-061

import * as THREE from '../vendor/three.module.min.js';
import { litPart } from './tools.js';

// The back of the chest, and where the straps go over the shoulders.
const BACK = 0.026, CHEST = 0.035, SHOULDER_TOP = 0.39, STRAP_X = 0.03;

const tube = (r0, r1, height, color, at, segments = 12) =>
  place(litPart(new THREE.CylinderGeometry(r0, r1, height, segments), color), at);
const box = (w, h, d, color, at) => place(litPart(new THREE.BoxGeometry(w, h, d), color), at);

function place(mesh, [x, y, z]) {
  mesh.position.set(x, y, z);
  return mesh;
}

// A strap from one point to another, as a flat band.
function band(from, to, width, color) {
  const run = new THREE.Vector3().subVectors(to, from);
  const mesh = litPart(new THREE.BoxGeometry(width, run.length(), 0.003), color);
  mesh.position.copy(from).addScaledVector(run, 0.5);
  mesh.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), run.normalize());
  return mesh;
}

// The harness both packs hang on: a strap over each shoulder from the top of the pack
// down the front of the chest, and a belt round the waist.
function harness(top, bottom, color) {
  const g = new THREE.Group();
  for (const side of [-1, 1]) {
    const x = side * STRAP_X;
    const over = new THREE.Vector3(x, SHOULDER_TOP, 0);
    g.add(band(new THREE.Vector3(x, top, BACK + 0.004), over, 0.009, color));
    g.add(band(over, new THREE.Vector3(x * 1.1, 0.31, -CHEST - 0.003), 0.009, color));
    g.add(band(new THREE.Vector3(x * 1.1, 0.31, -CHEST - 0.003), new THREE.Vector3(x * 1.2, bottom, BACK + 0.002), 0.008, color));
  }
  return g;
}

/**
 * The jet backpack: two tanks side by side on a back plate, each necking down into a
 * bell, and the flame below each bell - `flames`, for update to lick and to show only
 * while it is flying.
 */
function jetPack() {
  const g = new THREE.Group();
  g.name = 'jetpack';
  g.add(box(0.064, 0.1, 0.008, '#3c444d', [0, 0.32, BACK + 0.004]));
  const flames = [];
  for (const side of [-1, 1]) {
    const x = side * 0.019, z = BACK + 0.024;
    g.add(tube(0.017, 0.017, 0.085, '#48525c', [x, 0.325, z]));
    g.add(place(litPart(new THREE.SphereGeometry(0.017, 12, 8).scale(1, 0.6, 1), '#5a6068'), [x, 0.3675, z]));
    for (const y of [0.345, 0.315]) g.add(tube(0.0175, 0.0175, 0.003, '#39424c', [x, y, z]));
    g.add(tube(0.017, 0.011, 0.014, '#3c444d', [x, 0.2755, z]));
    g.add(tube(0.011, 0.016, 0.02, '#23292f', [x, 0.2585, z]));
    const flame = new THREE.Group();
    flame.position.set(x, 0.248, z);
    const soft = { transparent: true, depthWrite: false };
    flame.add(litPart(new THREE.ConeGeometry(0.008, 0.035, 10).rotateX(Math.PI).translate(0, -0.0175, 0), '#ffffff', { ...soft, opacity: 0.85 }));
    flame.add(litPart(new THREE.ConeGeometry(0.015, 0.07, 12).rotateX(Math.PI).translate(0, -0.035, 0), '#7fc8f0', { ...soft, opacity: 0.55 }));
    flame.add(litPart(new THREE.CylinderGeometry(0.012, 0.035, 0.11, 12, 1, true).translate(0, -0.09, 0), '#bcd8ea',
      { ...soft, opacity: 0.15, side: THREE.DoubleSide }));
    g.add(flame);
    flames.push(flame);
  }
  g.add(harness(0.36, 0.27, '#2b3138'));
  g.userData.flames = flames;
  return g;
}

/**
 * The parachute's container on its harness: the tray, its closing flap - `flap`, which
 * hangs open once the canopy is out of it - the pilot chute's pouch at its bottom, and
 * `userData.risers`, where the four risers leave the harness on top of the shoulders.
 */
function container() {
  const g = new THREE.Group();
  g.name = 'container';
  g.add(box(0.07, 0.095, 0.03, '#2b3138', [0, 0.322, BACK + 0.016]));
  g.add(box(0.072, 0.004, 0.032, '#59636e', [0, 0.37, BACK + 0.016]));
  const flap = box(0.06, 0.035, 0.004, '#39424c', [0, -0.0175, 0]);
  const hinge = new THREE.Group();
  hinge.name = 'flap';
  hinge.position.set(0, 0.368, BACK + 0.033);
  hinge.add(flap);
  g.add(hinge);
  g.add(place(litPart(new THREE.CylinderGeometry(0.009, 0.009, 0.02, 10).rotateZ(Math.PI / 2), '#39424c'), [0.022, 0.28, BACK + 0.034]));
  g.add(harness(0.37, 0.27, '#4a3f35'));
  g.userData.risers = [-1, 1].flatMap(side => [-0.006, 0.006].map(dz => new THREE.Vector3(side * STRAP_X, SHOULDER_TOP + 0.004, dz)));
  return g;
}

/** A skimmer's float under the foot on `side` (-1 left, 1 right), the binding round the shoe. */
function float(side) {
  const g = new THREE.Group();
  g.name = 'float';
  const x = side * 0.03;
  const hull = litPart(new THREE.CapsuleGeometry(0.012, 0.085, 4, 10).rotateX(Math.PI / 2), '#e8641f');
  hull.scale.set(1.3, 0.75, 1);
  g.add(place(hull, [x, -0.003, -0.016]));
  g.add(box(0.026, 0.003, 0.085, '#f1efe8', [x, 0.006, -0.016]));
  g.add(place(litPart(new THREE.TorusGeometry(0.017, 0.003, 5, 14).rotateX(Math.PI / 2), '#2b3138'), [x, 0.016, -0.02]));
  g.add(box(0.003, 0.012, 0.02, '#2f353c', [x, -0.015, 0.025]));
  return g;
}

/** Everything worn, each at rest in the body's frame and hidden until it is worn. */
export function buildGear() {
  const gear = { jetpack: jetPack(), container: container(), floats: [float(-1), float(1)] };
  for (const part of [gear.jetpack, gear.container, ...gear.floats]) {
    part.visible = false;
    part.traverse(o => { o.frustumCulled = false; });
  }
  return gear;
}

// How far the floats raise the feet off what is under them.
export const FLOAT_LIFT = 0.012;

/** The jet's flames, licking - `burn` longer while the throttle is open - at `now`. */
export function lick(jetpack, burn, now) {
  const t = now / 1000;
  for (const [i, flame] of jetpack.userData.flames.entries()) {
    const k = 1 + Math.sin(t * 21 + i * 1.3) * 0.12 + Math.sin(t * 7.3 + i) * 0.06;
    flame.scale.set(1 + (k - 1) * 0.4, k * burn, 1 + (k - 1) * 0.4);
  }
}
