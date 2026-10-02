// The walker's legs, as a model (scripts/legs.py) in the outfit of the map's style: a
// T-shirt, shorts and sneakers in a city, an electrician's coverall and boots on a
// board, a spacesuit in the galaxy. Seen looking down, sitting on a swing, sliding,
// kicking a ball.
//
// They stand in the walk scene at the walker's feet - where the planet's bend is
// nothing, so they are drawn flat - turned the way the walker faces, a little behind
// the eye as hips are, and posed from what the walker is doing. Unlit like the map,
// with their shading baked in from their normals, as the props' is.
//
// Implements: REQ-WALK-059

import * as THREE from '../vendor/three.module.min.js';
import { GLTFLoader } from '../vendor/GLTFLoader.js';
import { clone as cloneRigged } from '../vendor/SkeletonUtils.js';
import { STATIC } from '../core/data.js';
import { EYE } from './walkbase.js';

const HIP = 0.245;   // the hips over the feet, as the model has them
const SIT = 0.22;    // the eye over a seat (play.js)
const BEHIND = 0.03;  // the hips behind the eye
const KICK = 0.45;   // seconds a kick takes

let model = null, loading = null;

/** Starts loading the model; resolves with it, or null if it will not load. */
export function loadLegs() {
  loading ||= new GLTFLoader().loadAsync(STATIC?.legs || 'legs.glb').then(gltf => {
    model = gltf.scene;
    model.updateMatrixWorld(true);
    return model;
  }).catch(err => {
    console.error('legs model:', err);
    return null;
  });
  return loading;
}

// A geometry with its shading baked into its colors: lighter facing up and towards the
// light, as city.js shades the props.
const baked = new Map();
function bake(geometry) {
  if (baked.has(geometry)) return baked.get(geometry);
  const out = geometry.clone();
  out.computeVertexNormals();
  const n = out.getAttribute('normal'), c = out.getAttribute('color');
  const colors = new Float32Array(c.count * 3);
  for (let i = 0; i < c.count; i++) {
    const k = 0.62 + 0.3 * Math.max(0, n.getY(i)) + 0.1 * n.getX(i) - 0.05 * n.getZ(i);
    colors.set([c.getX(i) * k, c.getY(i) * k, c.getZ(i) * k], i * 3);
  }
  out.setAttribute('color', new THREE.BufferAttribute(colors, 3));
  baked.set(geometry, out);
  return out;
}

export class Legs {
  constructor(scene) {
    this.scene = scene;
    this.group = new THREE.Group();
    this.group.visible = false;
    this.worn = null;   // the style the legs were dressed for
    this.rig = null;    // the clone being drawn
    this.bones = null;
    this.phase = 0;     // the stride
    scene.scene.add(this.group);
  }

  /** Dresses the legs for `style`, once the model is in. */
  wear(style) {
    if (this.worn === style && this.rig) return;
    if (!model) {
      loadLegs().then(m => { if (m) this.wear(style); });
      return;
    }
    this.worn = style;
    if (this.rig) this.group.remove(this.rig);
    const rig = cloneRigged(model);
    const wanted = `legs_${style}`, fallback = 'legs_city';
    const meshes = [];
    rig.traverse(o => { if (o.isSkinnedMesh) meshes.push(o); });
    const pick = meshes.find(o => o.name === wanted) || meshes.find(o => o.name === fallback);
    for (const o of meshes) {
      o.visible = o === pick;
      if (o !== pick) continue;
      o.geometry = bake(o.geometry);
      o.material = new THREE.MeshBasicMaterial({ vertexColors: true });
      o.frustumCulled = false;
    }
    this.bones = new Map();
    rig.traverse(o => { if (o.isBone) this.bones.set(o.name, { bone: o, rest: o.quaternion.clone() }); });
    this.rig = rig;
    this.group.add(rig);
  }

