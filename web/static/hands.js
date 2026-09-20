// The walker's hands, as a model rather than as a pile of boxes.
//
// hand.glb is prepared by tools/hand.py: the MIT-licensed `generic-hand` from the
// WebXR Input Profiles project, turned to the axes used here, scaled to map units,
// given the forearm a VR hand has no need of, and re-rigged so that a finger carries
// its own tip when it curls. It is loaded once and every hand the UI draws is a clone
// sharing that geometry, so a second hand costs a skeleton and nothing else.
//
// The rig is what makes it worth loading a model at all: the fingers close around
// whatever the walker is holding, and the wrist and forearm turn so the arm leaves the
// frame at its corner however the tool is angled. Posing is by name (pose), and the
// names are the WebXR joint names, which is the contract between this and tools/hand.py.

import * as THREE from './vendor/three.module.min.js';
import { GLTFLoader } from './vendor/GLTFLoader.js';
import { clone as cloneRigged } from './vendor/SkeletonUtils.js';
import { STATIC } from './data.js';

// The joints that bend. WebXR gives a finger a metacarpal inside the palm as well,
// but that one is part of the hand's shape rather than part of closing it.
export const FINGERS = ['index', 'middle', 'ring', 'pinky'].map(d =>
  ['proximal', 'intermediate', 'distal'].map(j => `${d}-finger-phalanx-${j}`));
export const THUMB = ['thumb-metacarpal', 'thumb-phalanx-proximal', 'thumb-phalanx-distal'];

let model = null;    // the loaded scene, shared by every clone
let loading = null;  // the load in flight

/**
 * Starts loading the model, and resolves once it is in. Calling it again while it is
 * loading joins the same load. The UI can go on drawing without it: a hand that has
 * not arrived yet simply is not there, which is a frame or two at startup.
 */
export function loadHands() {
  // A static export has no server to fetch from and carries the model inline.
  loading ||= new GLTFLoader().loadAsync(STATIC?.hand || 'hand.glb').then(gltf => {
    model = gltf.scene;
    model.updateMatrixWorld(true);
    return model;
  }).catch(err => {
    console.error('hand model:', err);
    return null;
  });
  return loading;
}

export const handsReady = () => model !== null;

/**
 * One hand, mirrored for the left. Returns a group whose origin is the wrist, with the
 * fingers along +z, the back of the hand up and the arm running back along -z, and
 * whose userData.bones maps the rig's names to their bones and rest orientations.
 *
 * The model is a right hand; a left hand is the same mesh mirrored across x, which is
 * what a left hand is. Mirroring reverses the winding, so its material draws back
 * faces instead - otherwise the hand turns inside out.
 */
export function handModel(mirror = 1, material) {
  if (!model) return null;
  const g = new THREE.Group();
  const h = cloneRigged(model);
  h.scale.x = mirror;
  if (mirror < 0) material.side = THREE.BackSide;
  const bones = new Map();
  h.traverse(o => {
    if (o.isMesh) {
      o.material = material;
      o.frustumCulled = false;
    }
    if (o.isBone) bones.set(o.name, { bone: o, rest: o.quaternion.clone() });
  });
  g.add(h);
  g.userData.bones = bones;
  g.userData.mirror = mirror;
  return g;
}

const axis = new THREE.Vector3();
const turn = new THREE.Quaternion();

/**
 * Turns one bone by an angle about one of its own axes, from its rest pose. tools/hand.py
 * rolls every bone so that its local x runs across the hand and its local z points out
 * of the back of it, which is why one axis curls every finger the same way.
 */
export function pose(hand, name, about, angle) {
  const joint = hand?.userData.bones?.get(name);
  if (!joint) return;
  axis.set(about === 'x' ? 1 : 0, about === 'y' ? 1 : 0, about === 'z' ? 1 : 0);
  joint.bone.quaternion.copy(joint.rest).multiply(turn.setFromAxisAngle(axis, angle));
}

// How far each joint of a finger gives when the hand closes. A hand does not fold
// evenly: the knuckle gives least and the middle joint most, which is the difference
// between a fist and a claw.
const GIVE = [0.75, 1.25, 0.95];

// Across the hand, and out of the back of it: fingers curl about the first and fan
// about the second.
const CURL = 'x', SPREAD = 'z';

/**
 * Closes every finger: 0 is the model's open rest pose, 1 a fist. `spread` opens the
 * hand out again at the knuckles, which is what a hand does as it lets something go.
 */
export function closeHand(hand, amount, spread = 0) {
  if (!hand) return;
  for (let i = 0; i < FINGERS.length; i++) {
    closeFinger(hand, i, amount);
    pose(hand, FINGERS[i][0], SPREAD, spread * (i - 1.5) * 0.12);
  }
  // The thumb comes across rather than curling under.
  pose(hand, THUMB[0], CURL, -amount * 0.35);
  pose(hand, THUMB[1], CURL, -amount * 0.5);
  pose(hand, THUMB[2], CURL, -amount * 0.4);
}

/**
 * One finger on its own, 0 straight and 1 curled, which is what a trigger finger is:
 * the rest of the hand holds the thing while the index lies along it and presses.
 * Call it after closeHand, which closes this one too.
 */
export function closeFinger(hand, i, amount) {
  for (let j = 0; j < 3; j++) pose(hand, FINGERS[i][j], CURL, -amount * GIVE[j]);
}

/** Bends the wrist and turns the forearm; both are bones like any other. */
export function setWrist(hand, bend, twist) {
  pose(hand, 'wrist', CURL, bend);
  pose(hand, 'forearm', 'y', twist);
}
