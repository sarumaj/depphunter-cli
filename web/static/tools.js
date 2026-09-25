// What walk mode puts in your hands. A tool is a viewmodel - a hand and the thing it
// holds, drawn in front of the camera - an animation for using it, and whatever
// travels to the target. Tagging a module is the same act throughout; only the
// gesture changes, so none of this touches the map or the graph.
//
// Tools come in two kinds, and the difference is what they are for. A primary tool is
// the hunt: it tags a module or catches a bug, and everything about it - its reach,
// its flight, its reticle - is about reaching one of those. A secondary tool touches
// neither. It carries the walker instead: a line to a wall, a jet to fly on, floats to
// cross the water with.
//
// One of each is carried at a time, one to a hand: the primary in the right and the
// secondary in the left, so a walker can fly over the map and net what they find
// without putting either down. Nothing a secondary tool does can tag or catch, which
// is what hits() below is for, and the HUD keeps the two rows apart.
//
// The hand and the forearm are a rigged model (hands.js, modelled by scripts/hand.py in
// Blender); the tools themselves are built here. Both are lit, which nothing else on
// the map is: the walk camera carries its own lights, and they reach only what it
// holds, because every other material in the scene is unlit.
//
// Each tool provides:
//   viewmodel(scene)  the group parented to the walk camera, posed at rest
//   pose(vm, u, now)  u through the swing, 0..1
//   projectile(scene) what flies, or null when the tool acts where it is pointed. It
//                     is built out of `flying` rather than `part`, because it ends up
//                     in the scene rather than in the hands.
//   reticle           the aim helper's style, so it fits the tool rather than an
//                     ever-present crosshair
//   verb / noun       what the HUD and its messages call the act and its tally
//   kind              'primary' (it hunts) or 'secondary' (it carries you)
//   catchAs           how a bug it caught leaves the map (bugs.js): reeled in,
//                     scooped, bubbled, foamed, pinned or blinked out. It is the
//                     tool's own gesture carried through to the thing it acted on.
//   targets           'bugs', 'buildings' or 'both': what it is any use against.
//                     A secondary tool has none, whatever it says: hits() below is
//                     what decides, and it never answers yes for one.
//   reach             how far it works, in map units. Every tool has one but the
//                     camera, which photographs whatever is in the frame however far
//                     away it is. A reach with nothing thrown is a tool swung by hand
//                     - the net - which has to be walked up to; a reach on a tool
//                     that throws is how far what it throws carries, and past it the
//                     reticle dims, the shot still leaves and it falls short.
//   auto              seconds between shots while the trigger is held down. A tool
//                     without it fires once per click, which is most of them.
//   flight            how what it throws behaves: speed, how much it is lobbed
//                     (arc), what gravity does to it, and how the air holds it back.
//                     A dart is fast and flat, a bubble slow and rising; the same
//                     code flies both.
//   reel              what happens to the line once it has stuck: how fast it pulls
//                     the walker along it, how close it brings them, how far away it
//                     will still pull from, and whether it sets them on top of what
//                     it caught or leaves them against it.
//   flies / floats    what carrying a secondary tool does to the walker: the jet
//                     backpack holds them in the air, the skimmers hold them on the
//                     water. walk.js reads both off whatever is in hand.
//   fuel              how long it will do it for, and how long it takes to come back:
//                     {full} seconds of use, {fills} seconds to refill from empty
//                     while it is not being used. A tool without it never runs out.

import * as THREE from './vendor/three.module.min.js';

import { handModel, closeHand, closeFinger, setWrist, loadHands, handsReady } from './hands.js';

/**
 * One part of a tool. Unlike everything else on the map these are lit: the walk
 * camera carries its own lights (viewLights), and since every material in the scene
 * proper is unlit, nothing but what is in the walker's hands can see them. That is
 * what gives a rod blank its highlight and a hand its roundness.
 *
 * Implements: REQ-TOOL-012
 */
function part(geo, color, opts) {
  return new THREE.Mesh(geo, new THREE.MeshPhongMaterial({
    color, shininess: 22, specular: 0x1b1b1b, ...opts,
  }));
}

/**
 * One part of something in flight, which is a different matter entirely: what leaves
 * the hand is out in the scene, and the scene is unlit and bent around the planet.
 * So these are unlit - a lit material out there sees no light at all and is drawn
 * black, which is what a soap bubble and a gout of white foam made very plain - and
 * bendable, so a dart in the air follows the same curve as the street under it.
 *
 * Implements: REQ-TOOL-039, REQ-TOOL-040
 */
function flying(scene, geo, color, opts) {
  return new THREE.Mesh(geo, scene.bendable(new THREE.MeshBasicMaterial({ color, ...opts })));
}

/** A tube between two points, for anything in flight that has a shaft. */
function flyingRod(scene, r0, r1, len, color) {
  return flying(scene, new THREE.CylinderGeometry(r0, r1, len, 12), color);
}

// The camera's screen, which is what a photograph put up on it has to be fitted to.
const SCREEN_W = 0.135, SCREEN_H = 0.097;

// The butterfly net's mouth and how deep its bag hangs.
const HOOP = 0.145, BAG = 0.34;
// The bubble wand's ring, which is built the same way the net's hoop is: standing in
// the line of the stick with its rim on the tip, rather than threaded onto it.
const RING = 0.062;

/**
 * How wide the bag is at a depth: the hoop's width at the mouth, drawn in to a point
 * at the bottom, with the belly a hanging net has in the middle of it.
 */
const bagAt = (t, r) => r * Math.cos(t * Math.PI / 2) ** 0.7 * (1 + 0.14 * Math.sin(t * Math.PI));

/**
 * A net bag on a hoop: a haze of a surface with the strands and rings that make it
 * drawn over the top.
 *
 * Netting is mostly holes, and what the eye reads is the mesh rather than the cloth -
 * so a smooth translucent cone, which is what this was, reads as a plastic bag. The
 * other way to do it is a texture with holes in it, and there are no textures on this
 * map; lines are what is left, and they are what netting is anyway.
 */
function bagOf(parent, r, depth) {
  const rings = [];
  for (let i = 0; i <= 10; i++) {
    const t = i / 10;
    rings.push(new THREE.Vector2(Math.max(0.0015, bagAt(t, r)), t * depth));
  }
  const cloth = part(new THREE.LatheGeometry(rings, 18), '#eef3f8',
    { transparent: true, opacity: 0.14, side: THREE.DoubleSide, depthWrite: false });
  parent.add(cloth);

  const at = (t, a) => [Math.cos(a) * bagAt(t, r), t * depth, Math.sin(a) * bagAt(t, r)];
  const p = [];
  for (let s = 0; s < 14; s++) { // down the bag
    const a = (s / 14) * Math.PI * 2;
    for (let i = 0; i < 6; i++) p.push(...at(i / 6, a), ...at((i + 1) / 6, a));
  }
  for (const t of [0.22, 0.45, 0.68]) { // and round it
    for (let i = 0; i < 18; i++) {
      p.push(...at(t, (i / 18) * Math.PI * 2), ...at(t, ((i + 1) / 18) * Math.PI * 2));
    }
  }
  const mesh = new THREE.BufferGeometry();
  mesh.setAttribute('position', new THREE.Float32BufferAttribute(p, 3));
  parent.add(new THREE.LineSegments(mesh,
    new THREE.LineBasicMaterial({ color: '#f4f7fb', transparent: true, opacity: 0.8 })));
  return cloth;
}

// A tube between two points, for rod blanks, handles and strap runs.
function rodPart(r0, r1, len, color) {
  return part(new THREE.CylinderGeometry(r0, r1, len, 12), color);
}

// The axis a cylinder is built along, for linkPart below.
const ALONG = new THREE.Vector3(0, 1, 0);

/**
 * A tube from one point to another, both given: it is as long as the gap and pointed
 * down it, and `userData.len` says what that came to, for anything to be hung along
 * it.
 *
 * The alternative is a tube of a chosen length at a chosen angle, aimed at whatever it
 * is meant to reach - and that is how a tool ends up with something floating beside
 * it, because the two ends are set independently and only one of them is ever checked.
 * Anything that has to arrive somewhere exactly is built this way instead.
 *
 * Implements: REQ-TOOL-037
 */
function linkPart(from, to, r, color) {
  const run = new THREE.Vector3().subVectors(to, from);
  const len = run.length();
  const tube = rodPart(r, r, len, color);
  tube.position.copy(from).addScaledVector(run, 0.5);
  tube.quaternion.setFromUnitVectors(ALONG, run.normalize());
  tube.userData.len = len;
  return tube;
}

// Skin, and the sleeve the arm comes out of.
const SKIN = '#c98d63';

// Both faces, because a hand arrives after the viewmodel holding it has been built and
// may by then be in the mirrored one walk.js hangs in the off hand. A closed surface
// costs nothing for not being culled, and the alternative is a hand that turns itself
// inside out a frame or two after it loads.
const skinMaterial = () =>
  new THREE.MeshPhongMaterial({ color: SKIN, shininess: 8, specular: 0x141414, side: THREE.DoubleSide });

/**
 * The lights the walk camera carries for its own hands: a key over the left shoulder,
 * a dim fill from the other side, a rim behind to pick the arm out of whatever is
 * behind it, and sky above and ground below instead of a flat ambient - which is what
 * stops the shaded side of a forearm from going to one dead tone. They are parented
 * to the camera, so they travel with the view, and they reach nothing else, because
 * the map is drawn with unlit materials.
 *
 * Implements: REQ-TOOL-012
 */
export function viewLights() {
  const g = new THREE.Group();
  const key = new THREE.DirectionalLight(0xfff4e6, 1.85);
  key.position.set(-0.5, 0.9, 0.6);
  const fill = new THREE.DirectionalLight(0xbcd2ea, 0.6);
  fill.position.set(0.8, -0.1, 0.35);
  // From behind and above: a thin bright edge along the top of the arm, which is what
  // a hand in front of a bright sky actually looks like.
  const rim = new THREE.DirectionalLight(0xffffff, 0.75);
  rim.position.set(0.25, 0.7, -1);
  g.add(key, fill, rim, new THREE.HemisphereLight(0xdceaff, 0x6a5a4a, 0.85));
  return g;
}

// ------------------------------------------------------------------ the viewmodel

const REST = { x: 0.235, y: -0.2, z: -0.55, rx: 0.1, ry: -0.25, rz: 0 };

