// The walker's legs, as a model (scripts/legs.py) in the outfit of the map's style: a
// T-shirt, shorts and sneakers in a city, an electrician's coverall and boots on a
// board, a spacesuit in the galaxy. Seen looking down, sitting on a swing, sliding,
// kicking a ball.
//
// They stand in the walk scene at the walker's feet - where the planet's bend is
// nothing, so they are drawn flat - turned the way the walker faces, a little behind
// the eye as hips are, and posed from what the walker is doing. Unlit like the map; the
// fabric shades itself (cloth.js).
//
// Implements: REQ-WALK-059

import * as THREE from '../vendor/three.module.min.js';
import { GLTFLoader } from '../vendor/GLTFLoader.js';
import { clone as cloneRigged } from '../vendor/SkeletonUtils.js';
import { STATIC } from '../core/data.js';
import { EYE } from './walkbase.js';
import { fabric } from './cloth.js';

const HIP = 0.245;   // the hips over the feet, as the model has them
const SIT = 0.22;    // the eye over a seat (play.js)
const BEHIND = 0.03;  // the hips behind the eye
const KICK = 0.45;   // seconds a kick takes
const SOLE = 0.024;  // from the ankle down to the sole
const THIGH = 0.115, SHIN = 0.13; // hip to knee, and knee to sole
const SEATED = 1.5;  // how far forward a seated thigh is turned
const SEAT_UP = 0.03; // the hips over a seat: the thighs' thickness, sat on

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

