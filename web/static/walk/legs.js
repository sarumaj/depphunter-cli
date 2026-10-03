// The walker's body - legs, torso, arms and a head - as a model (scripts/legs.py) in
// the outfit of the map's style: a T-shirt, shorts and sneakers in a city, an
// electrician's coverall, gloves, boots and hard hat on a board, a spacesuit in the
// galaxy. Seen looking down, sitting on a swing, sliding, kicking a ball.
//
// One pair of hands at a time: while a tool or a ball is held before the eye (walk.js)
// the body's arms are folded away, and they are the hands seen otherwise - empty-handed,
// or looking down past what is held. The head is the eye's, so it is drawn only for a
// view from outside it: seen from behind (walk.js thirdPerson), where the body holds
// what is in hand - a tool aimed along the view, a ball carried before the chest.
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
const STOOP = 0.035;  // ... and further behind it looking straight down: the head bent forward over the chest
const SOLE = 0.024;  // from the ankle down to the sole
const THIGH = 0.115, SHIN = 0.13; // hip to knee, and knee to sole
const SEATED = 1.5;  // how far forward a seated thigh is turned
const STRADDLE = 0.75; // how far a thigh is turned out astride a car
const SEAT_UP = 0.03; // the hips over a seat: the thighs' thickness, sat on

/** Seconds into a kick that the foot meets the ball. */
export const CONTACT = 0.26;

// A kick, as keys through which it is eased: seconds in; the right thigh, shin and
// foot; the left thigh and shin, the planted leg giving at the knee. Drawn back with
// the heel up, whipped through at the knee into the ball, swung on up and let down.
const KICK_KEYS = [
  [0, 0, 0, 0, 0, 0],
  [0.12, -0.55, -1.5, -0.3, 0.12, -0.3],
  [0.21, -0.15, -1.35, -0.5, 0.16, -0.36],
  [CONTACT, 0.3, -0.4, -0.55, 0.16, -0.36],
  [0.36, 1.05, -0.1, -0.4, 0.12, -0.3],
  [0.46, 0.85, -0.45, -0.1, 0.08, -0.2],
  [0.66, 0, 0, 0, 0, 0],
];
const KICK_JOINTS = ['thigh_R', 'shin_R', 'foot_R', 'thigh_L', 'shin_L'];

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
    this.showHead = false; // the head, for a view from outside the walker's own eye
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
    // Sat, the body faces the seat's way, not where the walker looks.
    const look = w.rideYaw?.() ?? p.yaw, yaw = p.yaw + Math.atan2(Math.sin(look - p.yaw), Math.cos(look - p.yaw)) * sit;
    const r = w.riding, slide = r?.entry.ride === 'slide' ? r : null;
    const slope = slide && r.s > r.marks.edge && r.s < r.marks.foot ? r.slopeAt(r.s) : 0;
    this.lean = (this.lean || 0) + (slope * 0.5 * sit - (this.lean || 0)) * Math.min(1, deltaTime * 8);
    // Looking down from the eye, the head is bent over the chest; seen from behind, it is not.
    const behind = BEHIND + (w.thirdPerson ? 0 : STOOP * Math.min(1, Math.max(0, -p.pitch) / (Math.PI / 2)));
    const hipsAt = HIPS.set(p.x + Math.sin(yaw) * behind, y + HIP, p.z + Math.cos(yaw) * behind);
    this.tilt(this.lean, yaw, hipsAt);
    this.phase = (this.phase + deltaTime * 7.5 * Math.max(0.5, w.pace || 0)) % (Math.PI * 2);
    const pose = posture(w, this.phase, performance.now(), room);
    for (const name of ['thigh_L', 'thigh_R', 'shin_L', 'shin_R', 'foot_L', 'foot_R']) this.turn(name, pose[name] || 0, pose[`${name}_out`] || 0);
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
        this.tilt(this.lean, yaw, hipsAt);
      }
    }
    // The legs lean about the hips; the torso stays upright under the eye, turning a
    // little against the stride as a walker's shoulders do.
    this.turn('spine', this.lean, (pose.twist || 0));
    // The arms: as they swing or hold on - or folded away, while the hands held before
    // the eye are the ones seen.
    const arms = w.bodyArms?.() ?? true;
    for (const side of ['L', 'R']) {
      const out = side === 'L' ? -1 : 1;
      for (const part of ['upperarm', 'forearm', 'hand']) {
        const [angle, spread] = pose[`${part}_${side}`] || [0, 0];
        this.turn(`${part}_${side}`, angle, 0, spread * out);
      }
      this.bones.get(`upperarm_${side}`)?.bone.scale.setScalar(arms ? 1 : 1e-4);
    }
    // The head looks where the eye does, as far as a neck turns.
    const glance = Math.max(-1.4, Math.min(1.4, Math.atan2(Math.sin(p.yaw - yaw), Math.cos(p.yaw - yaw))));
    this.turn('head', Math.max(-0.9, Math.min(0.7, p.pitch)), glance);
    this.bones.get('head')?.bone.scale.setScalar(this.showHead || w.thirdPerson ? 1 : 1e-4);
    // A ball carried seen from behind is between the hands.
    if (w.thirdPerson && w.ballHeld && arms) {
      this.rig.updateMatrixWorld(true);
      const left = this.bones.get('hand_L')?.bone.getWorldPosition(AT), right = this.bones.get('hand_R')?.bone.getWorldPosition(ON);
      if (left && right) w.ballHeld.mesh.position.addVectors(left, right).multiplyScalar(0.5).add(HELD.set(-Math.sin(yaw), 0, -Math.cos(yaw)).multiplyScalar(0.03));
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

  // Bends one joint forward by `angle` from its rest, turns it `out` about its own
  // length - a thigh turned out, its knee bent, straddles - and swings it `spread` to
  // the side.
  turn(name, angle, out = 0, spread = 0) {
    const joint = this.bones?.get(name);
    if (!joint) return;
    joint.bone.quaternion.copy(joint.rest).multiply(OUT.setFromAxisAngle(Y, out))
      .multiply(SIDE.setFromAxisAngle(Z, spread)).multiply(TURN.setFromAxisAngle(X, angle));
  }

  dispose() {
    this.group.removeFromParent();
  }
}