// How a tool sits in the fist, for the two shapes that repeat.
//
// SWUNG is anything held across the palm - a rod, a net, a wand - whose shaft lies
// along the arm. Each still places its own hold: what the arm is swinging changes
// where it comes into the frame.
//
// PISTOL is a grip hanging down out of the fist, so the shaft through it points up,
// pointed down the view a few degrees off it. Held square across the frame the barrel
// aims sixteen degrees wide of the reticle and what it fires leaves sideways; end-on
// it would be a dark blob instead. These angles put the barrel eleven degrees off the
// aim with the scope on top, solved from the pose rather than guessed at, so the
// flank still reads.
const SWUNG = { x: 0, y: 0, z: 0, rx: 0, ry: 0, rz: -Math.PI / 2 };
const PISTOL = {
  hold: { x: 0.19, y: -0.24, z: -0.5, along: [0.05, 1, 0.1], back: [0.45, -0.3, 1] },
  grip: { x: 0, y: 0, z: 0, rx: -3.124, ry: 0.117, rz: -1.599 },
  restGrip: 0.88,
};

// viewmodel places a hand and its tool where a first-person view expects them: low
// and to the right, angled towards the middle of the screen.
// Implements: REQ-TOOL-001
function viewmodel(build) {
  const g = new THREE.Group();
  g.position.set(REST.x, REST.y, REST.z);
  g.rotation.set(REST.rx, REST.ry, REST.rz);
  g.userData.hands = [];
  build(g);
  g.traverse(o => { o.frustumCulled = false; o.renderOrder = 10; });
  g.userData.restY = REST.y;
  return g;
}

/**
 * A hand, and the tool it holds. `hold` places the wrist in front of the camera and
 * points the arm back out of the frame; `grip` places the tool inside that hand, so
 * its shaft runs through the fist. The tool is a child of the hand: the two move
 * together, which is the whole reason for holding one.
 *
 * The model arrives asynchronously, so the hand is an empty anchor until it does. One
 * or two frames at the start of a session is the price of not blocking on it.
 *
 * Implements: REQ-TOOL-013
 */
function armed(g, { hold, grip, restGrip = 0.75 }, build, mirror = 1) {
  const holder = new THREE.Group();
  holder.position.set(hold.x, hold.y, hold.z);
  aimHand(holder, hold.along, hold.back);
  g.add(holder);
  g.userData.hands.push(holder);
  g.userData.restGrip = restGrip;
  holder.userData.restGrip = restGrip;
  fillHand(holder, mirror, restGrip);
  if (build) {
    // Two groups, not one: the outer holds the tool where the fist has it, and the
    // inner is what a gesture turns. A gesture writes absolute angles - it is easier
    // to read a swing written that way - and with one group those angles landed on
    // top of the grip and the tool never came back upright after a shot.
    const gripped = new THREE.Group();
    gripped.position.set(grip.x, grip.y, grip.z);
    gripped.rotation.set(grip.rx, grip.ry, grip.rz);
    holder.add(gripped);
    const tool = new THREE.Group();
    gripped.add(tool);
    build(tool);
    return tool;
  }
  return holder;
}

/**
 * The other way round: a hand laid on a tool that is already in the frame, rather
 * than a tool laid in a hand. `at` is the point the fist closes around, `along` the
 * shaft through it and `back` the way the arm leaves - all in the tool's own frame,
 * which is where anyone looking at the model would measure them. It is what a tool
 * held in both hands needs, because only one hand can be the one holding it.
 */
function grasps(g, tool, { at, along, back, close = 0.75 }, mirror = 1) {
  const holder = new THREE.Group();
  holder.position.set(...at);
  aimHand(holder, along, back);
  tool.add(holder);
  g.userData.hands.push(holder);
  holder.userData.restGrip = close;
  fillHand(holder, mirror, close);
  return holder;
}

/**
 * Turns a hand so that a shaft through its fist runs `along` and its forearm runs
 * `back` out of the frame. Those two are what anyone would say about a held tool -
 * which way it points and which way the arm goes - and they are perpendicular by
 * construction, so `back` is squared up against `along` rather than trusted.
 *
 * Implements: REQ-TOOL-013
 */
function aimHand(holder, along, back) {
  const x = new THREE.Vector3(...along).normalize();
  const z = new THREE.Vector3(...back).normalize().negate();
  z.addScaledVector(x, -z.dot(x)).normalize();
  const y = new THREE.Vector3().crossVectors(z, x);
  holder.quaternion.setFromRotationMatrix(new THREE.Matrix4().makeBasis(x, y, z));
}

// Where a held shaft runs through the model: down the palm, under the knuckles. The
// hand is offset by it so that an anchor's origin is the middle of the fist rather
// than the wrist, which is what `hold` and `grip` are both written against.
const FIST = new THREE.Vector3(0, -0.028, 0.102);

// Puts the model into an anchor, now or as soon as it has loaded.
function fillHand(holder, mirror, restGrip) {
  const put = () => {
    const h = handModel(mirror, skinMaterial());
    if (!h) return;
    h.position.set(-FIST.x * mirror, -FIST.y, -FIST.z);
    holder.add(h);
    holder.userData.hand = h;
    holder.traverse(o => { o.frustumCulled = false; o.renderOrder = 10; });
    closeHand(h, restGrip);
  };
  if (handsReady()) put(); else loadHands().then(put);
}

/**
 * Closes every hand of a viewmodel further than it already is. The model's own pose is
 * an open hand; a tool is held with the fist already most of the way shut, and a
 * gesture takes it the rest of the way.
 *
 * Implements: REQ-TOOL-019
 */
function grip(vm, amount, press = 0) {
  for (const holder of vm.userData.hands || []) {
    const rest = holder.userData.restGrip ?? vm.userData.restGrip ?? 0.75;
    closeHand(holder.userData.hand, rest + amount * (1 - rest));
  }
  // A tool with a button on it lays its index finger back out afterwards, because
  // closing the fist closed that one too, and presses it as the gesture lands.
  const trigger = vm.userData.trigger;
  if (trigger != null) finger(vm, trigger + press * (1 - trigger));
}

/**
 * Marks where what a tool throws leaves it: the rod's tip, the net's hoop, the wand's
 * ring, the launcher's muzzle. walk.js reads its world position when it fires, so a
 * bobber starts at the end of the rod rather than at the walker's eye - which is what
 * made a cast look like it came from nowhere.
 *
 * Implements: REQ-TOOL-038
 */
function muzzle(parent, x, y, z) {
  const m = new THREE.Object3D();
  m.name = 'muzzle';
  m.position.set(x, y, z);
  parent.add(m);
  return m;
}

/**
 * Lays the first hand's index finger on a trigger: 0 is straight along it, 1 pressed.
 * A fist closed evenly round a camera or a launcher has nothing to fire it with.
 */
function finger(vm, amount) {
  const holder = vm.userData.hands?.[0];
  if (holder?.userData.hand) closeFinger(holder.userData.hand, 0, amount);
}

/** Back to where a gesture started; the one place the rest pose is written down. */
export function restTool(vm) {
  vm.position.set(REST.x, vm.userData.restY ?? REST.y, REST.z);
  vm.rotation.set(REST.rx, REST.ry, REST.rz);
  vm.scale.setScalar(1);
  grip(vm, 0);
}

// Where a tool is held when it is being looked at rather than used: right up at the
// face, turned square on to it.
//
// These are solved rather than chosen. The rotation is the camera body's own turn
// (`body`) undone, which is what leaves the screen facing the eye instead of twenty
// degrees off it; the position is what puts the middle of that screen on the line of
// sight three centimetres out, where it covers nine tenths of the view's height and
// two thirds of its width - the picture edge to edge, with the body's own edges past
// the corners of the frame. Nothing of the tool comes nearer than the screen, so
// nothing is cut open by the near plane; the arm leaves the frame at the bottom
// corner and runs past the eye, which is what it was modelled long enough to do.
const STUDY = { x: 0.0028, y: 0.0464, z: -0.0114, rx: -0.06, ry: 0.3, rz: -0.04 };
const STUDY_AT = new THREE.Vector3();

/**
 * Eases a viewmodel between where it is carried and where it is held up to be looked
 * at; `u` is 0 for the first and 1 for the second.
 *
 * It is laid over whatever the idle wrote rather than replacing it, so the camera
 * still breathes on its way up and the hand does not go rigid the moment a photograph
 * is put on it.
 *
 * Implements: REQ-HUNT-042, REQ-HUNT-043
 */
export function studyTool(vm, u) {
  const k = clamp01(u);
  if (k <= 0) return;
  const rest = vm.userData.restY ?? REST.y;
  vm.position.lerp(STUDY_AT.set(STUDY.x, STUDY.y + rest - REST.y, STUDY.z), k);
  vm.rotation.x += (STUDY.rx - vm.rotation.x) * k;
  vm.rotation.y += (STUDY.ry - vm.rotation.y) * k;
  vm.rotation.z += (STUDY.rz - vm.rotation.z) * k;
}

/**
 * idle gives the held tool a life of its own: a slow breath standing still, and a
 * walk cycle on top of it - the tool rises and falls and swings across as the walker's
 * weight shifts. pace is 0 standing, 1 walking, higher running.
 *
 * The walk cycle is advanced by this frame's turn rather than read off the clock as
 * `t * rate`. A rate read off the clock multiplies the whole of the elapsed time, not
 * the last frame of it, so the moment the pace changes the phase jumps by however many
 * radians that came to - on a tab that has been open an hour, hundreds - and the tool
 * shudders every time the walker breaks into a run. The breath keeps one rate for
 * ever and can be read off the clock; this cannot. It is kept modulo two full turns of
 * the slower term, so both stay continuous where it wraps.
 *
 * Implements: REQ-WALK-042, REQ-TOOL-020
 */
function idle(vm, now, pace = 0, dt = 0) {
  const t = now / 1000;
  const step = vm.userData.step =
    ((vm.userData.step || 0) + dt * 6.2 * Math.max(0.6, pace)) % (4 * Math.PI);
  const breathe = Math.sin(t * 1.15) * 0.006;
  vm.position.y = (vm.userData.restY ?? REST.y) + breathe + Math.abs(Math.sin(step)) * 0.016 * pace - 0.008 * pace;
  vm.position.x = REST.x + Math.sin(step * 0.5) * 0.02 * pace;
  vm.position.z = REST.z;
  vm.rotation.x = REST.rx + Math.sin(step) * 0.022 * pace;
  vm.rotation.y = REST.ry + Math.sin(step * 0.5) * 0.03 * pace;
  vm.rotation.z = Math.sin(t * 0.82) * 0.018 + Math.sin(step * 0.5 + 1.2) * 0.05 * pace;
}