// The model's geometry ready to draw: its normals, which the fabric is lit by
// (cloth.js), and which fabric each vertex is (_CLOTH, from scripts/legs.py).
const prepared = new Map();
function prepare(geometry) {
  if (prepared.has(geometry)) return prepared.get(geometry);
  const out = geometry.clone();
  out.computeVertexNormals();
  const kinds = out.getAttribute('_cloth');
  out.setAttribute('cloth', kinds || new THREE.Float32BufferAttribute(new Array(out.getAttribute('position').count).fill(0), 1));
  out.deleteAttribute('_cloth');
  prepared.set(geometry, out);
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
      o.geometry = prepare(o.geometry);
      // A weave every couple of centimeters: 180 to a unit of the map.
      o.material = fabric(new THREE.MeshBasicMaterial({ vertexColors: true }), 180, { lit: false });
      o.frustumCulled = false;
    }
    this.bones = new Map();
    rig.traverse(o => { if (o.isBone) this.bones.set(o.name, { bone: o, rest: o.quaternion.clone() }); });
    this.rig = rig;
    this.group.add(rig);
  }

  /**
   * Stands the legs under a walker this frame: `w` is the Walker. Hidden while there
   * is no walker out there to have legs. On their feet, the lower foot is on the
   * ground; sitting, the hips are on the seat and the feet clear of what is under it.
   */
  update(w, deltaTime) {
    const shown = w.active && !w.arrival && w.dying === null && !!this.rig;
    this.group.visible = shown;
    if (!shown) return;
    const p = w.p, sit = sitting(w);
    // Sitting, the eye is SIT over the seat and the hips on it.
    const eye = p.feet + EYE + (w.eyeShift || 0), seat = eye - SIT;
    const y = p.feet + (seat + SEAT_UP - HIP - p.feet) * sit;
    const room = seat - w.height(p.x, p.z, seat);
    // Down a chute, the legs out ahead along half its slope, as a rider holds them -
    // turned about the hips, which stay on the seat.
    const r = w.riding, slide = r?.entry.ride === 'slide' ? r : null;
    const slope = slide && r.s > r.marks.edge && r.s < r.marks.foot ? r.slopeAt(r.s) : 0;
    this.lean = (this.lean || 0) + (slope * 0.5 * sit - (this.lean || 0)) * Math.min(1, deltaTime * 8);
    const hipsAt = HIPS.set(p.x + Math.sin(p.yaw) * BEHIND, y + HIP, p.z + Math.cos(p.yaw) * BEHIND);
    this.tilt(this.lean, p.yaw, hipsAt);
    this.phase = (this.phase + deltaTime * 7.5 * Math.max(0.5, w.pace || 0)) % (Math.PI * 2);
    const pose = posture(w, this.phase, performance.now(), room);
    for (const name of ['thigh_L', 'thigh_R', 'shin_L', 'shin_R', 'foot_L', 'foot_R']) this.turn(name, pose[name] || 0);
    // Sat, no foot goes under what is beneath it - the chute ahead, the ground: the
    // legs are lifted about the hips until the lower one clears it.
    if (sit > 0) {
      this.rig.updateMatrixWorld(true);
      let short = 0, reach = 0;
      for (const name of ['foot_L', 'foot_R']) {
        const foot = this.bones.get(name)?.bone.getWorldPosition(AT);
        if (!foot) continue;
        const out = Math.hypot(foot.x - hipsAt.x, foot.z - hipsAt.z);
        let under = w.height(foot.x, foot.z, foot.y);
        if (slide) under = Math.max(under, slide.pointAt(Math.min(slide.marks.end, slide.s + out), ON).y);
        short = Math.max(short, under + SOLE - foot.y);
        reach = Math.max(reach, out);
      }
      if (short > 0 && reach > 0.05) {
        this.lean -= Math.asin(Math.min(1, short / reach));
        this.tilt(this.lean, p.yaw, hipsAt);
      }
    }
    // Standing or walking, the lower foot planted on the ground.
    if (p.ground && sit < 1) {
      this.rig.updateMatrixWorld(true);
      const low = Math.min(...['foot_L', 'foot_R'].map(n => this.bones.get(n)?.bone.getWorldPosition(AT).y ?? y + SOLE));
      this.group.position.y += (p.feet - (low - SOLE)) * (1 - sit);
    }
  }

  // Turns the legs `lean` forward and down about the hips, which are kept at `hips`.
  tilt(lean, yaw, hips) {
    this.group.rotation.set(-lean, yaw, 0, 'YXZ');
    const from = AT.set(0, HIP, 0).applyEuler(this.group.rotation);
    this.group.position.set(hips.x - from.x, hips.y - from.y, hips.z - from.z);
    this.group.updateMatrixWorld(true);
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

const X = new THREE.Vector3(1, 0, 0), TURN = new THREE.Quaternion(), AT = new THREE.Vector3(), HIPS = new THREE.Vector3(), ON = new THREE.Vector3();

/**
 * How far the walker is sat down, 0 to 1: on a swing, a seesaw or a rider once on it,
 * down a slide as far as it has them sitting, and getting up again after either.
 */
export function sitting(w) {
  const r = w.riding, ride = r?.entry.ride;
  if (ride === 'swing' || ride === 'rock') return r.blend * r.blend * (3 - 2 * r.blend);
  if (ride === 'slide') return r.sit;
  return r ? 0 : w.unseat || 0;
}

/**
 * How far forward each joint of the legs is bent, radians, for what the walker is
 * doing: sitting (feet kept `room` clear of what is under the seat), in the air,
 * climbing a ladder, walking at `phase` of a stride, and a kick taken lately laid over
 * any of them.
 */
export function posture(w, phase, now, room = Infinity) {
  const p = w.p, r = w.riding, ride = r?.entry.ride, pace = w.pace || 0, sit = sitting(w);
  const pose = {};
  if (!p.ground && !r && sit === 0) {
    Object.assign(pose, { thigh_L: 0.45, thigh_R: 0.25, shin_L: -0.8, shin_R: -0.55 });
  } else if (ride === 'slide' && r.s < r.marks.top) {
    // Climbing: a leg up a rung at a time.
    const up = Math.max(0, Math.sin(phase * 1.3));
    Object.assign(pose, { thigh_L: up * 1.1, thigh_R: (1 - up) * 1.1, shin_L: -up * 1.4, shin_R: -(1 - up) * 1.4 });
  } else {
    const stride = Math.sin(phase) * 0.42 * Math.min(1.3, pace) * (1 - sit);
    Object.assign(pose, {
      thigh_L: stride, thigh_R: -stride,
      shin_L: -Math.max(0, -Math.sin(phase + 0.9)) * 0.9 * Math.min(1.3, pace) * (1 - sit),
      shin_R: -Math.max(0, Math.sin(phase + 0.9)) * 0.9 * Math.min(1.3, pace) * (1 - sit),
    });
  }
  if (sit > 0) {
    // Thighs out along the seat; shins hanging - no lower than the room under the seat
    // allows - or out straight down a slide.
    let down = ride === 'slide' ? 0.1 : ride === 'swing' ? 1.25 + 0.4 * Math.sin(r.angle * 2) : 1.4;
    const reach = Math.max(-1, Math.min(1, (room - 0.01 - THIGH * Math.cos(SEATED)) / SHIN));
    down = Math.min(down, SEATED - Math.acos(reach));
    for (const [joint, to] of [['thigh_L', SEATED], ['thigh_R', SEATED], ['shin_L', -down], ['shin_R', -down]]) {
      pose[joint] = (pose[joint] || 0) + (to - (pose[joint] || 0)) * sit;
    }
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