const X = new THREE.Vector3(1, 0, 0), Y = new THREE.Vector3(0, 1, 0), Z = new THREE.Vector3(0, 0, 1);
const TURN = new THREE.Quaternion(), OUT = new THREE.Quaternion(), SIDE = new THREE.Quaternion(), AT = new THREE.Vector3(), HIPS = new THREE.Vector3(), ON = new THREE.Vector3();
const HELD = new THREE.Vector3();

/**
 * How far the walker is sat down, 0 to 1: on a swing, a seesaw, a rider or a car once on it,
 * down a slide as far as it has them sitting, and getting up again after either.
 */
export function sitting(w) {
  const r = w.riding, ride = r?.entry.ride;
  if (ride === 'swing' || ride === 'rock' || ride === 'drive') return r.blend * r.blend * (3 - 2 * r.blend);
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
    let down = ride === 'slide' ? 0.1 : ride === 'swing' ? 1.25 + 0.4 * Math.sin(r.angle * 2) : ride === 'drive' ? 1.2 : 1.4;
    const reach = Math.max(-1, Math.min(1, (room - 0.01 - THIGH * Math.cos(SEATED)) / SHIN));
    down = Math.min(down, SEATED - Math.acos(reach));
    for (const [joint, to] of [['thigh_L', SEATED], ['thigh_R', SEATED], ['shin_L', -down], ['shin_R', -down]]) {
      pose[joint] = (pose[joint] || 0) + (to - (pose[joint] || 0)) * sit;
    }
    // Astride a car, the thighs turned out round its body and the feet down beside it.
    if (ride === 'drive') Object.assign(pose, { thigh_L_out: STRADDLE * sit, thigh_R_out: -STRADDLE * sit });
  }
  const since = (now - (w.kicked ?? -Infinity)) / 1000;
  const kicking = since >= 0 && since < KICK_KEYS.at(-1)[0];
  if (kicking) Object.assign(pose, kickPose(since));
  arms(pose, w, phase, sit, kicking ? Math.sin(Math.PI * since / KICK_KEYS.at(-1)[0]) : 0);
  return pose;
}

// How each arm is held, ride by ride: the upper arm forward, the forearm bent up from the
// elbow and the arm out from the side - holding a swing's chains, the handles of a
// seesaw or a rider, a car's wheel, a roundabout's rail, the sides of a chute.
const REST_ARM = [0.3, 0.55, 0.1];
const HOLDS = {
  swing: [0.35, 1.7, 0.12],
  rock: [0.8, 0.9, -0.05],
  drive: [1.0, 0.6, -0.1],
  spin: [0.85, 0.9, 0],
  slide: [-0.15, 0.2, 0.45],
};