/**
 * Turns a viewmodel about the hand holding it rather than about its own origin.
 *
 * A viewmodel's origin is where the group sits in front of the camera, which is a
 * point in mid-air a little inboard of the fist; a gesture written as a rotation of
 * that group swings everything around that point. For a tool held in the fist that is
 * near enough, but a net is half a metre of shaft with the head on the far end, and
 * an arc about a point that far inboard turns the head into the pivot and the hand
 * into the thing going round it - which is the wrong way up entirely.
 *
 * So the rotation is set first and the group is then shifted by whatever keeps the
 * hand where the rotation would otherwise have moved it from: rotating about a point
 * is rotating about the origin and putting that point back. `hold` is the tool's own,
 * the offset the fist sits at, and `sway` is any travel the arm makes on top of it.
 *
 * Implements: REQ-TOOL-046
 */
function aboutHand(vm, hold, sway = null) {
  PIVOT.set(hold.x, hold.y, hold.z);
  SWUNG_AT.copy(PIVOT).applyEuler(vm.rotation);
  vm.position.set(
    REST.x + PIVOT.x - SWUNG_AT.x + (sway?.x || 0),
    (vm.userData.restY ?? REST.y) + PIVOT.y - SWUNG_AT.y + (sway?.y || 0),
    REST.z + PIVOT.z - SWUNG_AT.z + (sway?.z || 0));
}

const PIVOT = new THREE.Vector3(), SWUNG_AT = new THREE.Vector3();

const smooth = t => t * t * (3 - 2 * t);
const clamp01 = t => Math.min(1, Math.max(0, t));

/**
 * The shape of a swing: a gesture is not a ramp. It loads backwards first
 * (anticipation), drives through the strike, and settles back - which is what makes
 * a throw look thrown rather than slid. Returns -0.55 .. 1 .. 0.
 *
 * Implements: REQ-TOOL-019
 */
function swing(u) {
  if (u < 0.22) return -0.55 * smooth(u / 0.22);
  if (u < 0.46) return -0.55 + 1.55 * smooth((u - 0.22) / 0.24);
  return 1 - smooth(clamp01((u - 0.46) / 0.54));
}

/** A press: fast down, held a moment, eased back. */
function press(u) {
  if (u < 0.18) return smooth(u / 0.18);
  if (u < 0.42) return 1;
  return 1 - smooth(clamp01((u - 0.42) / 0.58));
}

// ------------------------------------------------------------------ the tools

// Implements: REQ-TOOL-014
const rod = {
  id: 'rod',
  label: 'Fishing rod',
  verb: 'Cast at',
  noun: 'landed',
  hint: 'Cast at a building and the line hauls you to it, twice for details; a cast at a bug lands it',
  reticle: 'bobber',
  slot: 1,
  kind: 'primary',
  // A bobber dropped on a beetle is as good a catch as a net over it; the line is
  // what the rod is for, and the catch is what any hook has always been for.
  catchAs: 'reel',
  targets: 'both',
  // A long cast, but a cast: a rod puts a bobber a good way down the street and no
  // further, and past that the line would not be worth hauling on anyway (reel.max).
  reach: 26,
  // A weighted bobber on a line: thrown hard, dropping the way a cast does.
  flight: { speed: 24, arc: 1.6, gravity: 6, drag: 0.1 },
  // The blank goes up, the arm back out of the bottom of the frame.
  hold: { x: 0.2, y: -0.28, z: -0.52, along: [-0.05, 1, 0.3], back: [0.4, -0.3, 1] },
  grip: SWUNG,
  viewmodel() {
    return viewmodel(g => {
      const rod = armed(g, this, tool => { tool.name = 'rod'; });

      const cork = rodPart(0.024, 0.026, 0.22, '#c69a63');
      cork.position.y = -0.03;
      rod.add(cork);
      const seat = rodPart(0.02, 0.02, 0.05, '#2d3239');
      seat.position.y = 0.095;
      rod.add(seat);
      const butt = rodPart(0.024, 0.02, 0.035, '#2d3239');
      butt.position.y = -0.145;
      rod.add(butt);
      // Two tapering sections, so the blank thins towards the tip the way a rod does.
      const lower = rodPart(0.014, 0.0095, 0.34, '#6f4526');
      lower.position.y = 0.29;
      rod.add(lower);
      const upper = rodPart(0.0095, 0.0035, 0.38, '#7d4e2b');
      upper.position.y = 0.64;
      rod.add(upper);
      const tip = part(new THREE.SphereGeometry(0.006, 8, 6), '#d8dee6');
      tip.position.y = 0.83;
      rod.add(tip);
      muzzle(rod, 0, 0.85, 0);
      // Line guides down the blank: small rings, smaller towards the tip.
      for (const [y, r] of [[0.18, 0.022], [0.36, 0.018], [0.54, 0.015], [0.7, 0.012], [0.81, 0.01]]) {
        const guide = part(new THREE.TorusGeometry(r, 0.0025, 5, 12), '#9aa4b0');
        guide.position.set(0, y, r * 0.9);
        guide.rotation.y = Math.PI / 2;
        rod.add(guide);
      }
      // The reel, hanging under the seat: a spool, a side plate and a handle.
      const reel = new THREE.Group();
      reel.position.set(0, 0.09, 0.07);
      const spool = rodPart(0.032, 0.032, 0.026, '#59636f');
      spool.rotation.z = Math.PI / 2;
      reel.add(spool);
      const plate = part(new THREE.CylinderGeometry(0.034, 0.034, 0.008, 14), '#39424c');
      plate.rotation.z = Math.PI / 2;
      plate.position.x = 0.016;
      reel.add(plate);
      const arm = part(new THREE.BoxGeometry(0.008, 0.044, 0.008), '#39424c');
      arm.position.set(0.026, 0.018, 0);
      arm.rotation.z = -0.5;
      reel.add(arm);
      const knob = part(new THREE.SphereGeometry(0.011, 8, 6), '#c69a63');
      knob.position.set(0.046, 0.036, 0);
      reel.add(knob);
      // The foot that joins the reel to the blank.
      const foot = part(new THREE.BoxGeometry(0.014, 0.03, 0.03), '#39424c');
      foot.position.set(0, 0.02, -0.02);
      reel.add(foot);
      rod.add(reel);
    });
  },
  // The rod loads backwards over the shoulder, then snaps through: the cast is in the
  // wrist, so the blank swings much farther than the hand does.
  pose(vm, u) {
    const k = swing(u);
    const rod = vm.getObjectByName('rod');
    rod.rotation.x = k * 1.15;
    rod.rotation.z = -k * 0.12;
    vm.rotation.x = REST.rx - k * 0.22;
    vm.rotation.z = REST.rz + k * 0.1;
    vm.position.z = REST.z + Math.max(0, k) * 0.05;
    vm.position.y = vm.userData.restY - k * 0.02;
    grip(vm, Math.max(0, k));
  },
  projectile(scene) {
    const g = new THREE.Group();
    g.add(flying(scene, new THREE.SphereGeometry(0.05, 12, 9, 0, Math.PI * 2, Math.PI / 2, Math.PI / 2), '#e4402f'));
    g.add(flying(scene, new THREE.SphereGeometry(0.05, 12, 9, 0, Math.PI * 2, 0, Math.PI / 2), '#f2f4f7'));
    const eye = flying(scene, new THREE.TorusGeometry(0.012, 0.004, 5, 10), '#9aa4b0');
    eye.position.y = 0.052;
    eye.rotation.x = Math.PI / 2;
    g.add(eye);
    const stem = flyingRod(scene, 0.006, 0.004, 0.05, '#f2f4f7');
    stem.position.y = -0.06;
    g.add(stem);
    return g;
  },
  line: '#e8ecf2', // a line is drawn from the rod to the bobber while it flies
  // A rod that has hooked something pulls. It anchors where the hook landed and
  // brings the walker to it rather than onto it - a cast at the tenth floor leaves
  // you against that wall, not standing on the roof - and it will not pull from
  // across the map, because past a point the sensible thing is to walk.
  reel: { speed: 11, stop: 1.1, max: 28, onto: false },
};

// Implements: REQ-TOOL-015
const net = {
  id: 'net',
  label: 'Butterfly net',
  verb: 'Net',
  noun: 'netted',
  hint: 'Swing at a bug you can reach; it does not throw',
  reticle: 'hoop',
  slot: 2,
  kind: 'primary',
  // A net catches what is in it. Throwing the whole net at a building across the
  // map was the one thing here that never made sense as a gesture.
  catchAs: 'net',
  targets: 'bugs',
  reach: 2.1,
  hold: { x: 0.2, y: -0.28, z: -0.52, along: [0.08, 1, 0.32], back: [0.4, -0.3, 1] },
  grip: SWUNG,
  viewmodel() {
    return viewmodel(g => {
      const net = armed(g, this, tool => { tool.name = 'net'; });

      const shaft = rodPart(0.014, 0.018, 0.62, '#c8a26a');
      shaft.position.y = 0.17;
      net.add(shaft);
      const wrap = rodPart(0.021, 0.021, 0.2, '#8a6a3c'); // the bound grip, in the fist
      wrap.position.y = -0.04;
      net.add(wrap);
      const ferrule = rodPart(0.016, 0.016, 0.04, '#b4bcc6');
      ferrule.position.y = 0.45;
      net.add(ferrule);

      // The head, built so the shaft runs into the rim rather than through the middle
      // of the mouth. A hoop centred on the shaft's axis is a landing net looked at
      // down its handle; a butterfly net's hoop stands in the line of the shaft, and
      // the stick ends where the rim begins. That is one group and one offset: the
      // mouth is turned a quarter so its plane contains the shaft, and lifted by its
      // own radius so the bottom of the rim sits exactly on the ferrule.
      const head = new THREE.Group();
      head.position.y = 0.47;
      head.rotation.x = -0.25;
      head.name = 'head';
      net.add(head);

      const mouth = new THREE.Group();
      mouth.rotation.x = -Math.PI / 2; // the mouth faces the way the net is swung
      mouth.position.y = HOOP;
      head.add(mouth);
      const hoop = part(new THREE.TorusGeometry(HOOP, 0.006, 6, 24), '#dfe4ea');
      hoop.rotation.x = Math.PI / 2;
      mouth.add(hoop);
      bagOf(mouth, HOOP, BAG);

      // And the join itself: a socket over the end of the shaft, and two struts
      // splayed from it to the rim, which is how a net is actually built. Without
      // them the hoop is a ring balanced on a stick.
      head.add(part(new THREE.CylinderGeometry(0.019, 0.022, 0.05, 10), '#aab2bb'));
      for (const side of [-1, 1]) {
        const out = Math.sin(0.85) * HOOP, up = HOOP - Math.cos(0.85) * HOOP;
        const strut = rodPart(0.006, 0.005, Math.hypot(out, up), '#aab2bb');
        strut.position.set(side * out / 2, 0.02 + up / 2, 0);
        strut.rotation.z = -Math.atan2(side * out, up);
        head.add(strut);
      }
      muzzle(head, 0, HOOP, 0);
    });
  },
  /**
   * The stroke a net is actually swung with: up and back over the shoulder on the
   * wind-up, then down across the body to finish low on the other side. swing() runs
   * back to -0.55 before it drives to 1, so one set of angles gives both ends of it -
   * the hand starts high and to the right and ends low and to the left, and the hoop
   * turns over with it so the mouth leads the whole way down.
   *
   * It turns about the hand (aboutHand), which is what makes it a swing rather than a
   * net held still while the arm is carried around it: the head is half a metre out on
   * the end of the shaft, and it is the head that has to travel.
   *
   * Implements: REQ-TOOL-044, REQ-TOOL-046
   */
  pose(vm, u) {
    const k = swing(u);
    const net = vm.getObjectByName('net');
    const head = vm.getObjectByName('head');
    vm.rotation.z = REST.rz + k * 1.25;
    vm.rotation.y = REST.ry - k * 0.45;
    vm.rotation.x = REST.rx + k * 0.62;
    // The arm's own travel on top of the turn: back over the shoulder on the wind-up,
    // then across the body, down, and out - a net is swung at something in front of
    // you, so the stroke has to go there and not only past you.
    aboutHand(vm, this.hold, { x: -k * 0.17, y: -k * 0.14, z: -k * 0.2 });
    net.rotation.z = k * 0.4;
    head.rotation.z = -k * 0.5; // the bag lags behind the hoop
    grip(vm, Math.abs(k) * 0.8);
  },
  projectile: null, // the net stays on the stick
};

