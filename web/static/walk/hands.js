// The walker's hands, as a model rather than as a pile of boxes.
//
// The hand is prepared by scripts/hand.py: the MIT-licensed `generic-hand` from the
// WebXR Input Profiles project, turned to the axes used here, given the forearm a VR
// hand has no need of, and re-rigged so that a finger carries its own tip when it
// curls. scripts/body.py exports it in body.glb, whole as `hand_rig`, and cut down to
// scale as the body's own hands (body.js) - one hand, seen held before the eye or at
// the end of the walker's arms. It is loaded once and every hand the UI draws is a
// clone sharing that geometry, so a second hand costs a skeleton and nothing else.
//
// The rig is what makes it worth loading a model at all: the fingers close around
// whatever the walker is holding, and the wrist and forearm turn so the arm leaves the
// frame at its corner however the tool is angled. Posing is by name (pose), and the
// names are the WebXR joint names, which is the contract between this and scripts/hand.py.

import * as THREE from '../vendor/three.module.min.js';
import { clone as cloneRigged } from '../vendor/SkeletonUtils.js';
import { CLOTH, layer } from './cloth.js';
import { loadBody } from './bodymodel.js';

// The joints that bend. WebXR gives a finger a metacarpal inside the palm as well,
// but that one is part of the hand's shape rather than part of closing it.
// Implements: REQ-TOOL-009
const FINGERS = ['index', 'middle', 'ring', 'pinky'].map(d =>
  ['proximal', 'intermediate', 'distal'].map(j => `${d}-finger-phalanx-${j}`));
const THUMB = ['thumb-metacarpal', 'thumb-phalanx-proximal', 'thumb-phalanx-distal'];

let model = null;    // the loaded hand, shared by every clone

/**
 * Starts loading the model, and resolves once it is in. Calling it again while it is
 * loading joins the same load. The UI can go on drawing without it: a hand that has
 * not arrived yet simply is not there, which is a frame or two at startup.
 *
 * Implements: REQ-TOOL-010
 */
export function loadHands() {
  return loadBody().then(scene => {
    model = scene?.getObjectByName('hand_rig') || null;
    return model;
  });
}

export const handsReady = () => model !== null;

/** A bare hand's color. */
export const SKIN = '#c98d63';

// What the arms wear in each of the map's styles: bare, with a T-shirt's sleeve above
// the elbow, in a city; an electrician's coverall sleeves and insulating gloves with
// gauntlets over the cuffs on a board; a spacesuit's sleeves and gloves, a ring of light
// at the wrist, in the galaxy. Each is a layer cut from the arm's own mesh (cloth.js),
// along z in the bind pose, where scripts/hand.py stands the wrist on the origin with
// the fingers along +z and the arm back along -z.
// Implements: REQ-WALK-059
const OUTFITS = {
  city: [{ to: -0.3, inflate: 0.004, color: '#2a9d8f', kind: CLOTH.knit }],
  circuit: [
    { to: -0.035, inflate: 0.0035, color: '#24365a', kind: CLOTH.twill },
    { from: -0.012, inflate: 0.0018, color: '#d9772b', kind: CLOTH.rubber },
    { from: -0.05, to: -0.012, inflate: 0.0048, color: '#c46a24', kind: CLOTH.rubber },
  ],
  galaxy: [
    { to: -0.02, inflate: 0.005, color: '#eef0f4', kind: CLOTH.suit },
    { from: -0.03, inflate: 0.0025, color: '#d6d9e2', kind: CLOTH.rubber },
    { from: -0.026, to: -0.018, inflate: 0.0065, color: '#6ff4ff', kind: CLOTH.rubber },
  ],
};

let worn = 'city';

/** Dresses the hands made from now on for the map's `style`. */
export function wear(style) { worn = OUTFITS[style] ? style : 'city'; }

/** What an arm dressed for `style` is the color of, `z` along it from the wrist (+z the fingers): its outermost layer's, or the skin's. */
export function outfitAt(style, z) {
  let top = null;
  for (const l of OUTFITS[style] || OUTFITS.city) {
    if (z >= (l.from ?? -Infinity) && z <= (l.to ?? Infinity) && (!top || l.inflate > top.inflate)) top = l;
  }
  return top ? top.color : SKIN;
}

// The bare arm, skin all over, and each style's layers over it: made once a geometry.
const bare = new Map(), layers = new Map();
function skinOf(geometry) {
  if (!bare.has(geometry)) {
    const out = geometry.clone(), n = out.getAttribute('position').count, c = new THREE.Color(SKIN);
    out.setAttribute('color', new THREE.Float32BufferAttribute(Array.from({ length: n }, () => [c.r, c.g, c.b]).flat(), 3));
    out.setAttribute('cloth', new THREE.Float32BufferAttribute(new Array(n).fill(CLOTH.skin), 1));
    bare.set(geometry, out);
  }
  return bare.get(geometry);
}
function layersOf(geometry, style) {
  const key = `${geometry.uuid}/${style}`;
  if (!layers.has(key)) layers.set(key, OUTFITS[style].map(l => layer(geometry, l)));
  return layers.get(key);
}