  /**
   * Stands the legs under a walker this frame: `w` is the Walker. Hidden while there
   * is no walker out there to have legs.
   */
  update(w, deltaTime) {
    const shown = w.active && !w.arrival && w.dying === null && !!this.rig;
    this.group.visible = shown;
    if (!shown) return;
    const p = w.p;
    // Sitting, the hips are on the seat, the eye SIT over it.
    const y = seated(w) ? p.feet + EYE - SIT - 0.01 - HIP : p.feet;
    this.group.position.set(p.x + Math.sin(p.yaw) * BEHIND, y, p.z + Math.cos(p.yaw) * BEHIND);
    this.group.rotation.set(0, p.yaw, 0);
    this.phase = (this.phase + deltaTime * 7.5 * Math.max(0.5, w.pace || 0)) % (Math.PI * 2);
    const pose = posture(w, this.phase, performance.now());
    for (const name of ['thigh_L', 'thigh_R', 'shin_L', 'shin_R', 'foot_L', 'foot_R']) this.turn(name, pose[name] || 0);
  }

  // Bends one joint forward by `angle` from its rest.
  turn(name, angle) {
    const joint = this.bones?.get(name);
    if (!joint) return;
    joint.bone.quaternion.copy(joint.rest).multiply(TURN.setFromAxisAngle(X, angle));
  }

  dispose() {
    this.group.removeFromParent();
  }
}

const X = new THREE.Vector3(1, 0, 0), TURN = new THREE.Quaternion();

/** Whether the walker is sitting: on a swing, a seesaw or a rider, or down a slide. */
const seated = w => {
  const r = w.riding, ride = r?.entry.ride;
  return ride === 'swing' || ride === 'rock' || (ride === 'slide' && r.stage >= r.entry.sit);
};

/**
 * How far forward each joint of the legs is bent, radians, for what the walker is
 * doing: sitting, in the air, climbing a ladder, walking at `phase` of a stride, and
 * a kick taken lately laid over any of them.
 */
export function posture(w, phase, now) {
  const p = w.p, r = w.riding, ride = r?.entry.ride, pace = w.pace || 0;
  const pose = {};
  if (seated(w)) {
    // Thighs out along the seat; shins hanging, or out straight down a slide.
    const down = ride === 'slide' ? 0.1 : ride === 'swing' ? 1.25 + 0.4 * Math.sin(r.angle * 2) : 1.4;
    Object.assign(pose, { thigh_L: 1.5, thigh_R: 1.5, shin_L: -down, shin_R: -down });
  } else if (!p.ground && !r) {
    Object.assign(pose, { thigh_L: 0.45, thigh_R: 0.25, shin_L: -0.8, shin_R: -0.55 });
  } else if (ride === 'slide' && r.stage === 0) {
    // Climbing: a leg up a rung at a time.
    const up = Math.max(0, Math.sin(phase * 1.3));
    Object.assign(pose, { thigh_L: up * 1.1, thigh_R: (1 - up) * 1.1, shin_L: -up * 1.4, shin_R: -(1 - up) * 1.4 });
  } else {
    const swing = Math.sin(phase) * 0.42 * Math.min(1.3, pace);
    Object.assign(pose, {
      thigh_L: swing, thigh_R: -swing,
      shin_L: -Math.max(0, -Math.sin(phase + 0.9)) * 0.9 * Math.min(1.3, pace),
      shin_R: -Math.max(0, Math.sin(phase + 0.9)) * 0.9 * Math.min(1.3, pace),
    });
  }
  // A kick: the right leg drawn back, then through and up.
  const since = (now - (w.kicked ?? -Infinity)) / 1000;
  if (since < KICK) {
    const u = since / KICK;
    pose.thigh_R = u < 0.35 ? -0.6 * (u / 0.35) : -0.6 + 1.9 * Math.min(1, (u - 0.35) / 0.3) - 1.3 * Math.max(0, (u - 0.65) / 0.35);
    pose.shin_R = u < 0.35 ? -1.2 * (u / 0.35) : -1.2 * Math.max(0, 1 - (u - 0.35) / 0.25);
  }
  return pose;
}