// How much smaller than life the camera body is drawn, so one hand can hold it.
const SHELL = 0.78;

// Implements: REQ-TOOL-016
const camera = {
  id: 'camera',
  label: 'Camera',
  verb: 'Photograph',
  noun: 'photographed',
  hint: 'Photograph a building or a bug at any range; every use keeps the picture, and G opens them',
  reticle: 'frame',
  slot: 3,
  kind: 'primary',
  // A photograph records whatever is in the frame, near or far, bug or building.
  catchAs: 'flash',
  // ... and every use of it keeps the picture, which goes into the stash (walk.js,
  // onPhoto). Pressing the shutter is what a camera is for, so there is no second
  // gesture to learn and no use of it that does nothing you can see.
  keeps: true,
  targets: 'both',
  // Unlike the other tools the camera is not held in a fist, so it is placed first
  // and the hand is laid on it: fingers round the body's own grip, the way anyone
  // holds a camera they are about to fire one-handed.
  body: { x: 0.055, y: -0.03, z: -0.09, rx: 0.06, ry: -0.3, rz: 0.04 },
  right: { at: [0.205, -0.03, 0.02], along: [0.1, 1, -0.05], back: [0.45, -0.95, 0.5], close: 0.9 },
  viewmodel() {
    return viewmodel(g => {
      const cam = new THREE.Group();
      cam.name = 'cam';
      cam.position.set(this.body.x, this.body.y, this.body.z);
      cam.rotation.set(this.body.rx, this.body.ry, this.body.rz);
      g.add(cam);
      // A camera modelled at the size of a real one is more than one hand can hold at
      // the scale the walker is: the body is drawn a little smaller than life, and the
      // hand goes on the grip where the shrinking leaves it.
      const shell = new THREE.Group();
      shell.scale.setScalar(SHELL);
      cam.add(shell);

      const body = part(new THREE.BoxGeometry(0.27, 0.165, 0.11), '#2c3138');
      shell.add(body);
      shell.add(part(new THREE.BoxGeometry(0.06, 0.15, 0.115).translate(0.105, -0.005, 0.005), '#20242a')); // the grip
      shell.add(part(new THREE.BoxGeometry(0.27, 0.03, 0.112).translate(0, 0.045, 0.002), '#3a4149'));      // a leatherette band
      // The prism hump, with the viewfinder behind it.
      shell.add(part(new THREE.BoxGeometry(0.09, 0.05, 0.08).translate(-0.02, 0.105, 0.005), '#262b31'));
      shell.add(part(new THREE.BoxGeometry(0.05, 0.035, 0.012).translate(-0.02, 0.1, 0.056), '#0d0f12'));
      const barrel = rodPart(0.062, 0.07, 0.115, '#1a1d21');
      barrel.rotation.x = Math.PI / 2;
      barrel.position.z = -0.11;
      shell.add(barrel);
      const focus = rodPart(0.073, 0.073, 0.03, '#2f353c'); // the focus ring, ribbed by its facets
      focus.rotation.x = Math.PI / 2;
      focus.position.z = -0.115;
      shell.add(focus);
      const hood = rodPart(0.078, 0.07, 0.03, '#15181b');
      hood.rotation.x = Math.PI / 2;
      hood.position.z = -0.175;
      shell.add(hood);
      const glass = part(new THREE.SphereGeometry(0.056, 14, 8, 0, Math.PI * 2, 0, Math.PI / 2), '#5c9be0');
      glass.rotation.x = -Math.PI / 2;
      glass.position.z = -0.168;
      shell.add(glass);
      const button = rodPart(0.019, 0.019, 0.022, '#e4402f');
      button.position.set(0.1, 0.09, 0.01);
      button.name = 'button';
      shell.add(button);
      const dial = rodPart(0.028, 0.028, 0.02, '#40474f');
      dial.position.set(0.045, 0.092, -0.01);
      shell.add(dial);
      // The flash, which brightens for a frame when the shutter goes.
      const flash = part(new THREE.BoxGeometry(0.05, 0.016, 0.02).translate(-0.02, 0.13, 0.0), '#e8eef6');
      flash.name = 'flash';
      shell.add(flash);

      // The screen on the back, which is the one part of this that is not scenery:
      // it shows the world the lens is pointed at, rendered small each frame (live
      // below). A bezel around it, and a lit strip above it while the film runs.
      const bezel = part(new THREE.BoxGeometry(0.155, 0.115, 0.008).translate(-0.035, -0.012, 0.058), '#15181b');
      shell.add(bezel);
      const screen = new THREE.Mesh(
        new THREE.PlaneGeometry(SCREEN_W, SCREEN_H).translate(-0.035, -0.012, 0.063),
        // Unlit, unlike the rest of what is held: a screen makes its own light, and
        // the walk camera's lamps would only wash the picture out.
        new THREE.MeshBasicMaterial({ color: '#0b0d10', toneMapped: false }));
      screen.name = 'screen';
      shell.add(screen);
      const lamp = part(new THREE.BoxGeometry(0.012, 0.008, 0.006).translate(-0.1, 0.05, 0.058), '#3ad07a');

      shell.add(lamp);

      grasps(g, cam, { ...this.right, at: this.right.at.map(v => v * SHELL) });
      g.userData.trigger = 0.12; // the index finger rests on the shutter release
    });
  },
  // The camera is lifted to the eye, the shutter pressed, and the body kicks back.
  pose(vm, u) {
    const k = press(u);
    const cam = vm.getObjectByName('cam');
    cam.getObjectByName('button').position.y = 0.09 - k * 0.011;
    cam.getObjectByName('flash').material.color.set(k > 0.4 ? '#ffffff' : '#e8eef6');
    vm.position.z = REST.z - k * 0.05;
    vm.position.y = vm.userData.restY + k * 0.03;
    vm.rotation.x = REST.rx + k * 0.1;
    vm.rotation.y = REST.ry + k * 0.12;
    grip(vm, k * 0.35, k);
  },
  projectile: null, // a photograph arrives the moment it is taken
  flash: '#ffffff',

  /**
   * Hangs a photograph on the back of the camera in place of the live view, or takes
   * it off again with null. The screen is squarer than the window the picture was
   * taken through, so the picture is filled to the screen and what will not fit is
   * cropped off the long side - a photograph squashed to a shape it was never in is
   * worse than a photograph with its edges missing.
   *
   * Implements: REQ-HUNT-042, REQ-HUNT-046
   */
  shows(vm, texture) {
    vm.userData.photo = texture || null;
    if (!texture) return;
    texture.colorSpace = THREE.SRGBColorSpace;
    const shot = (texture.image?.width || 1) / (texture.image?.height || 1);
    const wide = shot / (SCREEN_W / SCREEN_H);
    texture.center.set(0.5, 0.5);
    texture.repeat.set(wide > 1 ? 1 / wide : 1, wide > 1 ? 1 : wide);
    texture.needsUpdate = true;
  },

  /**
   * The live view, once a frame: the world drawn again from the lens into a small
   * texture and hung on the back of the camera. Only this tool has one, and only
   * while it is the tool in hand - it is a second pass over the whole map, which is
   * worth it for the one thing here that is supposed to be looking at something.
   *
   * A photograph put up on it stands in for that view, and while one is up the second
   * pass is not made at all: the camera is then being used as the thing that holds
   * its pictures rather than as a camera.
   *
   * Implements: REQ-HUNT-047
   */
  live(vm, scene, lens) {
    const screen = vm.getObjectByName('screen');
    if (!screen) return;
    const texture = vm.userData.photo || scene.film(lens);
    if (screen.material.map !== texture) {
      screen.material.map = texture;
      screen.material.color.set('#ffffff');
      screen.material.needsUpdate = true;
    }
  },
};

