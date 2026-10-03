// The walker's body - legs, torso, arms and a head - as a model (scripts/body.py,
// body.glb) in the outfit of the map's style: a T-shirt, shorts and sneakers in a city,
// an electrician's coverall, gloves, boots and hard hat on a board, a spacesuit in the
// galaxy. Seen looking down, sitting on a swing, sliding, kicking a ball.
//
// Its hands are the hand held before the eye (hands.js), at the body's scale, and close
// as that one does. One pair of hands at a time: while a tool or a ball is held before
// the eye (walk.js) the body's arms are folded away, and they are the hands seen
// otherwise - empty-handed, or looking down past what is held, which they then hold, as
// they do seen from behind (walk.js bodyHolds): a tool where the view holds it, a ball
// carried before the chest. The head is the eye's, so it is drawn only for a view from
// outside it (walk.js thirdPerson).
//
// They stand in the walk scene at the walker's feet - where the planet's bend is
// nothing, so they are drawn flat - turned the way the walker faces, a little behind
// the eye as hips are, and posed from what the walker is doing. Unlike the map it is
// lit, by lights of its own as the hand held before the eye is, in the same material.
//
// Implements: REQ-WALK-059

import * as THREE from '../vendor/three.module.min.js';
import { clone as cloneRigged } from '../vendor/SkeletonUtils.js';
import { loadBody } from './bodymodel.js';
import { closeHand, closeFinger } from './hands.js';
import { viewLights, skinMaterial } from './tools.js';
import { EYE } from './walkbase.js';

// The body as scripts/body.py builds it, on MakeHuman's joints (scripts/human.py).
const HIP = 0.256;   // the hips over the feet
const SIT = 0.22;    // the eye over a seat (play.js)
const BEHIND = 0.037;  // the hips behind the eye
const STOOP = 0.035;  // ... and further behind it looking straight down: the head bent forward over the chest
const SOLE = 0.02;   // from the ankle down to the sole
const THIGH = 0.115, SHIN = 0.14; // hip to knee, and knee to sole
const SEATED = 1.5;  // how far forward a seated thigh is turned
const STRADDLE = 0.75; // how far a thigh is turned out astride a car, a seesaw's plank or a rider
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

let model = null;

// The model's geometry ready to draw: which fabric each vertex is (_CLOTH, from
// scripts/body.py), as cloth.js reads it. Its normals are the file's, smooth across
// where one color meets another and the vertices are split.
const prepared = new Map();
function prepare(geometry) {
  if (prepared.has(geometry)) return prepared.get(geometry);
  const out = geometry.clone();
  if (!out.getAttribute('normal')) out.computeVertexNormals();
  const kinds = out.getAttribute('_cloth');
  out.setAttribute('cloth', kinds || new THREE.Float32BufferAttribute(new Array(out.getAttribute('position').count).fill(0), 1));
  out.deleteAttribute('_cloth');
  prepared.set(geometry, out);
  return out;
}

export class Body {
  constructor(scene) {
    this.scene = scene;
    this.group = new THREE.Group();
    this.group.visible = false;
    this.worn = null;   // the style the body was dressed for
    this.rig = null;    // the clone being drawn
    this.bones = null;
    this.phase = 0;     // the stride
    this.showHead = false; // the head, for a view from outside the walker's own eye
    this.carried = { L: null, R: null }; // what each hand holds seen from behind
    this.eased = { L: null, R: null };   // where each hand is holding on to a ride
    // Lit as the hand held before the eye is, by the same lights from where the view
    // is (update): nothing else in the map is lit, so these reach only the body and a
    // tool carried in its hand.
    this.lights = viewLights();
    this.lights.visible = false;
    scene.scene.add(this.group, this.lights);
  }

