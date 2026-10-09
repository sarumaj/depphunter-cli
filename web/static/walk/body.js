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
import { buildGear, fitOf, lick } from './gear.js';
import { aloft } from './parachute.js';
import { EYE, WALK, RUN, JUMP, SWIM_SINK, SWIM_PACE, reducedMotion } from './walkbase.js';

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

// A jump's legs, by how fast the walker is going up (walkbase.js JUMP): each thigh, shin
// and foot, left then right. Pushing off, the legs straight and the toes pointed, one
// trailing; at the top, the knees tucked up; coming down, reaching for the ground
// under the hips, bent ready to take it. Hanging under a jet, as before any of that.
const AIR_LEGS = {
  push: [0.2, -0.15, -0.55, -0.1, -0.35, -0.6],
  tuck: [0.85, -1.4, -0.15, 0.6, -1.15, -0.15],
  reach: [0.35, -0.5, -0.1, 0.18, -0.32, -0.1],
  hang: [0.45, -0.8, 0, 0.25, -0.55, 0],
};
const AIR_JOINTS = ['thigh_L', 'shin_L', 'foot_L', 'thigh_R', 'shin_R', 'foot_R'];
// Landing: how fast a fall the knees take all the way down and how fast one they do
// not give under at all, how far a thigh comes forward at the deepest, and how soon the
// knees are there - later after a harder landing - and up again in eight times that.
const LAND_HARD = 7, LAND_SOFT = 0.8, CROUCH = 1.2, LAND_PEAK = [0.06, 0.11];
// How fast the legs go to the air's pose and back, per second.
const AIR_EASE = 14;

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
    this.fists = { L: null, R: null };   // the middle of each fist, which holds on
    this.reaching = 0;                   // how far the torso leans in for a hold
    this.towing = 0;                     // how far a line is towing the body (tow) ...
    this.towYaw = 0;                     // ... toward the hook, and laid out how far
    this.towLean = 0;
    this.towSide = 'L';
    this.flown = 0;                      // how far the body is flying (flight) ...
    this.flightYaw = null;               // ... turned which way, if not where the walker looks,
    this.flightLean = 0;                 // and laid out how far, and over to which side
    this.flightRoll = 0;
    this.movedFrom = { x: 0, y: 0, z: 0, known: false }; // where the walker was last frame (measure) ...
    this.velocity = new THREE.Vector3();                 // ... and how fast they are going, eased
    this.swimPhase = 0;                  // the stroke (swimPose)
    this.adrift = 0;                     // how far down in the water with nothing to float on, eased
    this.air = 0;                        // how far off the ground the legs are, eased (posture)
    this.speed = 0;                      // how fast the feet are carrying the walker (gait)
    this.lastAt = { x: 0, z: 0, known: false }; // where the feet were last frame (gait)
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
    // What was worn for the last style goes with its rig.
    if (this.gear) {
      for (const part of [this.gear.jetpack, this.gear.container, this.gear.ring]) {
        part.traverse(o => { o.geometry?.dispose(); o.material?.dispose(); });
      }
    }
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
    // What is worn for a tool, on the bone it goes with: the packs on the back and the
    // ring round the chest.
    rig.updateMatrixWorld(true);
    this.gear = buildGear(fitOf(pick, rig));
    const hang = (part, bone) => {
      part.matrixAutoUpdate = false;
      part.matrix.copy(bone.matrixWorld).invert().multiply(rig.matrixWorld);
      bone.add(part);
    };
    const spine = this.bones.get('spine')?.bone;
    if (spine) for (const part of [this.gear.jetpack, this.gear.container, this.gear.ring]) hang(part, spine);
  }

  /**
   * Stands the body under a walker this frame: `w` is the Walker. Hidden while there
   * is no walker out there to have one. On their feet, the lower foot is on the
   * ground; sitting, the hips are on the seat and the feet clear of what is under it.
   */
  update(w, deltaTime) {
    const shown = w.active && !w.arrival && w.dying === null && !!this.rig;
    this.group.visible = this.lights.visible = shown;
    if (!shown) {
      this.lastAt.known = false; // nowhere to stride from when it is shown again (gait)
      this.movedFrom.known = false; // ... nor to fly or swim from (measure)
      return;
    }
    const view = this.scene.walkCamera;
    view.updateWorldMatrix(true, false);
    view.matrixWorld.decompose(this.lights.position, this.lights.quaternion, AT);
    const p = w.p, sit = sitting(w);
    this.measure(w, deltaTime);
    // Sitting, the eye is SIT over the seat and the hips on it.
    const eye = p.feet + EYE + (w.eyeShift || 0), seat = eye - SIT;
    // Swimming in the ring, down in the water to the chest - and with nothing to float
    // on, the same at first, then going under as the walker's view does (walk.js drown).
    const swim = (w.swim || 0) * (1 - sit), drowning = (w.sunk || 0) * (1 - sit);
    this.adrift += ((w.sinking ? 1 : 0) - this.adrift) * Math.min(1, deltaTime * 4);
    const under = drowning * drowning * (3 - 2 * drowning), wet = Math.max(swim, this.adrift, Math.min(1, drowning * DROWN_IN));
    const y = p.feet + (seat + SEAT_UP - HIP - p.feet) * sit - SWIM_SINK * wet - DROWN_DEPTH * under;
    const room = seat - w.height(p.x, p.z, seat);
    // Down a chute, the legs out ahead along half its slope, as a rider holds them -
    // turned about the hips, which stay on the seat.
    // Sat, the body faces the seat's way, not where the walker looks.
    const look = w.rideYaw?.() ?? p.yaw;
    let yaw = p.yaw + Math.atan2(Math.sin(look - p.yaw), Math.cos(look - p.yaw)) * sit;
    // Reeled in on a line, seen from outside the eye: hauled along it by the arm that
    // holds it, the body turned to it and laid out behind that arm.
    const towed = this.tow(w, deltaTime);
    yaw += Math.atan2(Math.sin(this.towYaw - yaw), Math.cos(this.towYaw - yaw)) * towed;
    // Flying, the same as if the flight were a line pulling the body the way it goes -
    // and a line towing it takes over from that.
    const flown = this.flight(w, deltaTime) * (1 - towed);
    if (this.flightYaw !== null) yaw += Math.atan2(Math.sin(this.flightYaw - yaw), Math.cos(this.flightYaw - yaw)) * flown;
    // Swimming, leant into the ring the way the walker swims, further flat out.
    const way = swimWay(this.velocity, p.yaw), stroking = swim * way.going;
    const swimLean = Math.max(-SWIM_BACK, way.ahead * (SWIM_LEAN + SWIM_DASH * way.hard)) * stroking;
    const laid = this.towLean * towed + this.flightLean * flown + swimLean + DROWN_LEAN * under;
    const rolled = this.flightRoll * flown - way.across * SWIM_ROLL * stroking;
    const r = w.riding, slide = r?.entry.ride === 'slide' ? r : null;
    const slope = slide && r.s > r.marks.edge && r.s < r.marks.foot ? r.slopeAt(r.s) : 0;
    this.lean = (this.lean || 0) + (slope * 0.5 * sit - (this.lean || 0)) * Math.min(1, deltaTime * 8);
    // Looking down from the eye, the head is bent over the chest; seen from behind, it is not.
    const behind = BEHIND + (w.thirdPerson ? 0 : STOOP * Math.min(1, Math.max(0, -p.pitch) / (Math.PI / 2)));
    const hipsAt = HIPS.set(p.x + Math.sin(yaw) * behind, y + HIP, p.z + Math.cos(yaw) * behind);
    // On a swing or a seesaw, the legs tipped with the seat, not through it.
    const tipped = (w.rideLean?.() ?? 0) * sit;
    this.tilt(this.lean + tipped + laid, yaw, hipsAt, rolled);
    // The stride as fast as the walker is going over the ground - measured, so the legs
    // stop against a wall and slow as the walker does - each stride a gait's length:
    // running, the strides come longer as well as quicker.
    const gait = this.gait(w, deltaTime);
    this.phase = (this.phase + deltaTime * Math.PI * 2 * this.speed / (STRIDE.walk + (STRIDE.run - STRIDE.walk) * running(w, gait))) % (Math.PI * 2);
    // Off the ground and back on it eased, so a curb stepped off is not a jump.
    this.air += ((p.ground ? 0 : 1) - this.air) * Math.min(1, deltaTime * AIR_EASE);
    const pose = posture(w, this.phase, performance.now(), room, gait, this.air);
    if (towed > 0) towPose(pose, this.towSide, towed);
    if (flown > 0) flightPose(pose, this.flightLean, this.flightRoll, flown);
    // The stroke on a phase of its own, so it quickens without jumping.
    // The stroke on a phase of its own, so it quickens without jumping - frantic, going under.
    const fighting = drowning * (1 - under);
    this.swimPhase = (this.swimPhase + deltaTime * (SWIM_RATE.tread + SWIM_RATE.swim * way.going + SWIM_RATE.dash * Math.max(way.hard, fighting))) % (Math.PI * 4);
    if (wet > 0) swimPose(pose, this.swimPhase, way, wet);
    if (drowning > 0) drownPose(pose, this.swimPhase, drowning, reducedMotion());
    for (const name of ['thigh_L', 'thigh_R', 'shin_L', 'shin_R', 'foot_L', 'foot_R']) this.turn(name, pose[name] || 0, pose[`${name}_out`] || 0, pose[`${name}_spread`] || 0);
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
        this.tilt(this.lean + tipped + laid, yaw, hipsAt, rolled);
      }
    }
    // The legs lean about the hips; the torso stays upright under the eye - but for a
    // runner's lean - turning a little against the stride as a walker's shoulders do.
    const upright = this.lean + tipped + (pose.lean || 0);
    this.turn('spine', upright, (pose.twist || 0));
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
    // Leaning in from the hips for a hold past the arms' reach - a rider's handlebar as
    // it rocks away, the rung over a climber's head - rather than the arm straining.
    let short = 0;
    const spine = this.bones.get('spine')?.bone;
    if (grips && spine) {
      this.rig.updateMatrixWorld(true);
      const bend = spine.getWorldPosition(AT);
      for (const [i, side] of ['L', 'R'].entries()) {
        const upper = this.bones.get(`upperarm_${side}`)?.bone;
        if (!grips[i] || !upper) continue;
        const shoulder = upper.getWorldPosition(SHOULDER);
        short = Math.max(short, (shoulder.distanceTo(grips[i].at) - ARM_REACH) / Math.max(0.05, shoulder.y - bend.y));
      }
    }
    this.reaching += (Math.min(MAX_REACH_LEAN, short) * holding - this.reaching) * Math.min(1, deltaTime * 6);
    if (this.reaching > 1e-3) this.turn('spine', upright - this.reaching, (pose.twist || 0));
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
      if (held) {
        this.follow(held);
        // Swimming somewhere, the hands are swimming, whatever they hold.
        this.reach(side, held, w, 1 - Math.max(stroking, Math.min(1, drowning * DROWN_IN)));
        if (towed > 0 && held.vm === w.pull?.hand) this.haul(side, w.pull.to, towed);
      }
      closeHand(this.hands[side], held ? held.grip : pose[`grip_${side}`] ?? RELAXED);
      if (held?.trigger != null) closeFinger(this.hands[side], 0, held.trigger);
    }
    // The head looks where the eye does, as far as a neck turns.
    const glance = Math.max(-1.4, Math.min(1.4, Math.atan2(Math.sin(p.yaw - yaw), Math.cos(p.yaw - yaw))));
    // ... level however the body leans, running or reaching for a hold.
    // Laid out in flight, it is raised to look the way the body goes, as far as a neck bends back.
    const raised = Math.max(-0.4, Math.min(0.8, this.flightLean * flown + swimLean));
    this.turn('head', Math.max(-0.9, Math.min(0.7, p.pitch)) + (pose.lean || 0) + this.reaching + raised + (pose.head || 0), glance);
    this.bones.get('head')?.bone.scale.setScalar(this.showHead || w.thirdPerson ? 1 : 1e-4);
    this.dress(w);
    this.float(swim);
    // A ball the body carries is between the hands.
    if (w.bodyHolds && w.ballHeld && arms) {
      this.rig.updateMatrixWorld(true);
      const left = this.bones.get('hand_L')?.bone.getWorldPosition(AT), right = this.bones.get('hand_R')?.bone.getWorldPosition(ON);
      if (left && right) w.ballHeld.mesh.position.addVectors(left, right).multiplyScalar(0.5).add(HELD.set(-Math.sin(yaw), 0, -Math.cos(yaw)).multiplyScalar(0.03));
    }
    // Standing or walking, the lower foot planted on the ground; swimming, they hang in
    // the water.
    if (p.ground && sit < 1 && wet < 1) {
      this.rig.updateMatrixWorld(true);
      const low = Math.min(...['foot_L', 'foot_R'].map(n => this.bones.get(n)?.bone.getWorldPosition(AT).y ?? y + SOLE));
      this.group.position.y += (p.feet - (low - SOLE)) * (1 - sit) * (1 - wet);
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
    // ... and without what each part keeps in its userData, which a clone copies by way
    // of JSON: the viewmodel's keeps its hands, and the copy of a hand model as JSON is
    // megabytes thrown away at every change of hands. The copy has no use for any of it.
    const kept = [];
    vm.traverse(o => { kept.push([o, o.userData]); o.userData = {}; });
    const copy = vm.clone();
    for (const [o, data] of kept) o.userData = data;
    // ... each part paired with the one it was copied from, to move as that does
    // (follow) - but for what reaches round from the back into the view, which the
    // body wears there instead (dress).
    const parts = [], pairs = [];
    vm.traverse(o => parts.push(o));
    copy.traverse(o => {
      const from = parts[pairs.length];
      pairs.push([from, o]);
      if (from.userData.worn) o.visible = false;
    });
    for (const [holding, h] of hands) holding.add(h);
    // ... and laid in the body's hand, mirrored for the left.
    copy.matrixAutoUpdate = false;
    copy.matrix.copy(side === 'L' ? MIRROR : IDENTITY).multiply(handToBone()).multiply(inHand);
    copy.traverse(o => { o.frustumCulled = false; o.renderOrder = 0; });
    bone.add(copy);
    const rest = holder.userData.restGrip ?? vm.userData.restGrip ?? 0.75;
    this.carried[side] = { vm, copy, grip: rest, trigger: vm.userData.trigger, pairs: pairs.slice(1).filter(([from]) => !from.userData.worn) };
  }

  /**
   * How far the body is being towed along a line (shots.js reel), eased in and out, 0
   * to 1 - only seen from outside the eye, which hangs on regardless - and, into
   * `towYaw` and `towLean`, the way to the hook and how far the body is laid out toward
   * it: flat along a line across, up along one going up, head first down one going down.
   */
  tow(w, deltaTime) {
    const pull = (w.thirdPerson || this.showHead) && !w.riding ? w.pull : null;
    if (pull) {
      const dx = pull.to.x - w.p.x, dy = pull.to.y - w.p.feet - HIP, dz = pull.to.z - w.p.z;
      this.towYaw = Math.atan2(-dx, -dz);
      this.towLean = Math.max(0.35, Math.min(1.5, Math.PI / 2 - Math.atan2(dy, Math.hypot(dx, dz)))) * TOWED;
      // Which hand holds the line, kept while the body comes upright again after it.
      this.towSide = pull.hand === w.offhand ? 'L' : 'R';
    }
    this.towing = (this.towing || 0) + ((pull ? 1 : 0) - (this.towing || 0)) * Math.min(1, deltaTime * 6);
    return this.towing < 1e-3 ? 0 : this.towing;
  }

  /**
   * How far the body is flying - on the jet backpack, off the ground, or hanging under
   * an open canopy - eased in and out, 0 to 1, only seen from outside the eye; and, into
   * `flightLean` and `flightRoll`, how it hangs. On the jet it is as if a line pulled it
   * the way it goes: the faster, the further it is laid out along the flight - forward
   * going forward, back going back, over to the side going sideways, upright going
   * straight up or down. Under a canopy it hangs from that instead, faced the way the
   * canopy flies (`flightYaw`) and swinging with it under the lines (parachute.js).
   */
  flight(w, deltaTime) {
    const p = w.p, open = aloft(w.chute), velocity = this.velocity;
    const flies = (w.thirdPerson || this.showHead) && !w.riding && (open || (!!p.fly && !p.ground));
    if (flies) {
      // Measured against the way the body faces: the canopy's heading under one, where
      // the walker looks on the jet.
      const facing = open ? w.chute.heading : p.yaw;
      const ahead = -velocity.x * Math.sin(facing) - velocity.z * Math.cos(facing);
      const across = velocity.x * Math.cos(facing) - velocity.z * Math.sin(facing);
      // Under a canopy the way down is the lines' doing and not a pull: only the way it
      // drifts over the ground lays the body over, and less than a jet's.
      const up = open ? 0 : velocity.y;
      const level = Math.hypot(ahead, across), speed = Math.hypot(level, up);
      // Off the upright toward the way it goes, as far as that is from straight up, and
      // further the faster - but not about a way that is all but straight up or down,
      // which has no side to lie over to.
      const off = Math.min(MAX_FLIGHT_LEAN, Math.atan2(level, up)) * (speed / (speed + FLIGHT_EASE)) * (level / (level + 0.5)) * (open ? CANOPY_LEAN : 1);
      const lean = level > 1e-6 ? Math.max(-MAX_FLIGHT_BACK, (off * ahead) / level) : 0;
      const roll = level > 1e-6 ? (-off * across) / level : 0;
      const swing = open ? w.chute.swing : null;
      this.flightYaw = open ? facing : null;
      this.flightLean = lean - (swing?.pitch || 0);
      this.flightRoll = roll + (swing?.roll || 0);
    }
    this.flown += ((flies ? 1 : 0) - this.flown) * Math.min(1, deltaTime * 6);
    return this.flown < 1e-3 ? 0 : this.flown;
  }

  /**
   * How fast and which way the walker is going, into `velocity`: measured, as the
   * stride's speed is, and eased, so a body swings round to a new way behind a jet or a
   * stroke rather than snapping to it. Not on a ride, which carries the walker its own way.
   */
  measure(w, deltaTime) {
    const p = w.p, from = this.movedFrom, known = !w.riding;
    if (!known) this.velocity.set(0, 0, 0);
    else if (from.known && deltaTime > 0) this.velocity.lerp(MOVED.set(p.x - from.x, p.feet - from.y, p.z - from.z).divideScalar(deltaTime), Math.min(1, deltaTime * 5));
    from.x = p.x;
    from.y = p.feet;
    from.z = p.z;
    from.known = known;
  }

  /**
   * The swim ring flat on the water round the chest, however the body leans in it - as
   * far as it is swimming (`swim`); out of the water it is worn as the chest has it.
   */
  float(swim) {
    const ring = this.gear?.ring, spine = this.bones?.get('spine')?.bone;
    if (!ring?.visible || !spine) return;
    ring.userData.rest ||= ring.matrix.clone();
    ring.matrix.copy(ring.userData.rest);
    if (swim > 0) {
      spine.updateWorldMatrix(true, false);
      FLOAT.multiplyMatrices(spine.matrixWorld, ring.matrix).decompose(FLOAT_AT, FLOAT_TURN, FLOAT_SIZE);
      // About its middle, which stays where the chest has it, turned only the way the body faces.
      const middle = FLOAT_MIDDLE.copy(ring.userData.center).applyMatrix4(FLOAT);
      FLOAT_TURN.slerp(LEVEL.setFromAxisAngle(Y, this.group.rotation.y).multiply(this.rig.quaternion), swim);
      FLOAT.compose(FLOAT_AT.set(0, 0, 0), FLOAT_TURN, FLOAT_SIZE);
      FLOAT.setPosition(middle.sub(FLOAT_AT.copy(ring.userData.center).applyMatrix4(FLOAT)));
      ring.matrix.copy(spine.matrixWorld).invert().multiply(FLOAT);
    }
    ring.matrixWorldNeedsUpdate = true;
  }

  // The arm on `side` straight out toward the hook `to`, `k` of the way, as the line hauls on it.
  haul(side, to, k) {
    const upper = this.bones.get(`upperarm_${side}`)?.bone;
    if (!upper) return;
    upper.updateWorldMatrix(true, true);
    const shoulder = upper.getWorldPosition(SHOULDER);
    const wrist = this.bones.get(`hand_${side}`).bone.getWorldPosition(POSED);
    GRIP_GOAL.subVectors(to, shoulder).setLength(0.16).add(shoulder);
    this.armTo(side, wrist.lerp(GRIP_GOAL, k));
  }

  /**
   * How fast the walker's feet are carrying them over the ground this frame, eased into
   * `speed`, and that as a pace (posture): 1 walking, 1.5 running, and in proportion
   * between and below - nothing in the air, flying or carried by a ride or a line.
   */
  gait(w, deltaTime) {
    const p = w.p, last = this.lastAt;
    let speed = 0;
    if (last.known && deltaTime > 0 && p.ground && !p.fly && !w.riding) speed = Math.min(RUN * 1.5, Math.hypot(p.x - last.x, p.z - last.z) / deltaTime);
    last.x = p.x;
    last.z = p.z;
    last.known = true;
    this.speed += (speed - this.speed) * Math.min(1, deltaTime * 10);
    return this.speed <= WALK ? this.speed / WALK : 1 + 0.5 * Math.min(1, (this.speed - WALK) / (RUN - WALK));
  }

  // A carried copy's parts where the tool's own are this frame: a claw gone, a lever
  // pushed, a handle let go of.
  follow(held) {
    for (const [from, to] of held.pairs) {
      to.position.copy(from.position);
      to.quaternion.copy(from.quaternion);
      to.scale.copy(from.scale);
      to.visible = from.visible;
    }
  }

  /**
   * What is worn for the tool in the off hand: the jet backpack, its flames out while
   * it flies; the parachute's container, with the canopy out of it or not; the swim
   * ring. The packs are on the back, so they are drawn only for a view from outside the
   * eye; the ring is seen looking down too, once the view's own edge of it (tools.js)
   * has gone with what the view holds.
   */
  dress(w) {
    const gear = this.gear;
    if (!gear) return;
    const kit = w.secondary?.id, behind = w.thirdPerson || this.showHead, open = aloft(w.chute);
    gear.jetpack.visible = behind && kit === 'jetpack';
    gear.container.visible = behind && (kit === 'parachute' || open);
    gear.ring.visible = kit === 'ring' && (behind || !!w.bodyHolds);
    if (gear.jetpack.visible) {
      const flying = !!w.p.fly;
      for (const flame of gear.jetpack.userData.flames) flame.visible = flying;
      if (flying) lick(gear.jetpack, 1 + (w.burst > 0 ? 1.4 : 0), performance.now());
    }
    const flap = gear.container.getObjectByName('flap');
    if (flap) flap.rotation.x = w.chute && w.chute.phase !== 'packed' ? 1.2 : 0;
  }

  /**
   * Where the canopy's four risers leave the harness, in the world, into `out` - or
   * null while the container is not drawn, and the risers are hung where the eye has
   * them (canopy.js).
   */
  risers(out) {
    const container = this.gear?.container;
    if (!container?.visible || !this.group.visible) return null;
    container.updateWorldMatrix(true, false);
    container.userData.risers.forEach((at, i) => (out[i] ||= new THREE.Vector3()).copy(at).applyMatrix4(container.matrixWorld));
    return out;
  }

  /**
   * Puts the hand on `side` where the view's hand holding `held` is: as far from the
   * eye, turned the same way to it, at the body's scale - the arm reaching it with the
   * elbow down and out, as far as it reaches.
   */
  reach(side, held, w, k = 1) {
    if (k <= 1e-3) return;
    const hand = held.vm.userData.hands[0].userData.hand, view = w.held;
    const fore = this.bones.get(`forearm_${side}`)?.bone, wrist = this.bones.get(`hand_${side}`)?.bone;
    if (!view || !fore || !wrist) return;
    // The view's hand from the held group, which is the eye's, in the hand model's units...
    hand.updateWorldMatrix(true, false);
    GOAL.copy(view.matrixWorld).invert().multiply(hand.matrixWorld);
    // ... before the walker's own eye, at the body's scale: where the hand bone goes.
    const p = w.p, rest = handRest(), s = handScale;
    EYE_AT.set(p.x, p.feet + EYE + (w.eyeShift || 0) - SWIM_SINK * (w.swim || 0), p.z);
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
    upper.updateWorldMatrix(true, true);
    const shoulder = upper.getWorldPosition(SHOULDER), reach = shoulder.distanceTo(fore.getWorldPosition(ELBOW)) + ELBOW.distanceTo(wrist.getWorldPosition(AT));
    TARGET.sub(shoulder);
    TARGET.y -= HOLD_LOW * reach;
    TARGET.setLength(Math.min(TARGET.length(), HOLD_IN * reach)).add(shoulder);
    this.armTo(side, TARGET, k);
    // The hand turned as the view's is.
    fore.getWorldQuaternion(EYE_TURN);
    wrist.quaternion.slerp(EYE_TURN.invert().multiply(TURNED), k);
    wrist.updateMatrixWorld(true);
  }

  /**
   * Holds on with the hand on `side`, `k` of the way from where the pose has it, to a
   * bar through `at` running `along`: the fist round it, the bar across the palm.
   */
  grip(side, at, along, k) {
    const wrist = this.bones.get(`hand_${side}`), fore = this.bones.get(`forearm_${side}`)?.bone;
    const knuckle = this.bones.get(`middle-finger-phalanx-proximal_${side}`)?.bone, thumb = this.bones.get(`thumb-phalanx-proximal_${side}`)?.bone;
    if (!wrist || !fore || !knuckle || !thumb) return;
    const hand = wrist.bone;
    // Only the arm is moved here, so only the arm - and what it hangs from - is brought
    // up to date, not the whole rig: this runs several times a frame for each hand.
    fore.parent.updateWorldMatrix(true, true);
    handToBone();
    // The fist's middle, which the arm reaches with: at the knuckles, in toward the palm.
    const fist = (this.fists[side] ||= new THREE.Object3D());
    if (fist.parent !== hand) hand.add(fist);
    hand.getWorldQuaternion(TURNED);
    fist.position.copy(hand.worldToLocal(knuckle.getWorldPosition(FIST_AT)
      .addScaledVector(PALM.set(0, 0, -1).applyQuaternion(TURNED), PALM_DEPTH * handScale)));
    fist.updateMatrixWorld(true);
    const posed = fist.getWorldPosition(POSED), goal = GRIP_GOAL.subVectors(at, posed).multiplyScalar(k).add(posed);
    // The thumb the way round a bar it goes most easily: up, in toward the body and
    // forward.
    THUMB_WAY.set(side === 'L' ? 1 : -1, 1, -0.5).applyQuaternion(this.group.quaternion);
    const bar = BAR.copy(along).normalize();
    if (bar.dot(THUMB_WAY) < 0) bar.negate();
    let rolled = 0;
    // The fist put round the bar, the hand turned so the bar runs across it, and over
    // again: how the hand is turned depends on where the arm has put it.
    this.armTo(side, goal, 1, fist);
    for (let pass = 0; pass < 2; pass++) {
      // From the wrist held straight, the forearm rolled - as far as one rolls - so the
      // thumb comes round toward the bar...
      hand.quaternion.copy(wrist.rest);
      hand.updateMatrixWorld(true);
      const length = TO.subVectors(hand.getWorldPosition(AT), fore.getWorldPosition(ELBOW)).normalize();
      const thumbWard = this.thumbWard(hand, thumb);
      const from = FROM.copy(thumbWard).addScaledVector(length, -thumbWard.dot(length));
      const to = REACH.copy(bar).addScaledVector(length, -bar.dot(length));
      const roll = from.lengthSq() > 1e-8 && to.lengthSq() > 1e-8 ? Math.atan2(length.dot(SPUN.crossVectors(from, to)), from.dot(to)) : 0;
      const kept = Math.max(-MAX_ROLL, Math.min(MAX_ROLL, rolled + roll * k)) - rolled;
      rolled += kept;
      fore.getWorldQuaternion(PARENT);
      fore.parent.getWorldQuaternion(EYE_TURN);
      fore.quaternion.copy(EYE_TURN.invert().multiply(ROLL.setFromAxisAngle(length, kept)).multiply(PARENT));
      fore.updateMatrixWorld(true);
      // ... and the wrist bent the rest of the way, no further than a wrist bends.
      SWING.setFromUnitVectors(this.thumbWard(hand, thumb), bar);
      const bend = 2 * Math.acos(Math.min(1, Math.abs(SWING.w)));
      SWING.slerp(NO_TURN, 1 - Math.min(1, MAX_BEND / Math.max(bend, 1e-6)) * k);
      hand.getWorldQuaternion(TURNED);
      hand.quaternion.multiply(BENT.copy(TURNED).invert().multiply(SWING).multiply(TURNED));
      hand.updateMatrixWorld(true);
      this.armTo(side, goal, 1, fist);
    }
  }

  // Which way across the hand, in the world, its thumb is.
  thumbWard(hand, thumb) {
    hand.getWorldQuaternion(TURNED);
    const across = ACROSS.set(1, 0, 0).applyQuaternion(TURNED);
    return thumb.getWorldPosition(SPUN).sub(hand.getWorldPosition(FIST_AT)).dot(across) < 0 ? across.negate() : across;
  }

  /**
   * Reaches the arm on `side` for `target`, a world point for the wrist - or for `end`,
   * a point the hand carries - `k` of the way from where the pose has it: the elbow where
   * the arm's lengths put it, bent down and out, and the arm straight toward it when it
   * is out of reach.
   */
  armTo(side, target, k = 1, end = null) {
    const upper = this.bones.get(`upperarm_${side}`)?.bone, fore = this.bones.get(`forearm_${side}`)?.bone, wrist = end || this.bones.get(`hand_${side}`)?.bone;
    if (!upper || !fore || !wrist) return;
    upper.updateWorldMatrix(true, true);
    const shoulder = upper.getWorldPosition(SHOULDER), elbow = fore.getWorldPosition(ELBOW);
    const a = shoulder.distanceTo(elbow), b = elbow.distanceTo(wrist.getWorldPosition(AT));
    const goal = AT.lerp(target, k);
    const toward = ON.subVectors(goal, shoulder), d = Math.min(a + b - 1e-4, Math.max(Math.abs(a - b) + 1e-4, FOLDED * (a + b), toward.length()));
    toward.normalize();
    // The torso stays upright however the legs tip (update), and the elbow with it.
    POLE.set(side === 'L' ? -0.6 : 0.6, -1, 0.3).applyAxisAngle(Y, this.group.rotation.y);
    POLE.addScaledVector(toward, -POLE.dot(toward)).normalize();
    const along = (a * a - b * b + d * d) / (2 * d), up = Math.sqrt(Math.max(0, a * a - along * along));
    point(upper, shoulder, HELD.copy(shoulder).addScaledVector(toward, along).addScaledVector(POLE, up));
    point(fore, fore.getWorldPosition(ELBOW), shoulder.addScaledVector(toward, d), wrist);
  }

  // Turns the body `lean` forward and down about the hips, which are kept at `hips`,
  // and `roll` over to its left.
  tilt(lean, yaw, hips, roll = 0) {
    this.group.rotation.set(-lean, yaw, roll, 'YXZ');
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
const FLOAT = new THREE.Matrix4(), FLOAT_AT = new THREE.Vector3(), FLOAT_TURN = new THREE.Quaternion(), FLOAT_SIZE = new THREE.Vector3();
const FLOAT_MIDDLE = new THREE.Vector3(), LEVEL = new THREE.Quaternion();
const POSED = new THREE.Vector3(), FIST_AT = new THREE.Vector3(), PALM = new THREE.Vector3(), GRIP_GOAL = new THREE.Vector3();
const ACROSS = new THREE.Vector3(), BAR = new THREE.Vector3(), NO_TURN = new THREE.Quaternion(), THUMB_WAY = new THREE.Vector3();
const ROLL = new THREE.Quaternion(), BENT = new THREE.Quaternion(), REACH = new THREE.Vector3(), SPUN = new THREE.Vector3();
// How far from the shoulder a hold is held with the arm not straining, and how far the
// torso leans in for one further off.
const ARM_REACH = 0.15, MAX_REACH_LEAN = 0.25;
// How far a forearm rolls, and a wrist bends, holding on; how close to the shoulder,
// of the arm's reach, the wrist comes with the elbow bent as far as one goes.
const MAX_ROLL = 1.3, MAX_BEND = 0.8, FOLDED = 0.25;
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

// How far the walker goes in one stride - both feet - walking and running. The map's
// walker covers ground far faster than legs their size could: these keep a walking and a
// running cadence a body could keep, the one about one and a half times the other.
const STRIDE = { walk: 2.7, run: 4.5 };

/** How far from walking to running the walker's `pace` (w.pace, or Body.gait's) is, 0 to 1. */
export function running(w, pace = w.pace || 0) {
  const t = Math.max(0, Math.min(1, (pace - 1) / 0.5));
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
 * doing: sitting (feet kept `room` clear of what is under the seat), in the air -
 * `air` of the way there, eased by the body (Body.air) - climbing a ladder, walking at
 * `phase` of a stride, the knees giving under a landing, and a kick taken lately laid
 * over any of them.
 */
export function posture(w, phase, now, room = Infinity, pace = w.pace || 0, air = w.p.ground ? 0 : 1) {
  const p = w.p, r = w.riding, ride = r?.entry.ride, sit = sitting(w);
  const pose = {};
  air = r || sit > 0 ? 0 : air;
  if (ride === 'slide' && r.s < r.marks.top) {
    // Climbing: a leg up a rung at a time.
    const up = Math.max(0, Math.sin(phase * 1.3));
    Object.assign(pose, { thigh_L: up * 1.1, thigh_R: (1 - up) * 1.1, shin_L: -up * 1.4, shin_R: -(1 - up) * 1.4 });
  } else {
    // Each leg `at` its own point of the stride: walking, it swings about the hip and
    // the knee gives as it comes through; running, it swings further ahead than behind,
    // the heel tucked up under the seat as it comes through and the knee driven up,
    // and the knee never quite straight.
    const reach = Math.min(1.3, pace) * (1 - sit), run = running(w, pace) * (1 - sit);
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
  if (air > 0) {
    const legs = airLegs(p);
    for (const [j, joint] of AIR_JOINTS.entries()) pose[joint] = (pose[joint] || 0) + (legs[j] - (pose[joint] || 0)) * air;
    pose.lean = (pose.lean || 0) * (1 - air);
  }
  if (!r) absorb(pose, w, now);
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
  arms(pose, w, phase, sit, kicking ? Math.sin(Math.PI * since / KICK_KEYS.at(-1)[0]) : 0, pace, air, now);
  // No joint bent further than it goes, nor a knee or an elbow the wrong way.
  for (const [joint, [low, high]] of Object.entries(JOINT_RANGES)) {
    for (const name of [`${joint}_L`, `${joint}_R`]) {
      const clamp = angle => Math.max(low, Math.min(high, angle));
      if (Array.isArray(pose[name])) pose[name][0] = clamp(pose[name][0]);
      else if (pose[name] !== undefined) pose[name] = clamp(pose[name]);
    }
  }
  return pose;
}

/**
 * The legs in the air (AIR_LEGS), for `p`'s speed up or down: from the push off the
 * ground, straight and pointed, through the knees tucked at the top of the jump, to
 * the legs reaching down for the ground as the walker falls - from a jump, or off an
 * edge, which starts at the top.
 *
 * Implements: REQ-WALK-063
 */
export function airLegs(p) {
  if (p.fly) return AIR_LEGS.hang;
  const up = Math.max(-1.5, Math.min(1, (p.vy || 0) / JUMP));
  const smooth = (a, b, x) => { const t = Math.max(0, Math.min(1, (x - a) / (b - a))); return t * t * (3 - 2 * t); };
  const push = smooth(0.15, 1, up), reach = smooth(0.05, -0.65, up);
  return AIR_LEGS.tuck.map((tuck, j) => tuck + (AIR_LEGS.push[j] - tuck) * push + (AIR_LEGS.reach[j] - tuck) * reach);
}

/**
 * How far the knees are giving under a landing (walk.js landed) `now`, 0 to 1: down
 * at once, deeper and slower the harder the feet came down, and up again smoothly -
 * (t/τ)·e^(1 - t/τ), at its deepest at τ.
 *
 * Implements: REQ-WALK-063
 */
export function landing(w, now) {
  const { at, speed } = w.landed || {};
  const depth = Math.sqrt(Math.min(1, Math.max(0, (speed || 0) - LAND_SOFT) / (LAND_HARD - LAND_SOFT)));
  const peak = LAND_PEAK[0] + (LAND_PEAK[1] - LAND_PEAK[0]) * depth;
  const t = (now - (at ?? -Infinity)) / 1000 / peak;
  return t > 0 && t < 8 ? depth * t * Math.exp(1 - t) : 0;
}

/**
 * Lays the knees' give under a landing over `pose`: each thigh forward and the shin
 * back so that the foot stays under the hips, flat on the ground, and the body leaning
 * a little over its knees. The feet planted (Body.update), bending the knees is what
 * brings the hips down.
 *
 * Implements: REQ-WALK-063
 */
function absorb(pose, w, now) {
  const k = landing(w, now);
  if (k <= 0) return;
  const thigh = CROUCH * k, shin = -thigh - Math.asin(Math.min(1, THIGH * Math.sin(thigh) / SHIN));
  for (const side of ['L', 'R']) {
    for (const [joint, to] of [['thigh', thigh], ['shin', shin], ['foot', -(thigh + shin)]]) {
      const name = `${joint}_${side}`;
      pose[name] = (pose[name] || 0) + (to - (pose[name] || 0)) * Math.min(1, k * 2.5);
    }
  }
  pose.lean = (pose.lean || 0) + 0.2 * k;
}

// How much of the way the body is laid out along a line that tows it.
const TOWED = 0.8;

/**
 * Towed (Body.tow), `k` of the way: the legs trailing, a little bent, and the arm that
 * does not hold the line (`holding`) flung back and out.
 */
export function towPose(pose, holding, k) {
  const free = holding === 'L' ? 'R' : 'L';
  for (const [joint, to] of [['thigh_L', -0.15], ['thigh_R', 0.1], ['shin_L', -0.7], ['shin_R', -0.35], ['foot_L', -0.3], ['foot_R', -0.3]]) {
    pose[joint] = (pose[joint] || 0) + (to - (pose[joint] || 0)) * k;
  }
  const [forward, out] = pose[`upperarm_${free}`], [bend] = pose[`forearm_${free}`];
  pose[`upperarm_${free}`] = [forward + (-0.6 - forward) * k, out + (0.7 - out) * k];
  pose[`forearm_${free}`] = [bend + (0.3 - bend) * k, 0];
}

// How far a jet laid out along its flight is laid out at most, forward and back; the
// speed at which it is half as far as that; and how much of that drifting under a
// canopy lays the body over.
const MAX_FLIGHT_LEAN = 1.4, MAX_FLIGHT_BACK = 0.8, FLIGHT_EASE = 6, CANOPY_LEAN = 0.25;
// How far the legs swing out to the side, flying sideways - the sign is the bones' own.
const LEG_SPREAD = -0.35;

/**
 * Flying (Body.flight), `k` of the way, laid out `lean` forward and `roll` over to the
 * left - which is the way it goes, so the limbs trail away from it as on a line. Going
 * nowhere, the legs hang a little bent and the arms are out for balance. Forward, the
 * legs trail straight behind, the toes pointed, and the arms sweep back along the
 * sides; backward, the hips lead and the legs and arms trail ahead, bent; sideways,
 * the legs swing out to the side it comes from and the arm on the side it goes to
 * reaches out that way.
 */
export function flightPose(pose, lean, roll, k) {
  const ahead = Math.max(0, Math.min(1, lean / 1.2)), back = Math.max(0, Math.min(1, -lean / 0.8));
  const left = Math.max(-1, Math.min(1, roll / 0.9));
  const ease = (joint, to) => { pose[joint] = (pose[joint] || 0) + (to - (pose[joint] || 0)) * k; };
  for (const [side, by, out] of [['L', 0.06, 1], ['R', -0.06, -1]]) {
    ease(`thigh_${side}`, 0.25 + by - 0.35 * ahead + 0.5 * back);
    ease(`shin_${side}`, -0.5 + 0.3 * ahead - 0.5 * back);
    ease(`foot_${side}`, -0.3 - 0.35 * ahead);
    // Both swung out away from the way it goes.
    ease(`thigh_${side}_spread`, LEG_SPREAD * left);
    // The leading arm: the left one going left, the right going right.
    const leads = Math.max(0, left * out);
    const [forward, spread] = pose[`upperarm_${side}`], [bend] = pose[`forearm_${side}`];
    const toForward = 0.35 - 0.85 * ahead + 0.6 * back, toSpread = 0.5 - 0.2 * ahead + 0.7 * leads - 0.3 * Math.max(0, -left * out);
    pose[`upperarm_${side}`] = [forward + (toForward - forward) * k, spread + (toSpread - spread) * k];
    pose[`forearm_${side}`] = [bend + (Math.max(0.05, 0.5 - 0.3 * ahead + 0.3 * back - 0.3 * leads) - bend) * k, 0];
  }
}

// Swimming in the ring: how far the body leans into it at a swimmer's pace, how much
// further flat out, how far back swimming backward and over to the side sideways; and
// how fast the stroke goes treading water, and how much quicker swimming and flat out.
const SWIM_LEAN = 0.25, SWIM_DASH = 0.3, SWIM_BACK = 0.3, SWIM_ROLL = 0.2;
const SWIM_RATE = { tread: 4, swim: 4, dash: 5 };

/**
 * The way a swimmer going at `velocity` swims, facing `yaw`: how far they are going
 * at a swim (`going`, 0 still to 1 at a swimmer's walking pace), how far on from that
 * to flat out (`hard`, 0 to 1 at a swimmer's run), and which way as the body faces -
 * `ahead` and `across` to its right, together of length 1.
 */
export function swimWay(velocity, yaw) {
  const ahead = -velocity.x * Math.sin(yaw) - velocity.z * Math.cos(yaw);
  const across = velocity.x * Math.cos(yaw) - velocity.z * Math.sin(yaw);
  const level = Math.hypot(ahead, across), swim = WALK * SWIM_PACE, dash = RUN * SWIM_PACE;
  return {
    going: Math.min(1, level / swim),
    hard: Math.max(0, Math.min(1, (level - swim) / (dash - swim))),
    ahead: level > 1e-3 ? ahead / level : 1,
    across: level > 1e-3 ? across / level : 0,
  };
}

/**
 * Swimming in the ring (swimWay's `way`), `k` of the way, `phase` into the stroke. Still,
 * treading water: the legs pedalling round under the body, the hands sculling on the
 * ring. Forward, a flutter kick from the hips behind and the hands paddling in turn
 * before the ring - flat out, a harder kick and the arms going over the ring in a
 * crawl. Backward, the legs pedalling ahead and the hands pushing the water forward at
 * the sides; sideways, the legs swept together away from the way it goes and the arm on
 * that side reaching out and pulling.
 */
export function swimPose(pose, phase, { going = 0, hard = 0, ahead = 1, across = 0 } = {}, k = 1) {
  const ease = (joint, to) => { pose[joint] = (pose[joint] || 0) + (to - (pose[joint] || 0)) * k; };
  // How much of it is each: still, forward, back and to the side, adding up to 1.
  const fore = Math.max(0, ahead), back = Math.max(0, -ahead), side = Math.abs(across), all = fore + back + side || 1;
  const still = 1 - going, f = (going * fore) / all, b = (going * back) / all, s = (going * side) / all;
  const kick = 0.2 + 0.25 * hard, sweep = Math.sin(phase);
  for (const [name, at, out] of [['L', 0, 1], ['R', Math.PI, -1]]) {
    const beat = Math.sin(phase + at), round = Math.cos(phase + at);
    ease(`thigh_${name}`, still * (0.55 + 0.15 * beat) + f * (0.1 + beat * kick) + b * (0.9 + 0.3 * beat) + s * (0.35 + 0.1 * beat));
    ease(`shin_${name}`, still * (-0.9 + 0.3 * round) + f * (-0.25 - Math.max(0, -beat) * (0.35 + 0.2 * hard)) + b * (-1.1 + 0.35 * round) - s * 0.5);
    ease(`foot_${name}`, -0.3 * still - 0.6 * f - 0.2 * b - 0.4 * s);
    ease(`thigh_${name}_spread`, -s * Math.sign(across) * LEG_SPREAD * (0.6 + 0.4 * sweep));
    // The arms: on the ring; paddling, or a crawl's stroke, half as quick as the kick,
    // over the top and bent on the way back; pushing at the sides; the leading one out.
    const stroke = phase / 2 + at, pull = Math.sin(stroke), recovering = Math.max(0, Math.cos(stroke));
    const leads = Math.max(0, -Math.sign(across) * out);
    const resting = still + s * (1 - leads), paddle = f * (1 - hard), crawl = f * hard;
    const forward = resting * (0.45 + 0.08 * round) + paddle * (0.95 + 0.35 * beat) + crawl * (1.3 + 1.5 * pull) + b * (0.2 + 0.5 * beat) + s * leads * 0.5;
    const spread = resting * 0.75 + paddle * 0.3 + crawl * (0.3 + 0.25 * recovering) + b * 0.6 + s * leads * (1.2 + 0.4 * beat);
    const bend = resting * (0.9 + 0.1 * beat) + paddle * (0.9 - 0.3 * round) + crawl * (0.25 + 0.9 * recovering) + b * (0.5 + 0.3 * round) + s * leads * (0.4 + 0.4 * round);
    const [was, wasOut] = pose[`upperarm_${name}`], [wasBend] = pose[`forearm_${name}`];
    pose[`upperarm_${name}`] = [was + (forward - was) * k, wasOut + (spread - wasOut) * k];
    pose[`forearm_${name}`] = [wasBend + (bend - wasBend) * k, 0];
  }
}

// Going under with nothing to float on (drownPose): how soon the swim stance it starts
// in is taken up, as a share of the way under; how much further down than swimming the
// body is at the end, the head under; and how far it slumps forward, limp.
const DROWN_IN = 5, DROWN_DEPTH = 0.3, DROWN_LEAN = 0.35;

/**
 * Going under (walk.js drown), `progress` of the way (0, just in, to 1, gone), `phase`
 * into the stroke, over the treading of swimPose. First the fight - the arms thrashing
 * up over the head in turn, the legs pedalling hard, the head thrown back for air -
 * and then, as it goes out of them, limp: the arms drifting up, the legs hanging, the
 * chin down. Where the page asks for reduced motion, nothing thrashes.
 */
export function drownPose(pose, phase, progress, reduced = false) {
  const k = Math.min(1, progress * DROWN_IN), limp = progress * progress * (3 - 2 * progress), fight = (1 - limp) * (reduced ? 0 : 1);
  const ease = (joint, to) => { pose[joint] = (pose[joint] || 0) + (to - (pose[joint] || 0)) * k; };
  for (const [side, at] of [['L', 0], ['R', Math.PI]]) {
    const thrash = Math.sin(phase * 1.5 + at), reach = Math.cos(phase * 1.5 + at), pedal = Math.sin(phase * 2 + at), drift = Math.sin(phase / 2 + at);
    ease(`thigh_${side}`, (1 - limp) * (0.7 + 0.35 * pedal * fight) + limp * (side === 'L' ? 0.35 : 0.2));
    ease(`shin_${side}`, (1 - limp) * (-1 + 0.4 * Math.cos(phase * 2 + at) * fight) + limp * -0.55);
    ease(`foot_${side}`, -0.2 * (1 - limp) - 0.45 * limp);
    const [forward, out] = pose[`upperarm_${side}`], [bend] = pose[`forearm_${side}`];
    const toForward = (1 - limp) * (2.3 + 0.6 * thrash * fight) + limp * (2.3 + 0.1 * drift * (reduced ? 0 : 1));
    const toOut = (1 - limp) * (0.6 + 0.3 * reach * fight) + limp * 0.5;
    const toBend = (1 - limp) * (0.5 + 0.4 * Math.max(0, thrash) * fight) + limp * 0.6;
    pose[`upperarm_${side}`] = [forward + (toForward - forward) * k, out + (toOut - out) * k];
    pose[`forearm_${side}`] = [bend + (toBend - bend) * k, 0];
  }
  // The head back for air while there is fight, the chin on the chest once there is not.
  pose.head = (pose.head || 0) + ((1 - limp) * 0.5 - limp * 0.4 - (pose.head || 0)) * k;
}

// How far each joint bends forward from its rest, in the pose's angles.
const JOINT_RANGES = { thigh: [-0.8, 2.2], shin: [-2.4, 0], upperarm: [-1, 3], forearm: [0, 2.5] };

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
 * they run - out for balance in the air (`air` of the way), forward against a landing,
 * up a ladder hand over hand, holding on to a ride as far as they are sat on it
 * (`sit`), and one thrown forward and the other back through a kick (`kick`, 0 to 1
 * and back).
 */
function arms(pose, w, phase, sit, kick, gait, air = w.p.ground ? 0 : 1, now = performance.now()) {
  const p = w.p, r = w.riding, ride = r?.entry.ride, pace = Math.min(1.6, gait);
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
  const run = running(w, gait) * (1 - sit), swing = Math.sin(phase) * (0.3 + 0.1 * run) * Math.min(1.3, pace) * (1 - sit);
  const forward = REST_ARM[0] + (0.05 - REST_ARM[0]) * run, bent = REST_ARM[1] + (1.3 - REST_ARM[1]) * run;
  set('L', forward - swing, bent + Math.max(0, -swing) * 0.3, REST_ARM[2]);
  set('R', forward + swing, bent + Math.max(0, swing) * 0.3, REST_ARM[2]);
  pose.twist = -swing * 0.25;
  if (air > 0 && !r && sit === 0) {
    // Out for balance in the air: swung up with the push off the ground, wide coming
    // down; under a jet, as they hang.
    const up = p.fly ? 0 : Math.max(0, Math.min(1, (p.vy || 0) / JUMP));
    for (const side of ['L', 'R']) towards(side, 0.35 + 0.5 * up, 0.5 - 0.05 * up, 0.5 - 0.2 * up, air);
  } else if (ride === 'slide' && r.s < r.marks.top) {
    // Reaching up the ladder, until the hands have a rung (rideGrips) - and at its top,
    // where there is none left, no higher than the shoulders.
    const up = Math.sin(phase * 1.3);
    set('L', 1.3 + up * 0.2, 0.5);
    set('R', 1.3 - up * 0.2, 0.5);
  }
  const landed = r ? 0 : landing(w, now);
  if (landed > 0) for (const side of ['L', 'R']) towards(side, 0.3, 0.45, 0.4, landed);
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