// Implements: REQ-TOOL-017, REQ-TOOL-045
const bubbles = {
  id: 'bubbles',
  label: 'Bubble wand',
  verb: 'Bubble',
  noun: 'bubbled',
  hint: 'Float a bubble onto a bug; it rises and drifts',
  slot: 4,
  kind: 'primary',
  // Soap on a beetle is a catch; soap on a wall is a clean wall.
  catchAs: 'bubble',
  targets: 'bugs',
  // Nothing blown off a wand carries: it is out of reach across the street, never
  // mind across the map.
  reach: 7,
  // A bubble is lighter than the air it is thrown through: it slows almost at once
  // and then climbs, which is why it is lobbed high and aimed early.
  flight: { speed: 12, arc: 2.2, gravity: -1.1, drag: 1.5 },
  reticle: 'soft',
  hold: { x: 0.2, y: -0.28, z: -0.52, along: [0.05, 1, 0.32], back: [0.4, -0.3, 1] },
  grip: SWUNG,
  viewmodel() {
    return viewmodel(g => {
      const wand = armed(g, this, tool => { tool.name = 'wand'; });

      const cap = rodPart(0.03, 0.028, 0.17, '#2f7fb8'); // the bottle cap it screws into
      cap.position.y = -0.03;
      wand.add(cap);
      const stick = rodPart(0.01, 0.012, 0.3, '#59b0e6');
      stick.position.y = 0.2;
      wand.add(stick);

      // The head is placed at the tip of the stick, and the ring is lifted inside it by
      // its own radius so the bottom of the rim sits on that tip. Centred on the tip
      // instead - which is what it was - the stick runs through the middle of the ring
      // and out the far side, and the thing reads as a ring threaded onto a stick
      // rather than a wand with a ring on the end of it.
      const head = new THREE.Group();
      head.position.y = 0.35;
      head.rotation.x = -0.3;
      head.name = 'head';
      wand.add(head);
      const ring = part(new THREE.TorusGeometry(RING, 0.0075, 6, 18), '#59b0e6');
      ring.rotation.x = Math.PI / 2;
      ring.position.y = RING;
      ring.name = 'ring';
      head.add(ring);
      // The soap film across the ring, which stretches as the wand is waved.
      const film = part(new THREE.CircleGeometry(RING - 0.005, 18), '#cdeaff',
        { transparent: true, opacity: 0.3, side: THREE.DoubleSide });
      film.rotation.x = Math.PI / 2;
      film.position.y = RING;
      film.name = 'film';
      head.add(film);
      // And the joint itself: a collar over the end of the stick where the rim meets
      // it, which is what stops the two from looking merely adjacent - and covers the
      // hair's breadth the ring lifts off it as it is waved.
      head.add(part(new THREE.CylinderGeometry(0.016, 0.019, 0.035, 10), '#2f7fb8'));
      muzzle(head, 0, RING, 0);
    });
  },
  // A slow wave through the ring, the film bulging as the air goes through it.
  pose(vm, u) {
    const k = swing(u);
    const wand = vm.getObjectByName('wand');
    const ring = vm.getObjectByName('ring');
    const film = vm.getObjectByName('film');
    vm.rotation.y = REST.ry + Math.sin(u * Math.PI * 2) * 0.22;
    vm.rotation.z = REST.rz - k * 0.3;
    vm.position.y = vm.userData.restY + Math.sin(u * Math.PI) * 0.05;
    wand.rotation.z = Math.sin(u * Math.PI * 2) * 0.25;
    ring.scale.setScalar(1 + Math.max(0, k) * 0.2);
    film.scale.setScalar(1 + Math.max(0, k) * 0.2);
    film.material.opacity = 0.3 * (1 - Math.sin(clamp01(u) * Math.PI)); // it thins, lets go, and re-forms
    grip(vm, 0.3 + Math.abs(k) * 0.4);
  },
  projectile(scene) {
    const g = new THREE.Group();
    g.add(flying(scene, new THREE.SphereGeometry(0.11, 16, 12), '#bfe6ff', { transparent: true, opacity: 0.4 }));
    const rim = flying(scene, new THREE.SphereGeometry(0.111, 16, 12), '#ffffff',
      { transparent: true, opacity: 0.22, side: THREE.BackSide });
    const sheen = flying(scene, new THREE.SphereGeometry(0.035, 10, 8), '#ffffff', { transparent: true, opacity: 0.65 });
    sheen.position.set(0.042, 0.045, 0.04);
    g.add(rim, sheen);
    g.userData.wobble = true;
    return g;
  },
};

// Where the extinguisher's lever sits when nothing is squeezing it, and how far down
// from the hand the bottle hangs.
const LEVER_UP = 0.055, BOTTLE = -0.2;

/**
 * A fire extinguisher: the third way of taking a bug off the street, and the one with
 * no finesse in it at all. The net has to be swung within arm's length and a bubble
 * has to be floated onto its target; a horn full of foam is pointed in the rough
 * direction and squeezed. What comes out is slow, spreads as it goes and drops out of
 * the air a few strides on, so it clears a doorway rather than picking a beetle off a
 * roof - which is the trade, and why all three are worth carrying.
 *
 * It is carried the way one is: by the handle at the neck, with the bottle hanging
 * below the fist. What runs through the fist is therefore the handle bar and not the
 * bottle, which is three times too fat for a hand to close on - the same reckoning as
 * the rod's cork and the net's bound grip.
 *
 * Implements: REQ-TOOL-035
 */
const extinguisher = {
  id: 'extinguisher',
  label: 'Fire extinguisher',
  verb: 'Douse',
  noun: 'doused',
  hint: 'Hold it on a burning building to put the fire out, or hose a bug at close quarters',
  reticle: 'spray',
  slot: 5,
  kind: 'primary',
  // Foam on a beetle is a beetle that has stopped; foam on a wall is a wall to wash.
  catchAs: 'foam',
  targets: 'bugs',
  // ... and foam on a fire is the reason the tool is in the bag at all (fires.js).
  //
  // It is the only thing here that is paid in seconds rather than in gestures. Every
  // other primary tool is one act with one result - a cast, a shot, a photograph -
  // and fire does not answer to an act. It answers to standing in front of it and
  // keeping the cone on it, which is what `auto` below was already for and what a
  // walker already does with this tool without being told.
  //
  // Dousing is not tagging: a building put out is not a module marked, and the aim
  // keeps the two apart (walk.js, aim.box against aim.i). The crosshair still marks a
  // building this can help, though, or there would be no way to tell from the street
  // which fire is in range.
  douses: true,
  // A horn throws foam across a room and no further.
  reach: 5,
  // Held down, it keeps discharging, which is what an extinguisher does and what makes
  // it worth carrying against a street full of bugs rather than one on a wall.
  auto: 0.12,
  // It leaves fast and is stopped almost at once by its own drag, then falls: the
  // shortest and heaviest arc in the bag, and the only one that spreads on the way.
  flight: { speed: 15, arc: 0.5, gravity: 3.2, drag: 2.6 },
  // Held higher than a rod, because what it carries hangs under the hand rather than
  // standing up out of it.
  hold: { x: 0.2, y: -0.2, z: -0.5, along: [0.06, 1, 0.24], back: [0.4, -0.3, 1] },
  grip: SWUNG,
  viewmodel() {
    return viewmodel(g => {
      const body = armed(g, this, tool => { tool.name = 'body'; });

      // The handle, which is the part in the fist: a bar no thicker than a rod's cork,
      // with the valve block and its lever standing above it.
      const handle = rodPart(0.023, 0.023, 0.15, '#2f353c');
      body.add(handle);
      const valve = part(new THREE.BoxGeometry(0.052, 0.05, 0.062).translate(0, 0.095, 0), '#3c444d');
      body.add(valve);
      // Placed rather than built where it sits, because the gesture moves it and a
      // geometry already carried into place would be moved from there twice.
      const lever = part(new THREE.BoxGeometry(0.04, 0.013, 0.09), '#d8dadf');
      lever.position.set(0, LEVER_UP, 0.028);
      lever.name = 'lever';
      body.add(lever);
      body.add(part(new THREE.CylinderGeometry(0.019, 0.019, 0.006, 12)
        .rotateX(Math.PI / 2).translate(0, 0.095, -0.036), '#f1efe8')); // the gauge

      // The bottle, hanging under the hand: a red cylinder with a domed base, a label
      // band round its middle and a neck up into the valve.
      const neck = rodPart(0.02, 0.026, 0.07, '#8d9199');
      neck.position.y = -0.055;
      body.add(neck);
      const shell = rodPart(0.055, 0.052, 0.28, '#c0231f');
      shell.position.y = BOTTLE;
      body.add(shell);
      body.add(part(new THREE.SphereGeometry(0.055, 12, 8).scale(1, 0.55, 1)
        .translate(0, BOTTLE - 0.14, 0), '#9b1c19'));
      const label = rodPart(0.0565, 0.0565, 0.09, '#f1efe8');
      label.position.y = BOTTLE + 0.01;
      body.add(label);

      // The hose out of the valve, and the horn on the end of it. Both run forward:
      // the tool is modelled with its shaft along +y because that is what the fist
      // closes on (SWUNG), which leaves +z pointing down the view - so a horn that is
      // to throw foam where the walker is looking is turned onto +z and left there.
      const hose = rodPart(0.012, 0.012, 0.13, '#1d2024');
      hose.rotation.x = 1.15;
      hose.position.set(0.035, 0.085, 0.055);
      hose.rotation.z = -0.35;
      body.add(hose);
      const horn = new THREE.Group();
      horn.position.set(0.06, 0.055, 0.105);
      horn.rotation.x = Math.PI / 2; // along the view, which is where it is aimed
      horn.name = 'horn';
      body.add(horn);
      horn.add(rodPart(0.013, 0.019, 0.07, '#1d2024'));
      horn.add(part(new THREE.CylinderGeometry(0.019, 0.05, 0.1, 14, 1, true).translate(0, 0.08, 0), '#15181b',
        { side: THREE.DoubleSide }));
      muzzle(horn, 0, 0.14, 0);
    });
  },
  // The lever is squeezed, the bottle kicks back against the hand, and the horn lifts
  // as the charge goes through it.
  pose(vm, u) {
    const k = press(u);
    vm.getObjectByName('lever').position.y = LEVER_UP - k * 0.014;
    vm.getObjectByName('horn').rotation.x = Math.PI / 2 - k * 0.12;
    vm.position.z = REST.z + k * 0.05;
    vm.position.y = vm.userData.restY + k * 0.02;
    vm.rotation.x = REST.rx + k * 0.16;
    vm.rotation.z = REST.rz - k * 0.1;
    grip(vm, 0.4 + k * 0.5);
  },
  // A gout of foam: three lumps of it, off centre, that swell as they fly.
  projectile(scene) {
    const g = new THREE.Group();
    const skin = { transparent: true, opacity: 0.55, depthWrite: false };
    g.add(flying(scene, new THREE.SphereGeometry(0.075, 10, 8), '#f4f8ff', skin));
    for (const [x, y, z, r] of [[0.05, 0.03, -0.03, 0.05], [-0.045, -0.02, 0.02, 0.042]]) {
      const lump = flying(scene, new THREE.SphereGeometry(r, 8, 6), '#e8eef7', skin);
      lump.position.set(x, y, z);
      g.add(lump);
    }
    g.userData.swell = 2.2; // it opens out into a cloud on the way
    return g;
  },
};