  /** Dresses the body for `style`, once the model is in. */
  wear(style) {
    if (this.worn === style && this.rig) return;
    if (!model) {
      loadBody().then(scene => { if ((model = scene?.getObjectByName('body_rig') || null)) this.wear(style); });
      return;
    }
    this.worn = style;
    if (this.rig) this.group.remove(this.rig);
    const rig = cloneRigged(model);
    const wanted = `body_${style}`, fallback = 'body_city';
    const meshes = [];
    rig.traverse(o => { if (o.isSkinnedMesh) meshes.push(o); });
    const pick = meshes.find(o => o.name === wanted) || meshes.find(o => o.name === fallback);
    for (const o of meshes) {
      o.visible = o === pick;
      if (o !== pick) continue;
      o.geometry = prepare(o.geometry);
      // Skin and cloth as the hand held before the eye has them, lit as it is - the
      // weave every couple of centimeters, coarser than the hand's, as it is seen
      // further off.
      o.material = skinMaterial(BODY_WEAVE);
      o.material.vertexColors = true;
      o.material.color.set('#ffffff');
      o.material.side = THREE.FrontSide;
      o.frustumCulled = false;
    }
    this.bones = new Map();
    rig.traverse(o => { if (o.isBone) this.bones.set(o.name, { bone: o, rest: o.quaternion.clone() }); });
    // Each hand's bones by the hand model's own names, for hands.js to close.
    this.hands = {};
    for (const side of ['L', 'R']) {
      const bones = new Map();
      for (const [name, joint] of this.bones) {
        if (!name.endsWith(`_${side}`) || !/finger|thumb|^hand_/.test(name)) continue;
        bones.set(name === `hand_${side}` ? 'wrist' : name.slice(0, -2), joint);
      }
      this.hands[side] = { userData: { bones } };
    }
    this.carried = { L: null, R: null };
    this.rig = rig;
    this.group.add(rig);
  }