/**
 * The arms, into `pose` as [forward, side] for each joint, and the shoulders' twist:
 * swinging against the legs as the walker walks - further, and bent at the elbow, as
 * they run - out for balance in the air, up a ladder hand over hand, holding on to a
 * ride as far as they are sat on it (`sit`), and one thrown forward and the other back
 * through a kick (`kick`, 0 to 1 and back).
 */
function arms(pose, w, phase, sit, kick) {
  const p = w.p, r = w.riding, ride = r?.entry.ride, pace = Math.min(1.6, w.pace || 0);
  const set = (side, forward, bend, out = 0.06) => {
    pose[`upperarm_${side}`] = [forward, out];
    pose[`forearm_${side}`] = [bend, 0];
    pose[`hand_${side}`] = [0, 0];
  };
  // ... or eased `k` of the way from how it is now.
  const towards = (side, forward, bend, out, k) => {
    const [f, o] = pose[`upperarm_${side}`], [b] = pose[`forearm_${side}`];
    set(side, f + (forward - f) * k, b + (bend - b) * k, o + (out - o) * k);
  };
  const swing = Math.sin(phase) * 0.45 * Math.min(1.3, pace) * (1 - sit), running = Math.max(0, pace - 1.1) * 1.8;
  // At rest a little forward and bent at the elbow, the hands before the thighs.
  set('L', REST_ARM[0] - swing, REST_ARM[1] + Math.max(0, -swing) * 0.6 + running, REST_ARM[2]);
  set('R', REST_ARM[0] + swing, REST_ARM[1] + Math.max(0, swing) * 0.6 + running, REST_ARM[2]);
  pose.twist = -swing * 0.25;
  if (!p.ground && !r && sit === 0) {
    set('L', 0.35, 0.5, 0.5);
    set('R', 0.35, 0.5, 0.5);
  } else if (ride === 'slide' && r.s < r.marks.top) {
    const up = Math.sin(phase * 1.3);
    set('L', 2.3 + up * 0.3, 0.5);
    set('R', 2.3 - up * 0.3, 0.5);
  }
  const hold = HOLDS[ride], k = ride === 'spin' ? 1 : sit;
  if (hold && k > 0) {
    for (const side of ['L', 'R']) towards(side, ...hold, k);
    pose.twist *= 1 - k;
  }
  if (kick > 0) {
    // Against the right leg: the left arm forward and up, the right one back and out.
    towards('L', 0.9, 0.6, 0.3, kick);
    towards('R', -0.6, 0.3, 0.6, kick);
  }
  // Seen from behind, the body holds what is in hand: a ball before the chest in both,
  // a tool aimed along the view - the off hand's too.
  if (w.thirdPerson && !r) {
    const aim = Math.PI / 2 + Math.max(-0.8, Math.min(1, p.pitch));
    if (w.ballHeld) {
      towards('L', 1.0, 1.1, -0.25, 1);
      towards('R', 1.0, 1.1, -0.25, 1);
    } else {
      if (!w.bare) towards('R', aim, 0.15, -0.15, 1);
      if (w.secondary) towards('L', aim - 0.15, 0.3, -0.1, 1);
    }
  }
}

// The kick `since` seconds in, eased through its keys as a Catmull-Rom spline: no
// stop at any of them.
function kickPose(since) {
  let i = 1;
  while (KICK_KEYS[i][0] < since) i++;
  const k0 = KICK_KEYS[Math.max(0, i - 2)], k1 = KICK_KEYS[i - 1], k2 = KICK_KEYS[i], k3 = KICK_KEYS[Math.min(KICK_KEYS.length - 1, i + 1)];
  const t = (since - k1[0]) / (k2[0] - k1[0]), t2 = t * t, t3 = t2 * t;
  const pose = {};
  KICK_JOINTS.forEach((joint, j) => {
    const [a, b, c, d] = [k0[j + 1], k1[j + 1], k2[j + 1], k3[j + 1]];
    pose[joint] = 0.5 * (2 * b + (c - a) * t + (2 * a - 5 * b + 4 * c - d) * t2 + (3 * b - a - 3 * c + d) * t3);
  });
  return pose;
}