/**
 * One hand, mirrored for the left. Returns a group whose origin is the wrist, with the
 * fingers along +z, the back of the hand up and the arm running back along -z, and
 * whose userData.bones maps the rig's names to their bones and rest orientations.
 *
 * The model is a right hand; a left hand is the same mesh mirrored across x, which is
 * what a left hand is. Mirroring reverses the winding, so its material draws back
 * faces instead - otherwise the hand turns inside out.
 *
 * Implements: REQ-TOOL-010
 */
export function handModel(mirror = 1, material) {
  if (!model) return null;
  const g = new THREE.Group();
  const h = cloneRigged(model);
  h.scale.x = mirror;
  if (mirror < 0) material.side = THREE.BackSide;
  const bones = new Map();
  material.vertexColors = true;
  material.color?.set('#ffffff');
  const skins = [];
  h.traverse(o => {
    if (o.isSkinnedMesh) skins.push(o);
    if (o.isMesh) {
      o.material = material;
      o.frustumCulled = false;
    }
    if (o.isBone) bones.set(o.name, { bone: o, rest: o.quaternion.clone() });
  });
  // What the walker wears over the arm, skinned to the same bones.
  for (const o of skins) {
    const source = o.geometry;
    o.geometry = skinOf(source);
    for (const geometry of layersOf(source, worn)) {
      const cover = new THREE.SkinnedMesh(geometry, material);
      cover.position.copy(o.position);
      cover.quaternion.copy(o.quaternion);
      cover.scale.copy(o.scale);
      cover.bind(o.skeleton, o.bindMatrix);
      cover.frustumCulled = false;
      o.parent.add(cover);
    }
  }
  g.add(h);
  g.userData.bones = bones;
  g.userData.mirror = mirror;
  return g;
}

const axis = new THREE.Vector3();
const turn = new THREE.Quaternion();

/**
 * Turns one bone by an angle about one of its own axes, from its rest pose - after
 * fanning it `fan` out of the back of the hand. scripts/hand.py rolls every bone so
 * that its local x runs across the hand and its local z points out of the back of it,
 * which is why one axis curls every finger the same way.
 *
 * Implements: REQ-TOOL-009
 */
export function pose(hand, name, about, angle, fan = 0) {
  const joint = hand?.userData.bones?.get(name);
  if (!joint) return;
  joint.bone.quaternion.copy(joint.rest);
  if (fan) joint.bone.quaternion.multiply(turn.setFromAxisAngle(axis.set(0, 0, 1), fan));
  axis.set(about === 'x' ? 1 : 0, about === 'y' ? 1 : 0, about === 'z' ? 1 : 0);
  joint.bone.quaternion.multiply(turn.setFromAxisAngle(axis, angle));
}

// How far each joint of a finger gives when the hand closes. A hand does not fold
// evenly: the knuckle gives least and the middle joint most, which is the difference
// between a fist and a claw.
// Implements: REQ-TOOL-011
const GIVE = [0.75, 1.25, 0.95];

// Across the hand: fingers curl about it, and fan out of the back of it (pose).
const CURL = 'x';

/**
 * Closes every finger: 0 is the model's open rest pose, 1 a fist. `spread` opens the
 * hand out again at the knuckles, which is what a hand does as it lets something go.
 */
export function closeHand(hand, amount, spread = 0) {
  if (!hand) return;
  for (let i = 0; i < FINGERS.length; i++) closeFinger(hand, i, amount, spread * (i - 1.5) * 0.12);
  // The thumb comes across rather than curling under.
  pose(hand, THUMB[0], CURL, -amount * 0.35);
  pose(hand, THUMB[1], CURL, -amount * 0.5);
  pose(hand, THUMB[2], CURL, -amount * 0.4);
}

/**
 * One finger on its own, 0 straight and 1 curled, which is what a trigger finger is:
 * the rest of the hand holds the thing while the index lies along it and presses.
 * Call it after closeHand, which closes this one too; `fan` turns it out at the knuckle.
 */
export function closeFinger(hand, i, amount, fan = 0) {
  for (let j = 0; j < 3; j++) pose(hand, FINGERS[i][j], CURL, -amount * GIVE[j], j ? 0 : fan);
}

/** Bends the wrist and turns the forearm; both are bones like any other. */
export function setWrist(hand, bend, twist) {
  pose(hand, 'wrist', CURL, bend);
  pose(hand, 'forearm', 'y', twist);
}