  /**
   * Stands the body under a walker this frame: `w` is the Walker. Hidden while there
   * is no walker out there to have one. On their feet, the lower foot is on the
   * ground; sitting, the hips are on the seat and the feet clear of what is under it.
   */
  update(w, deltaTime) {
    const shown = w.active && !w.arrival && w.dying === null && !!this.rig;
    this.group.visible = this.lights.visible = shown;
    if (!shown) return;
    const view = this.scene.walkCamera;
    view.updateWorldMatrix(true, false);
    view.matrixWorld.decompose(this.lights.position, this.lights.quaternion, AT);
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
    // Running, the strides come quicker as well as longer.
    this.phase = (this.phase + deltaTime * 7.5 * Math.max(0.5, w.pace || 0) * (1 + 0.15 * running(w))) % (Math.PI * 2);
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
    // The legs lean about the hips; the torso stays upright under the eye - but for a
    // runner's lean - turning a little against the stride as a walker's shoulders do.
    this.turn('spine', this.lean + (pose.lean || 0), (pose.twist || 0));
    // The arms: as they swing or hold on - or folded away, while the hands held before
    // the eye are the ones seen.
    const arms = w.bodyArms?.() ?? true;
    for (const side of ['L', 'R']) {
      const out = side === 'L' ? 1 : -1;
      for (const part of ['upperarm', 'forearm', 'hand']) {
        const [angle, spread] = pose[`${part}_${side}`] || [0, 0];
        this.turn(`${part}_${side}`, angle, 0, spread * out);
      }
      this.bones.get(`upperarm_${side}`)?.bone.scale.setScalar(arms ? 1 : 1e-4);
    }
    // On a ride, the hands on what it has to hold on to (play.js rideGrips): as far as
    // the walker is sat on it, or as far as they have got to it on a roundabout, which is
    // stood on, and up a ladder, which is climbed.
    const climbing = r?.entry.ride === 'slide' && r.s < r.marks.top;
    const holding = r?.entry.ride === 'spin' || climbing ? r.blend : sit, grips = holding > 0 && w.rideGrips?.();
    for (const [i, side] of ['L', 'R'].entries()) {
      const grip = grips?.[i];
      if (!grip) {
        this.eased[side] = null;
        continue;
      }
      // With what it holds as that moves - a seesaw going up, a swing swinging - and
      // eased to a new hold, the next rung up, rather than jumping there.
      const held = (this.eased[side] ||= { at: grip.at.clone(), was: grip.at.clone() });
      const moved = MOVED.subVectors(grip.at, held.was);
      if (moved.length() < NEW_HOLD) held.at.add(moved);
      held.was.copy(grip.at);
      held.at.lerp(grip.at, Math.min(1, deltaTime * 14));
      this.grip(side, held.at, grip.along, holding);
    }
    // Seen from behind, the tools are in the body's hands, which close round them;
    // otherwise the hands are as the pose has them.
    const holds = w.bodyHolds && !w.riding && !w.ballHeld;
    this.carry('R', holds ? w.viewmodel : null);
    this.carry('L', holds ? w.offhand : null);
    for (const side of ['L', 'R']) {
      const held = this.carried[side];
      if (held) this.reach(side, held, w);
      closeHand(this.hands[side], held ? held.grip : pose[`grip_${side}`] ?? RELAXED);
      if (held?.trigger != null) closeFinger(this.hands[side], 0, held.trigger);
    }
    // The head looks where the eye does, as far as a neck turns.
    const glance = Math.max(-1.4, Math.min(1.4, Math.atan2(Math.sin(p.yaw - yaw), Math.cos(p.yaw - yaw))));
    // ... level however the body leans running.
    this.turn('head', Math.max(-0.9, Math.min(0.7, p.pitch)) + (pose.lean || 0), glance);
    this.bones.get('head')?.bone.scale.setScalar(this.showHead || w.thirdPerson ? 1 : 1e-4);
    // A ball the body carries is between the hands.
    if (w.bodyHolds && w.ballHeld && arms) {
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

  /**
   * Lays a copy of the tool `vm` - a viewmodel, built for the view (tools.js) - in the
   * hand on `side`, held as the viewmodel's own hand holds it, without that hand: the
   * body's hand is the same model, scaled to the body. Nothing (null) empties the hand.
   */
  carry(side, vm) {
    const now = this.carried[side];
    if (now?.vm === vm) return;
    const holder = vm?.userData.hands?.[0], hand = holder?.userData.hand;
    const bone = this.bones?.get(`hand_${side}`)?.bone;
    // The viewmodel's hand comes in with the model; until then, nothing to copy.
    if (vm && (!hand || !bone)) return;
    now?.copy.removeFromParent();
    this.carried[side] = null;
    if (!vm) return;
    // The tool where the viewmodel's hand has it, in the hand model's own space...
    vm.updateWorldMatrix(true, true);
    const inHand = new THREE.Matrix4().copy(hand.matrixWorld).invert().multiply(vm.matrixWorld);
    // ... copied without any of the viewmodel's hands ...
    const hands = (vm.userData.hands || []).map(h => [h, h.userData.hand]).filter(([, h]) => h);
    for (const [, h] of hands) h.removeFromParent();
    const copy = vm.clone();
    for (const [holding, h] of hands) holding.add(h);
    // ... and laid in the body's hand, mirrored for the left.
    copy.matrixAutoUpdate = false;
    copy.matrix.copy(side === 'L' ? MIRROR : IDENTITY).multiply(handToBone()).multiply(inHand);
    copy.traverse(o => { o.frustumCulled = false; o.renderOrder = 0; });
    bone.add(copy);
    const rest = holder.userData.restGrip ?? vm.userData.restGrip ?? 0.75;
    this.carried[side] = { vm, copy, grip: rest, trigger: vm.userData.trigger };
  }

  /**
   * Puts the hand on `side` where the view's hand holding `held` is: as far from the
   * eye, turned the same way to it, at the body's scale - the arm reaching it with the
   * elbow down and out, as far as it reaches.
   */
  reach(side, held, w) {
    const hand = held.vm.userData.hands[0].userData.hand, view = w.held;
    const fore = this.bones.get(`forearm_${side}`)?.bone, wrist = this.bones.get(`hand_${side}`)?.bone;
    if (!view || !fore || !wrist) return;
    // The view's hand from the held group, which is the eye's, in the hand model's units...
    hand.updateWorldMatrix(true, false);
    GOAL.copy(view.matrixWorld).invert().multiply(hand.matrixWorld);
    // ... before the walker's own eye, at the body's scale: where the hand bone goes.
    const p = w.p, rest = handRest(), s = handScale;
    EYE_AT.set(p.x, p.feet + EYE + (w.eyeShift || 0), p.z);
    // Looking down, the hands stay before the chest, where the eye looks down at them.
    EYE_TURN.setFromEuler(LOOK.set(Math.max(p.pitch, HOLD_PITCH), p.yaw, 0, 'YXZ'));
    BONE_GOAL.compose(EYE_AT, EYE_TURN, ONE).multiply(SCALED.makeScale(s, s, s)).multiply(GOAL)
      .multiply(rest).multiply(SCALED.makeScale(1 / s, 1 / s, 1 / s));
    if (side === 'L') BONE_GOAL.multiply(MIRROR);
    BONE_GOAL.decompose(TARGET, TURNED, SIZE);
    // The view's arm is longer than the body's, reaching from past the corner of the
    // eye: the body holds a tool that way from the shoulder, a little lower, the elbow
    // bent.
    const upper = this.bones.get(`upperarm_${side}`).bone;
    this.rig.updateMatrixWorld(true);
    const shoulder = upper.getWorldPosition(SHOULDER), reach = shoulder.distanceTo(fore.getWorldPosition(ELBOW)) + ELBOW.distanceTo(wrist.getWorldPosition(AT));
    TARGET.sub(shoulder);
    TARGET.y -= HOLD_LOW * reach;
    TARGET.setLength(Math.min(TARGET.length(), HOLD_IN * reach)).add(shoulder);
    this.armTo(side, TARGET);
    // The hand turned as the view's is.
    fore.getWorldQuaternion(EYE_TURN);
    wrist.quaternion.copy(EYE_TURN.invert().multiply(TURNED));
    wrist.updateMatrixWorld(true);
  }

  /**
   * Holds on with the hand on `side`, `k` of the way from where the pose has it, to a
   * bar through `at` running `along`: the fist round it, the bar across the palm.
   */
  grip(side, at, along, k) {
    const wrist = this.bones.get(`hand_${side}`)?.bone, knuckle = this.bones.get(`middle-finger-phalanx-proximal_${side}`)?.bone;
    if (!wrist || !knuckle) return;
    this.rig.updateMatrixWorld(true);
    const posed = wrist.getWorldPosition(POSED);
    handToBone();
    // Twice over: where the fist is depends on how the hand is turned, and how it is
    // turned on where the arm has put it.
    for (let pass = 0; pass < 2; pass++) {
      // The fist's middle: at the knuckles, in toward the palm.
      wrist.getWorldQuaternion(TURNED);
      const fist = knuckle.getWorldPosition(FIST_AT).sub(wrist.getWorldPosition(AT))
        .addScaledVector(PALM.set(0, 0, -1).applyQuaternion(TURNED), PALM_DEPTH * handScale);
      this.armTo(side, GRIP_GOAL.subVectors(at, fist).sub(posed).multiplyScalar(k).add(posed));
      // The hand turned, by the least turn, so the bar runs across it.
      wrist.getWorldQuaternion(TURNED);
      const across = ACROSS.set(1, 0, 0).applyQuaternion(TURNED), bar = BAR.copy(along);
      if (across.dot(bar) < 0) bar.negate();
      SWING.setFromUnitVectors(across, bar);
      SWING.slerp(NO_TURN, 1 - k);
      wrist.parent.getWorldQuaternion(PARENT);
      wrist.quaternion.copy(PARENT.invert().multiply(SWING).multiply(TURNED));
      wrist.updateMatrixWorld(true);
    }
  }

  /**
   * Reaches the arm on `side` for `target`, a world point for the wrist, `k` of the way
   * from where the pose has it: the elbow where the arm's lengths put it, bent down and
   * out, and the arm straight toward it when it is out of reach.
   */
  armTo(side, target, k = 1) {
    const upper = this.bones.get(`upperarm_${side}`)?.bone, fore = this.bones.get(`forearm_${side}`)?.bone, wrist = this.bones.get(`hand_${side}`)?.bone;
    if (!upper || !fore || !wrist) return;
    this.rig.updateMatrixWorld(true);
    const shoulder = upper.getWorldPosition(SHOULDER), elbow = fore.getWorldPosition(ELBOW);
    const a = shoulder.distanceTo(elbow), b = elbow.distanceTo(wrist.getWorldPosition(AT));
    const goal = AT.lerp(target, k);
    const toward = ON.subVectors(goal, shoulder), d = Math.min(a + b - 1e-4, Math.max(Math.abs(a - b) + 1e-4, toward.length()));
    toward.normalize();
    POLE.set(side === 'L' ? -0.6 : 0.6, -1, 0.3).applyQuaternion(this.group.quaternion);
    POLE.addScaledVector(toward, -POLE.dot(toward)).normalize();
    const along = (a * a - b * b + d * d) / (2 * d), up = Math.sqrt(Math.max(0, a * a - along * along));
    point(upper, shoulder, HELD.copy(shoulder).addScaledVector(toward, along).addScaledVector(POLE, up));
    point(fore, fore.getWorldPosition(ELBOW), shoulder.addScaledVector(toward, d), wrist);
  }

  // Turns the body `lean` forward and down about the hips, which are kept at `hips`.
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
    this.lights.removeFromParent();
  }
}

const X = new THREE.Vector3(1, 0, 0), Y = new THREE.Vector3(0, 1, 0), Z = new THREE.Vector3(0, 0, 1);
const TURN = new THREE.Quaternion(), OUT = new THREE.Quaternion(), SIDE = new THREE.Quaternion(), AT = new THREE.Vector3(), HIPS = new THREE.Vector3(), ON = new THREE.Vector3();
const HELD = new THREE.Vector3();
const IDENTITY = new THREE.Matrix4(), MIRROR = new THREE.Matrix4().makeScale(-1, 1, 1);
const GOAL = new THREE.Matrix4(), BONE_GOAL = new THREE.Matrix4(), SCALED = new THREE.Matrix4();
const EYE_AT = new THREE.Vector3(), EYE_TURN = new THREE.Quaternion(), LOOK = new THREE.Euler(), ONE = new THREE.Vector3(1, 1, 1);
const TARGET = new THREE.Vector3(), TURNED = new THREE.Quaternion(), SIZE = new THREE.Vector3(), POLE = new THREE.Vector3();
const SHOULDER = new THREE.Vector3(), ELBOW = new THREE.Vector3();
// How far what a hand holds goes in a frame before it is a new hold to reach for.
const NEW_HOLD = 0.15;
const MOVED = new THREE.Vector3();
const POSED = new THREE.Vector3(), FIST_AT = new THREE.Vector3(), PALM = new THREE.Vector3(), GRIP_GOAL = new THREE.Vector3();
const ACROSS = new THREE.Vector3(), BAR = new THREE.Vector3(), NO_TURN = new THREE.Quaternion();
// How far in from the knuckles toward the palm a fist's middle is, in the hand model's
// units (tools.js FIST).
const PALM_DEPTH = 0.028;
const FROM = new THREE.Vector3(), TO = new THREE.Vector3(), SWING = new THREE.Quaternion(), PARENT = new THREE.Quaternion();

// Turns `bone`, whose head is at `head`, so that its child (`tip`, or its first bone)
// lies toward `goal`: by the least turn, so it keeps its twist.
function point(bone, head, goal, tip = bone.children.find(c => c.isBone)) {
  FROM.copy(tip.getWorldPosition(FROM)).sub(head).normalize();
  TO.subVectors(goal, head).normalize();
  SWING.setFromUnitVectors(FROM, TO);
  bone.getWorldQuaternion(EYE_TURN);
  bone.parent.getWorldQuaternion(PARENT);
  bone.quaternion.copy(PARENT.invert().multiply(SWING).multiply(EYE_TURN));
  bone.updateMatrixWorld(true);
}

// How many times a weave repeats along a unit of the map: some 2 cm.
const BODY_WEAVE = 180;

// How closed a hand is: relaxed, round a ball, and holding on to a ride.
const RELAXED = 0.3, BALL_GRIP = 0.4, GRIPPED = 0.85;

// How far out a tool is held, of the arm's reach, and how much lower than the view has it;
// and how far down the hands go with the look.
const HOLD_IN = 0.8, HOLD_LOW = 0.5, HOLD_PITCH = -0.3;

// From the first-person hand's space (hand_rig) to the space of a body's hand bone: the
// wrist's rest undone and scaled down to the body. The left hand's bone is the right's
// mirrored with its x turned back again, which MIRROR undoes.
let toBone = null, wristRest = null, handScale = 1;
function handToBone() {
  if (toBone) return toBone;
  const hand = model.parent.getObjectByName('hand_rig');
  const length = (rig, a, b) => rig.getObjectByName(a).getWorldPosition(AT).distanceTo(rig.getObjectByName(b).getWorldPosition(ON));
  handScale = length(model, 'hand_R', 'middle-finger-tip_R') / length(hand, 'wrist', 'middle-finger-tip');
  wristRest = hand.getObjectByName('wrist').matrixWorld.clone();
  toBone = new THREE.Matrix4().makeScale(handScale, handScale, handScale).multiply(wristRest.clone().invert());
  return toBone;
}
// The hand model's wrist at rest, in the hand model's space.
function handRest() {
  handToBone();
  return wristRest;
}

// How far forward the body leans running.
const RUN_LEAN = 0.16;

/** How far from walking to running the walker's pace is, 0 to 1. */
export function running(w) {
  const t = Math.max(0, Math.min(1, ((w.pace || 0) - 1) / 0.5));
  return t * t * (3 - 2 * t);
}

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
    // Each leg `at` its own point of the stride: walking, it swings about the hip and
    // the knee gives as it comes through; running, it swings further ahead than behind,
    // the heel tucked up under the seat as it comes through and the knee driven up,
    // and the knee never quite straight.
    const reach = Math.min(1.3, pace) * (1 - sit), run = running(w) * (1 - sit);
    const leg = (at, k) => ({
      thigh: (1 - run) * Math.sin(at) * 0.42 * reach + run * (0.25 + 0.6 * Math.sin(at)),
      shin: -((1 - run) * Math.max(0, -Math.sin(at + 0.9)) * 0.9 * reach + run * (0.3 + 1.5 * Math.max(0, Math.cos(at - 0.3)))),
      k,
    });
    for (const { thigh, shin, k } of [leg(phase, 'L'), leg(phase + Math.PI, 'R')]) {
      pose[`thigh_${k}`] = thigh;
      pose[`shin_${k}`] = shin;
    }
    pose.lean = RUN_LEAN * run;
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
    // Astride a car, a seesaw's plank or a spring rider, the thighs turned out round it
    // and the feet down beside it, not through it.
    if (ride === 'drive' || ride === 'rock') Object.assign(pose, { thigh_L_out: -STRADDLE * sit, thigh_R_out: STRADDLE * sit });
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
const REST_ARM = [0.15, 0.4, -0.04];
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
  const set = (side, forward, bend, out = -0.04) => {
    pose[`upperarm_${side}`] = [forward, out];
    pose[`forearm_${side}`] = [bend, 0];
    pose[`hand_${side}`] = [0, 0];
  };
  // ... or eased `k` of the way from how it is now.
  const towards = (side, forward, bend, out, k) => {
    const [f, o] = pose[`upperarm_${side}`], [b] = pose[`forearm_${side}`];
    set(side, f + (forward - f) * k, b + (bend - b) * k, o + (out - o) * k);
  };
  // At rest a little forward and bent at the elbow, the hands before the thighs;
  // walking, swinging against the legs; running, pumping from the shoulder, bent near
  // square at the elbow, the hands passing the hips.
  const run = running(w) * (1 - sit), swing = Math.sin(phase) * (0.3 + 0.1 * run) * Math.min(1.3, pace) * (1 - sit);
  const forward = REST_ARM[0] + (0.05 - REST_ARM[0]) * run, bent = REST_ARM[1] + (1.3 - REST_ARM[1]) * run;
  set('L', forward - swing, bent + Math.max(0, -swing) * 0.3, REST_ARM[2]);
  set('R', forward + swing, bent + Math.max(0, swing) * 0.3, REST_ARM[2]);
  pose.twist = -swing * 0.25;
  if (!p.ground && !r && sit === 0) {
    set('L', 0.35, 0.5, 0.5);
    set('R', 0.35, 0.5, 0.5);
  } else if (ride === 'slide' && r.s < r.marks.top) {
    // Reaching up the ladder, until the hands have a rung (rideGrips) - and at its top,
    // where there is none left, no higher than the shoulders.
    const up = Math.sin(phase * 1.3);
    set('L', 1.3 + up * 0.2, 0.5);
    set('R', 1.3 - up * 0.2, 0.5);
  }
  const hold = HOLDS[ride], k = ride === 'spin' ? 1 : sit;
  if (hold && k > 0) {
    for (const side of ['L', 'R']) towards(side, ...hold, k);
    pose.twist *= 1 - k;
  }
  // The hands relaxed, a little closed; shut round what they hold on to.
  const grip = ride === 'slide' && r.s < r.marks.top ? 1 : hold ? k : 0;
  pose.grip_L = pose.grip_R = RELAXED + (GRIPPED - RELAXED) * grip;
  if (kick > 0) {
    // Against the right leg: the left arm forward and up, the right one back and out.
    towards('L', 0.9, 0.6, 0.3, kick);
    towards('R', -0.6, 0.3, 0.6, kick);
  }
  // Seen from behind or looking down, a ball is carried before the chest in both hands. A
  // tool is held where the view holds it (Body.reach).
  if (w.bodyHolds && !r && w.ballHeld) {
    towards('L', 1.0, 1.1, -0.25, 1);
    towards('R', 1.0, 1.1, -0.25, 1);
    pose.grip_L = pose.grip_R = BALL_GRIP;
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