// Implements: REQ-TOOL-018, REQ-TOOL-027, REQ-TOOL-041
const dart = {
  id: 'dart',
  label: 'Tracking dart',
  verb: 'Tag',
  noun: 'tagged',
  hint: 'One aimed shot at a distant building, through the scope; the dart steers itself home. Hit it again for details',
  reticle: 'scope',
  slot: 6,
  kind: 'primary',
  targets: 'buildings',
  // The longest shot in the hunt, and the only one worth taking through a scope: a
  // dart launcher throws a good deal further than an arm and still not as far as a
  // rifle, which is what the reach is for - from a rooftop the crosshair finds most of
  // the city, and tagging it all from up there would be no hunt at all.
  reach: 34,
  // What the name says: a dart that is thrown high and steers. It is slow enough to
  // watch, heavy enough to fall, and while it falls its fins pull it round towards
  // whatever building lies ahead - so a shot lobbed over a block still lands on a
  // wall. One goes at a time, and it is aimed.
  flight: { speed: 26, arc: 1.8, gravity: 9, drag: 0.15, track: 2.4 },
  ...PISTOL,
  viewmodel() {
    return viewmodel(g => {
      const gun = armed(g, this, tool => { tool.name = 'gun'; });
      // The launcher is modelled about its barrel, the way one would be drawn; this
      // hangs the whole thing off the pistol grip, which is the part a fist closes on.
      const frame = new THREE.Group();
      frame.position.set(0, 0.045, -0.01);
      gun.add(frame);

      const body = part(new THREE.BoxGeometry(0.078, 0.078, 0.2).translate(0, 0.05, -0.12), '#4d5761');
      frame.add(body);
      const barrel = rodPart(0.028, 0.032, 0.3, '#39424c');
      barrel.rotation.x = Math.PI / 2;
      barrel.position.set(0, 0.05, -0.3);
      frame.add(barrel);
      const nozzle = rodPart(0.036, 0.034, 0.04, '#23292f');
      nozzle.rotation.x = Math.PI / 2;
      nozzle.position.set(0, 0.05, -0.44);
      frame.add(nozzle);
      muzzle(frame, 0, 0.05, -0.47);
      // A charged air cylinder along the barrel, on the side that faces the view.
      const tank = rodPart(0.024, 0.024, 0.18, '#4c8ab2');
      tank.rotation.x = Math.PI / 2;
      tank.position.set(-0.042, 0.03, -0.2);
      frame.add(tank);
      const valve = rodPart(0.014, 0.014, 0.03, '#b9bec6');
      valve.rotation.x = Math.PI / 2;
      valve.position.set(-0.042, 0.03, -0.3);
      frame.add(valve);
      // The grip, running down through the fist.
      frame.add(part(new THREE.BoxGeometry(0.052, 0.14, 0.05).translate(0, -0.045, 0.01), '#262b31'));
      frame.add(part(new THREE.BoxGeometry(0.02, 0.035, 0.012).translate(0, 0.015, -0.055), '#15181b')); // the trigger guard
      // The scope, on two mounts.
      const scope = rodPart(0.022, 0.022, 0.19, '#2f353c');
      scope.rotation.x = Math.PI / 2;
      scope.position.set(0, 0.115, -0.2);
      frame.add(scope);
      const bell = rodPart(0.03, 0.026, 0.045, '#2f353c');
      bell.rotation.x = Math.PI / 2;
      bell.position.set(0, 0.115, -0.31);
      frame.add(bell);
      const lens = part(new THREE.CircleGeometry(0.026, 14), '#7fb8ff');
      lens.position.set(0, 0.115, -0.333);
      frame.add(lens);
      for (const z of [-0.14, -0.26]) {
        frame.add(part(new THREE.BoxGeometry(0.016, 0.04, 0.018).translate(0, 0.085, z), '#2b3138'));
      }
      // A magazine of darts, one of them showing.
      const mag = part(new THREE.BoxGeometry(0.05, 0.05, 0.05).translate(0, 0.095, -0.08), '#39424c');
      frame.add(mag);
      const loaded = rodPart(0.008, 0.008, 0.06, '#ff8a1f');
      loaded.rotation.x = Math.PI / 2;
      loaded.position.set(0, 0.095, -0.125);
      loaded.name = 'loaded';
      frame.add(loaded);
      g.userData.trigger = 0.1; // the index finger rests on the trigger
    });
  },
  // Recoil: the whole thing drives back and the muzzle rises, then settles.
  pose(vm, u) {
    const k = press(u);
    const gun = vm.getObjectByName('gun');
    gun.getObjectByName('loaded').visible = u > 0.55; // the next dart rides up
    vm.position.z = REST.z + k * 0.07;
    vm.position.y = vm.userData.restY + k * 0.012;
    vm.rotation.x = REST.rx + k * 0.22;
    vm.rotation.z = REST.rz + k * 0.05;
    grip(vm, k * 0.4, k);
  },
  projectile(scene) {
    const g = new THREE.Group();
    const body = flyingRod(scene, 0.009, 0.009, 0.24, '#2b2d31');
    body.rotation.x = Math.PI / 2;
    g.add(body);
    const tip = flying(scene, new THREE.ConeGeometry(0.019, 0.07, 10).rotateX(Math.PI / 2).translate(0, 0, 0.155), '#ff8a1f');
    g.add(tip);
    const collar = flying(scene, new THREE.TorusGeometry(0.012, 0.004, 5, 10), '#9aa4b0');
    collar.position.z = 0.1;
    g.add(collar);
    for (let i = 0; i < 3; i++) {
      const fin = flying(scene, new THREE.BoxGeometry(0.004, 0.05, 0.07).translate(0, 0.025, -0.1), '#d8dadf');
      fin.rotation.z = (i * Math.PI * 2) / 3;
      g.add(fin);
    }
    g.userData.aim = true; // points along its flight
    return g;
  },
};

// ------------------------------------------------- what the buildings are for

/**
 * A framing nailer: the construction trade's answer to the tracking dart, and its
 * opposite. A nail goes exactly where the muzzle pointed, very fast and very flat,
 * and it never thinks better of it; the dart is lobbed and steers. The magazine means
 * nails go one after another, so a wall of a warehouse can be pinned at a run in a way
 * a single dart cannot - and anything small enough gets pinned to the wall with it.
 *
 * Implements: REQ-TOOL-027, REQ-TOOL-042, REQ-TOOL-051
 */
const nailer = {
  id: 'nailer',
  label: 'Nail gun',
  verb: 'Pin',
  noun: 'pinned',
  hint: 'Hold the button down and it keeps firing; anything small enough is pinned to the wall behind it',
  reticle: 'cross',
  slot: 7,
  kind: 'primary',
  // A nail through a beetle is a beetle pinned to the wall behind it, which is a
  // catch by anyone's reckoning.
  catchAs: 'pin',
  targets: 'both',
  // Across a street and no further: a nail gun drives a nail into what is in front of
  // it, and everything about it is built for the near end of the map.
  reach: 9,
  // Held down, it keeps going - which is the whole point of a strip nailer, and the
  // thing that makes it the opposite of the dart rather than a worse one. Six a second
  // is fast enough to sweep a wall and slow enough to see each nail leave.
  auto: 0.16,
  // Fired rather than thrown, and the opposite of the dart in every term: three times
  // the speed, no lob worth the name, barely any drop over the distance it covers, and
  // no steering at all. What it has instead is scatter - a nail leaves a strip nailer
  // crooked - so held down it hoses a wall rather than picking a spot on it.
  //
  // Fast, but no longer so fast that the nail is never on screen: at ninety a unit a
  // second it crossed its own reach in six frames, which for something a centimeter
  // across is a shot nobody saw leave. Fifty-five is still twice the dart's.
  flight: { speed: 55, arc: 0.02, gravity: 1.2, drag: 0, spread: 0.045 },
  ...PISTOL,
  viewmodel() {
    return viewmodel(g => {
      const gun = armed(g, this, tool => { tool.name = 'gun'; });
      const frame = new THREE.Group();
      frame.position.set(0, 0.045, -0.01);
      gun.add(frame);
      // The body, in the yellow every tool on a site is painted.
      frame.add(part(new THREE.BoxGeometry(0.085, 0.1, 0.17).translate(0, 0.06, -0.09), '#e8a317'));
      frame.add(part(new THREE.BoxGeometry(0.089, 0.03, 0.06).translate(0, 0.105, -0.11), '#2b2f34'));
      // The nose: a stepped muzzle with the safety contact tip standing off it.
      const nose = rodPart(0.026, 0.02, 0.16, '#39424c');
      nose.rotation.x = Math.PI / 2;
      nose.position.set(0, 0.045, -0.24);
      frame.add(nose);
      const tip = rodPart(0.016, 0.013, 0.05, '#b9bec6');
      tip.rotation.x = Math.PI / 2;
      tip.position.set(0, 0.045, -0.34);
      frame.add(tip);
      muzzle(frame, 0, 0.045, -0.37);
      // The magazine, raked back under the nose the way a strip nailer's is, with a
      // strip of nails showing along it.
      const mag = new THREE.Group();
      mag.position.set(0, 0.03, -0.13);
      mag.rotation.x = -0.62;
      frame.add(mag);
      mag.add(part(new THREE.BoxGeometry(0.036, 0.26, 0.032).translate(0, -0.11, 0), '#2b2f34'));
      mag.add(part(new THREE.BoxGeometry(0.014, 0.2, 0.01).translate(0.024, -0.09, 0), '#c9ced6'));
      // The grip and its trigger, and the air line coming out of the heel.
      frame.add(part(new THREE.BoxGeometry(0.05, 0.13, 0.05).translate(0, -0.04, 0.01), '#2b2f34'));
      frame.add(part(new THREE.BoxGeometry(0.018, 0.032, 0.012).translate(0, 0.018, -0.05), '#15181b'));
      const hose = rodPart(0.012, 0.012, 0.09, '#d0433a');
      hose.rotation.x = 0.5;
      hose.position.set(0, -0.1, 0.06);
      frame.add(hose);
      g.userData.trigger = 0.1;
    });
  },
  // Recoil: a nailer kicks up and comes straight back down onto the work.
  pose(vm, u) {
    const k = press(u);
    vm.position.z = REST.z + k * 0.05;
    vm.position.y = vm.userData.restY + k * 0.02;
    vm.rotation.x = REST.rx + k * 0.3;
    vm.rotation.z = REST.rz + k * 0.03;
    grip(vm, k * 0.4, k);
  },
  projectile(scene) {
    const g = new THREE.Group();
    const shank = flyingRod(scene, 0.009, 0.009, 0.16, '#dfe4ea');
    shank.rotation.x = Math.PI / 2;
    g.add(shank);
    g.add(flying(scene, new THREE.ConeGeometry(0.011, 0.04, 8).rotateX(Math.PI / 2).translate(0, 0, 0.096), '#f4f7fb'));
    const head = flying(scene, new THREE.CylinderGeometry(0.019, 0.019, 0.008, 10), '#aab2bb');
    head.rotation.x = Math.PI / 2;
    head.position.z = -0.082;
    g.add(head);
    // A nail is small and leaves at the speed of a nail, so on its own it is a couple
    // of frames of nothing between the muzzle and the wall. The streak behind it is
    // what makes the shot legible: it is drawn from the same nose, points the same way
    // (userData.aim), and is the length of about one frame of travel.
    const streak = flying(scene, new THREE.CylinderGeometry(0.004, 0.012, 0.9, 6).rotateX(Math.PI / 2).translate(0, 0, -0.48),
      '#ffe9b8', { transparent: true, opacity: 0.4, depthWrite: false });
    g.add(streak);
    g.userData.aim = true;
    return g;
  },
};

/**
 * A grapple gun, which is the one tool here that does nothing to what it hits. It
 * bites, and then the line pulls the walker up it: a facade is climbed, a roof is
 * arrived on, and from that roof the same shot fired at the street below is the way
 * down. Nothing is tagged and nothing is caught - this is how you get about.
 */
const grapple = {
  id: 'grapple',
  label: 'Grapple gun',
  verb: 'Hook',
  noun: 'climbed',
  hint: 'F or C fires it: hook a building to be pulled onto its roof, and from up there hook something lower to come down',
  reticle: 'hook',
  slot: 8,
  kind: 'secondary',
  // The longest reach in the bag, because a line is the one thing here that is meant
  // to span a street - but a line, not a rifle: it ends where the rope does.
  reach: 50,
  // A line paid out taut: no lob and no drop worth speaking of.
  flight: { speed: 55, arc: 0.05, gravity: 0.6, drag: 0 },
  line: '#cfd6de', // the line stays drawn, out and back
  // A winch rather than a rod: faster, right up to what it caught, from any range,
  // and it sets the walker on top of it. Catching nothing else is the point of the
  // tool, so arriving is all it does (walk.js).
  reel: { speed: 17, stop: 0.25, max: 60, onto: true },
  climbs: true, // ... and nothing is tagged or caught when it lands
  ...PISTOL,
  viewmodel() {
    return viewmodel(g => {
      const gun = armed(g, this, tool => { tool.name = 'gun'; });
      const frame = new THREE.Group();
      frame.position.set(0, 0.045, -0.01);
      gun.add(frame);
      // A stubby launcher in the matt grey of issued kit, with the drum of line on
      // its side - which is the part that says what this does.
      frame.add(part(new THREE.BoxGeometry(0.07, 0.085, 0.15).translate(0, 0.055, -0.08), '#3c444d'));
      const barrel = rodPart(0.034, 0.036, 0.22, '#2b3138');
      barrel.rotation.x = Math.PI / 2;
      barrel.position.set(0, 0.055, -0.22);
      frame.add(barrel);
      muzzle(frame, 0, 0.055, -0.33);
      const drum = rodPart(0.05, 0.05, 0.03, '#59636e');
      drum.rotation.z = Math.PI / 2;
      drum.position.set(-0.05, 0.055, -0.08);
      frame.add(drum);
      const coil = part(new THREE.TorusGeometry(0.034, 0.005, 5, 16), '#cfd6de'); // the line on it
      coil.rotation.y = Math.PI / 2;
      coil.position.set(-0.066, 0.055, -0.08);
      frame.add(coil);
      // The claw, folded back along the barrel until it is fired.
      const claw = new THREE.Group();
      claw.position.set(0, 0.055, -0.32);
      claw.name = 'claw';
      frame.add(claw);
      for (let i = 0; i < 3; i++) {
        const arm = rodPart(0.006, 0.004, 0.09, '#b9bec6');
        arm.position.set(0, 0.03, 0.03);
        arm.rotation.x = -0.9;
        const hinge = new THREE.Group();
        hinge.rotation.z = (i * Math.PI * 2) / 3;
        hinge.add(arm);
        claw.add(hinge);
      }
      frame.add(part(new THREE.BoxGeometry(0.05, 0.13, 0.05).translate(0, -0.04, 0.01), '#23292f'));
      frame.add(part(new THREE.BoxGeometry(0.018, 0.032, 0.012).translate(0, 0.018, -0.05), '#15181b'));
      g.userData.trigger = 0.1;
    });
  },
  // A launcher this size shoves back rather than kicking up, and the claw goes with it.
  pose(vm, u) {
    const k = press(u);
    const claw = vm.getObjectByName('claw');
    if (claw) claw.visible = u < 0.25 || u > 0.8; // it leaves, and the next one rides up
    vm.position.z = REST.z + k * 0.1;
    vm.rotation.x = REST.rx + k * 0.16;
    grip(vm, k * 0.4, k);
  },
  projectile(scene) {
    const g = new THREE.Group();
    g.add(flying(scene, new THREE.ConeGeometry(0.02, 0.09, 8).rotateX(Math.PI / 2).translate(0, 0, 0.05), '#9aa4b0'));
    const shaft = flyingRod(scene, 0.01, 0.01, 0.1, '#6f7884');
    shaft.rotation.x = Math.PI / 2;
    g.add(shaft);
    // Three flukes, opened out: what it looks like once it has left the barrel.
    for (let i = 0; i < 3; i++) {
      const fluke = flying(scene, new THREE.ConeGeometry(0.008, 0.075, 6).translate(0, 0.038, 0), '#b9bec6');
      fluke.rotation.set(2.3, 0, (i * Math.PI * 2) / 3);
      fluke.position.z = -0.03;
      g.add(fluke);
    }
    g.userData.aim = true;
    return g;
  },
};

// Where the jet backpack's throttle lever stands with the hand resting on it.
const THROTTLE_OUT = -0.075;

/**
 * A jet backpack, which is how you get off the ground. It is not aimed and nothing
 * leaves it: while it is the thing in your hands the pack on your back is running,
 * and the walker flies - forward where they are looking, straight up and down on the
 * jump and crouch keys. Using it opens the throttle for a moment, which is a burst of
 * speed rather than a shot.
 *
 * What is drawn is the throttle the hand is holding and the feed line running back
 * over the shoulder to the pack itself, with one of the thrusters swung into the
 * bottom of the frame - the pack is behind the camera, and a first-person view of it
 * is the part of it that reaches round.
 *
 * Implements: REQ-TOOL-023, REQ-TOOL-035, REQ-TOOL-036, REQ-TOOL-052
 */
const jetpack = {
  id: 'jetpack',
  label: 'Jet backpack',
  verb: 'Burn',
  noun: 'flown',
  hint: 'Carrying it is flying: W and S follow your eyes, Space and C go up and down, and F opens the throttle for a burst',
  reticle: 'thrust',
  slot: 9,
  kind: 'secondary',
  flies: true,
  // Long enough to cross a district and pick a roof to land on, and about half as long
  // again on the ground to fill: flying is the slow way to look at a map from above,
  // so it is worth being able to stay up there and steer rather than counting seconds.
  fuel: { full: 18, fills: 26 },
  ...PISTOL,
  viewmodel() {
    return viewmodel(g => {
      const gun = armed(g, this, tool => { tool.name = 'throttle'; });
      const frame = new THREE.Group();
      frame.position.set(0, 0.045, -0.01);
      gun.add(frame);

      // The throttle: a grip through the fist with a lever on the front of it and a
      // gauge on top, so what is held reads as a control rather than a weapon.
      frame.add(part(new THREE.BoxGeometry(0.05, 0.14, 0.05).translate(0, -0.04, 0.01), '#2b3138'));
      frame.add(part(new THREE.BoxGeometry(0.06, 0.05, 0.09).translate(0, 0.05, -0.03), '#48525c'));
      // Placed rather than built in place: the gesture moves it, and a geometry
      // already carried there would be moved from there twice.
      const lever = part(new THREE.BoxGeometry(0.03, 0.055, 0.016), '#e8a317');
      lever.position.set(0, 0.03, THROTTLE_OUT);
      lever.name = 'lever';
      frame.add(lever);
      const dial = part(new THREE.CylinderGeometry(0.017, 0.017, 0.006, 12), '#dfe4ea');
      dial.position.set(0, 0.08, -0.03);
      frame.add(dial);

      // Where everything that leaves the grip leaves it from: inside the heel of it,
      // below the fist, so a run out of there begins in the metal rather than a
      // finger's breadth off the end of it.
      const heel = new THREE.Vector3(0.012, -0.085, 0.016);

      // The feed line, out of that heel and back past the shoulder, in the braided
      // sleeve a pressure line wears. The bands are hung off the hose itself rather
      // than placed beside it, so they sit on it wherever it is pointed - laid out
      // separately they drift off it the moment the hose is angled.
      const feed = linkPart(heel, new THREE.Vector3(0.026, -0.215, 0.365), 0.013, '#3c444d');
      frame.add(feed);
      for (const t of [-0.34, -0.17, 0, 0.17, 0.34]) {
        const band = part(new THREE.TorusGeometry(0.016, 0.0035, 5, 10), '#6d7782');
        band.rotation.x = Math.PI / 2;
        band.position.y = t * feed.userData.len;
        feed.add(band);
      }

      // The thruster, reaching round from the pack into the corner of the view: a
      // tank, the shoulder it necks down through, a bell with a lip on it, and the
      // ribs and the mount that say it is bolted to something behind the walker. It is
      // set well below and outboard of the fist, because everything on it is wider
      // than the hand and any of it level with the grip is drawn through the fingers.
      const pod = new THREE.Group();
      pod.position.set(0.085, -0.36, 0.11);
      pod.rotation.set(-0.22, 0, 0.14);
      pod.updateMatrix(); // the arm below is measured against it
      frame.add(pod);
      pod.add(part(new THREE.CylinderGeometry(0.052, 0.052, 0.17, 14), '#48525c'));
      pod.add(part(new THREE.SphereGeometry(0.052, 14, 8).scale(1, 0.55, 1).translate(0, 0.085, 0), '#5a6068'));
      pod.add(part(new THREE.CylinderGeometry(0.052, 0.035, 0.05, 14).translate(0, -0.11, 0), '#3c444d'));
      pod.add(part(new THREE.CylinderGeometry(0.035, 0.062, 0.08, 14, 1, true).translate(0, -0.175, 0), '#23292f',
        { side: THREE.DoubleSide }));
      pod.add(part(new THREE.TorusGeometry(0.062, 0.005, 6, 16).rotateX(Math.PI / 2).translate(0, -0.215, 0), '#15181b'));
      for (const y of [0.04, -0.01, -0.06]) {
        pod.add(part(new THREE.TorusGeometry(0.053, 0.004, 5, 14).rotateX(Math.PI / 2).translate(0, y, 0), '#39424c'));
      }
      pod.add(part(new THREE.CylinderGeometry(0.075, 0.075, 0.008, 14).translate(0, -0.235, 0), '#2b2f34'));

      // The arm the thruster hangs on, from the heel of the grip to a collar on the
      // tank, with a boss at each end. It is built between its two ends rather than
      // aimed from one of them, because a strut aimed from one end stops wherever its
      // length runs out - which up to now was in mid-air, a hand's breadth short of
      // both the grip and the tank.
      const collar = new THREE.Vector3(-0.022, 0.05, 0).applyMatrix4(pod.matrix);
      frame.add(linkPart(heel, collar, 0.011, '#5a6068'));
      for (const end of [heel, collar]) {
        const boss = part(new THREE.SphereGeometry(0.017, 10, 8), '#48525c');
        boss.position.copy(end);
        frame.add(boss);
      }

      // The fumes: a lit core, the flame around it, and a column of exhaust below that
      // widens and thins as it goes. All three are drawn from the moment the pack is
      // taken out, because a running jet is what is holding the walker up.
      const flare = new THREE.Group();
      flare.position.y = -0.22;
      flare.name = 'flare';
      pod.add(flare);
      const soft = { transparent: true, depthWrite: false };
      const core = part(new THREE.ConeGeometry(0.026, 0.11, 10).rotateX(Math.PI).translate(0, -0.055, 0), '#ffffff',
        { ...soft, opacity: 0.85 });
      core.name = 'core';
      const flame = part(new THREE.ConeGeometry(0.05, 0.22, 12).rotateX(Math.PI).translate(0, -0.11, 0), '#7fc8f0',
        { ...soft, opacity: 0.6 });
      flame.name = 'flame';
      const wash = part(new THREE.CylinderGeometry(0.038, 0.13, 0.34, 14, 1, true).translate(0, -0.3, 0), '#bcd8ea',
        { ...soft, opacity: 0.16, side: THREE.DoubleSide });
      wash.name = 'wash';
      flare.add(wash, flame, core);
      g.userData.trigger = 0.15; // a finger laid on the lever
    });
  },
  /**
   * The fumes, which are never still: the core and the flame lick at different rates
   * so the two do not pulse as one lump, and the exhaust below them lags behind both,
   * which is what makes it read as something being blown out rather than a cone that
   * changes size. It says the pack is running without anything having to be pressed.
   */
  live(vm, scene, lens, now = performance.now()) {
    const flame = vm.getObjectByName('flame'), core = vm.getObjectByName('core');
    const wash = vm.getObjectByName('wash'), flare = vm.getObjectByName('flare');
    if (!flame) return;
    const t = now / 1000;
    // The burst the gesture opened, easing shut again: pose writes it while the
    // throttle is down and this is what closes it afterwards, so the jet settles back
    // to its idle instead of staying wide open. Eased over seconds rather than over
    // frames, or the jet would settle twice as fast on a screen running twice as fast.
    const since = Math.min(0.2, Math.max(0, (now - (vm.userData.burnAt ?? now)) / 1000));
    vm.userData.burnAt = now;
    const burn = vm.userData.burn = 1 + ((vm.userData.burn || 1) - 1) * Math.exp(-since * 9);
    const lick = 1 + Math.sin(t * 21) * 0.12 + Math.sin(t * 7.3) * 0.06;
    const flicker = 1 + Math.sin(t * 17 + 1.7) * 0.16;
    flame.scale.set(1 + (lick - 1) * 0.4, lick * burn, 1 + (lick - 1) * 0.4);
    core.scale.set(1, flicker * burn, 1);
    wash.scale.set(1 + (flicker - 1) * 0.25, (0.8 + 0.2 * lick) * burn, 1 + (flicker - 1) * 0.25);
    wash.material.opacity = 0.1 + 0.09 * Math.min(2, burn);
    // The whole plume shivers a little off the axis, the way a jet of anything does.
    flare.rotation.x = Math.sin(t * 9.3) * 0.04;
    flare.rotation.z = Math.sin(t * 11.7 + 2.1) * 0.04;
  },
  // The throttle goes forward and the jet lengthens behind it; the hand is pushed back
  // by a pack that is suddenly pulling.
  pose(vm, u) {
    const k = press(u);
    vm.getObjectByName('lever').position.z = THROTTLE_OUT - k * 0.012;
    vm.userData.burn = 1 + k * 2.4; // live() reads it, so the flame keeps flickering
    vm.position.z = REST.z + k * 0.06;
    vm.position.y = vm.userData.restY - k * 0.02;
    vm.rotation.x = REST.rx - k * 0.1;
    grip(vm, k * 0.5, k);
  },
  projectile: null, // the thrust is the walker's, not something thrown
};

// How far below the fist the float hangs, and how long it is.
const FLOAT_AT = -0.24, FLOAT_LEN = 0.34;

/**
 * Water skimmers: a pair of floats, one of them carried in your off hand and the other
 * being the one you are standing on. Carrying them is the whole of using them - the
 * water holds while they are in hand, so the bay becomes a street and the islands stop
 * being somewhere only a bridge reaches. Put them away over deep water and you are in
 * it, which is the reason to look where you are going.
 *
 * What runs through the fist is the binding strap, the way anyone carries a boot or a
 * ski: the float itself hangs below the hand, because a hull wide enough to stand on
 * is far too wide for a hand to close around.
 *
 * Implements: REQ-TOOL-025, REQ-TOOL-035, REQ-TOOL-052
 */
const skimmers = {
  id: 'skimmers',
  label: 'Water skimmers',
  verb: 'Skim',
  noun: 'skimmed',
  hint: 'Carrying them is walking on water; stow them over the bay and it will not hold you',
  reticle: 'ripple',
  slot: 10,
  kind: 'secondary',
  floats: true,
  // Floats waterlog, and quickly: the bay is a thing to cross rather than a place to
  // be, so they last a dash to the far shore and no more, and are slower to dry out
  // than a tank is to fill.
  fuel: { full: 7, fills: 18 },
  hold: { x: 0.21, y: -0.17, z: -0.52, along: [0.05, 1, 0.26], back: [0.4, -0.3, 1] },
  grip: SWUNG,
  viewmodel() {
    return viewmodel(g => {
      const float = armed(g, this, tool => { tool.name = 'float'; });

      // The strap through the fist, and the two risers down to the deck.
      float.add(rodPart(0.019, 0.019, 0.13, '#2b3138'));
      for (const side of [-1, 1]) {
        const riser = rodPart(0.01, 0.01, 0.13, '#2b3138');
        riser.position.set(side * 0.035, -0.09, 0);
        riser.rotation.z = side * 0.3;
        float.add(riser);
      }

      // The hull: long, rounded at the nose, flat on top, in the orange everything
      // meant to be found in the water is painted.
      const hull = part(new THREE.CapsuleGeometry(0.048, FLOAT_LEN - 0.1, 5, 10), '#e8641f');
      hull.position.y = FLOAT_AT;
      float.add(hull);
      const deck = part(new THREE.BoxGeometry(0.072, 0.012, 0.22)
        .translate(0, FLOAT_AT + 0.04, 0), '#f1efe8');
      deck.rotation.y = Math.PI / 2;
      float.add(deck);
      // The binding on the deck, and a skeg under the tail.
      const bind = part(new THREE.TorusGeometry(0.038, 0.008, 6, 14), '#2b3138');
      bind.rotation.x = Math.PI / 2;
      bind.position.y = FLOAT_AT + 0.05;
      float.add(bind);
      float.add(part(new THREE.BoxGeometry(0.01, 0.045, 0.07)
        .translate(0, FLOAT_AT - FLOAT_LEN / 2 - 0.01, 0.02), '#2f353c'));
    });
  },
  // Nothing is fired, so using them is a look at them: the float is turned over in
  // the hand and set down again.
  pose(vm, u) {
    const k = press(u);
    const float = vm.getObjectByName('float');
    float.rotation.z = k * 0.5;
    float.rotation.x = -k * 0.35;
    vm.position.y = vm.userData.restY + k * 0.06;
    vm.rotation.z = REST.rz - k * 0.18;
    grip(vm, 0.3 + k * 0.3);
  },
  projectile: null,
};

// Implements: REQ-TOOL-004, REQ-TOOL-005, REQ-TOOL-021
export const TOOLS = { rod, net, camera, bubbles, extinguisher, dart, nailer, grapple, jetpack, skimmers };

/**
 * The tools in slot order, which is the order they are drawn in and cycled through.
 * It is not what the number keys count along any more: the digits number the hunt's
 * row alone and the off hand is Q's, for the reasons switcher.js gives.
 */
export const TOOL_IDS = Object.values(TOOLS).sort((a, b) => a.slot - b.slot).map(t => t.id);

/** The same, split by which hand they go in: the hunt's row, and the carried row. */
export const PRIMARY_IDS = TOOL_IDS.filter(id => TOOLS[id].kind === 'primary');
export const SECONDARY_IDS = TOOL_IDS.filter(id => TOOLS[id].kind === 'secondary');

/**
 * What a tool is any use against, for the aim and for what the HUD says. A secondary
 * tool is no use against anything: it carries the walker, and nothing it touches is
 * tagged or caught. This is the one place that is decided.
 *
 * Implements: REQ-TOOL-022, REQ-TOOL-026
 */
export const hits = (tool, what) =>
  tool.kind !== 'secondary' && ((tool.targets || 'both') === 'both' || tool.targets === what);

/** Whether a tool is one that carries the walker rather than one that hunts. */
export const isSecondary = tool => tool.kind === 'secondary';

/** A tool that is swung rather than thrown: it has a reach and nothing leaves it. */
export const isMelee = tool => !tool.projectile && tool.reach != null;
export const DEFAULT_TOOL = 'rod';

export function toolFor(id) { return TOOLS[id] || TOOLS[DEFAULT_TOOL]; }
export { idle as idleTool };
