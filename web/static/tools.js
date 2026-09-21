// What walk mode puts in your hands. A tool is a viewmodel - a hand and the thing it
// holds, drawn in front of the camera - an animation for using it, and whatever
// travels to the target. Tagging a module is the same act throughout; only the
// gesture changes, so none of this touches the map or the graph.
//
// The hand and the forearm are a rigged model (hands.js, modelled by tools/hand.py in
// Blender); the tools themselves are built here. Both are lit, which nothing else on
// the map is: the walk camera carries its own lights, and they reach only what it
// holds, because every other material in the scene is unlit.
//
// Each tool provides:
//   viewmodel(scene)  the group parented to the walk camera, posed at rest
//   pose(vm, u, now)  u through the swing, 0..1
//   projectile(scene) what flies, or null when the tool acts where it is pointed
//   reticle           the aim helper's style, so it fits the tool rather than an
//                     ever-present crosshair
//   verb / noun       what the HUD and its messages call the act and its tally
//   targets           'bugs', 'buildings' or 'both': what it is any use against
//   reach             how far it works, in map units; absent means as far as it is
//                     thrown, and on a tool that throws nothing that is any distance
//                     at all (the camera). A reach with nothing thrown is a tool
//                     swung by hand - the net - which has to be walked up to.
//   flight            how what it throws behaves: speed, how much it is lobbed
//                     (arc), what gravity does to it, and how the air holds it back.
//                     A dart is fast and flat, a bubble slow and rising; the same
//                     code flies both.
//   reel              what happens to the line once it has stuck: how fast it pulls
//                     the walker along it, how close it brings them, how far away it
//                     will still pull from, and whether it sets them on top of what
//                     it caught or leaves them against it.

import * as THREE from './vendor/three.module.min.js';

import { handModel, closeHand, closeFinger, setWrist, loadHands, handsReady } from './hands.js';

/**
 * One part of a tool. Unlike everything else on the map these are lit: the walk
 * camera carries its own lights (viewLights), and since every material in the scene
 * proper is unlit, nothing but what is in the walker's hands can see them. That is
 * what gives a rod blank its highlight and a hand its roundness.
 */
function part(geo, color, opts) {
  return new THREE.Mesh(geo, new THREE.MeshPhongMaterial({
    color, shininess: 22, specular: 0x1b1b1b, ...opts,
  }));
}

// The butterfly net's mouth and how deep its bag hangs.
const HOOP = 0.145, BAG = 0.34;

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

// Skin, and the sleeve the arm comes out of.
const SKIN = '#c98d63';

const skinMaterial = () => new THREE.MeshPhongMaterial({ color: SKIN, shininess: 8, specular: 0x141414 });

/**
 * The lights the walk camera carries for its own hands: a key over the left shoulder,
 * a dim fill from the other side, a rim behind to pick the arm out of whatever is
 * behind it, and sky above and ground below instead of a flat ambient - which is what
 * stops the shaded side of a forearm from going to one dead tone. They are parented
 * to the camera, so they travel with the view, and they reach nothing else, because
 * the map is drawn with unlit materials.
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

// viewmodel places a hand and its tool where a first-person view expects them: low
// and to the right, angled towards the middle of the screen.
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

/**
 * idle gives the held tool a life of its own: a slow breath standing still, and a
 * walk cycle on top of it - the tool rises and falls and swings across as the walker's
 * weight shifts. pace is 0 standing, 1 walking, higher running.
 */
function idle(vm, now, pace = 0) {
  const t = now / 1000;
  const step = t * 6.2 * Math.max(0.6, pace);
  const breathe = Math.sin(t * 1.15) * 0.006;
  vm.position.y = (vm.userData.restY ?? REST.y) + breathe + Math.abs(Math.sin(step)) * 0.016 * pace - 0.008 * pace;
  vm.position.x = REST.x + Math.sin(step * 0.5) * 0.02 * pace;
  vm.position.z = REST.z;
  vm.rotation.x = REST.rx + Math.sin(step) * 0.022 * pace;
  vm.rotation.y = REST.ry + Math.sin(step * 0.5) * 0.03 * pace;
  vm.rotation.z = Math.sin(t * 0.82) * 0.018 + Math.sin(step * 0.5 + 1.2) * 0.05 * pace;
}

const smooth = t => t * t * (3 - 2 * t);
const clamp01 = t => Math.min(1, Math.max(0, t));

/**
 * The shape of a swing: a gesture is not a ramp. It loads backwards first
 * (anticipation), drives through the strike, and settles back - which is what makes
 * a throw look thrown rather than slid. Returns -0.55 .. 1 .. 0.
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

const rod = {
  id: 'rod',
  label: 'Fishing rod',
  verb: 'Cast at',
  noun: 'landed',
  hint: 'Cast at a building; the line hauls you to it, twice for details',
  reticle: 'bobber',
  slot: 1,
  targets: 'buildings',
  // A weighted bobber on a line: thrown hard, dropping the way a cast does.
  flight: { speed: 24, arc: 1.6, gravity: 6, drag: 0.1 },
  // The blank goes up, the arm back out of the bottom of the frame.
  hold: { x: 0.2, y: -0.28, z: -0.52, along: [-0.05, 1, 0.3], back: [0.4, -0.3, 1] },
  grip: { x: 0, y: 0, z: 0, rx: 0, ry: 0, rz: -Math.PI / 2 },
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
  projectile() {
    const g = new THREE.Group();
    g.add(part(new THREE.SphereGeometry(0.05, 12, 9, 0, Math.PI * 2, Math.PI / 2, Math.PI / 2), '#e4402f'));
    g.add(part(new THREE.SphereGeometry(0.05, 12, 9, 0, Math.PI * 2, 0, Math.PI / 2), '#f2f4f7'));
    const eye = part(new THREE.TorusGeometry(0.012, 0.004, 5, 10), '#9aa4b0');
    eye.position.y = 0.052;
    eye.rotation.x = Math.PI / 2;
    g.add(eye);
    const stem = rodPart(0.006, 0.004, 0.05, '#f2f4f7');
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

const net = {
  id: 'net',
  label: 'Butterfly net',
  verb: 'Net',
  noun: 'netted',
  hint: 'Swing at a bug you can reach; it does not throw',
  reticle: 'hoop',
  slot: 2,
  // A net catches what is in it. Throwing the whole net at a building across the
  // map was the one thing here that never made sense as a gesture.
  targets: 'bugs',
  reach: 2.1,
  hold: { x: 0.2, y: -0.28, z: -0.52, along: [0.08, 1, 0.32], back: [0.4, -0.3, 1] },
  grip: { x: 0, y: 0, z: 0, rx: 0, ry: 0, rz: -Math.PI / 2 },
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
  // A wind-up away from the view, then a sweep across it, the head trailing the hand.
  pose(vm, u) {
    const k = swing(u);
    const net = vm.getObjectByName('net');
    const head = vm.getObjectByName('head');
    vm.rotation.z = REST.rz - k * 1.15;
    vm.rotation.y = REST.ry + k * 0.55;
    vm.rotation.x = REST.rx - k * 0.1;
    vm.position.x = REST.x - k * 0.24;
    vm.position.y = vm.userData.restY + Math.max(0, k) * 0.05;
    net.rotation.z = k * 0.35;
    head.rotation.z = -k * 0.45; // the bag lags behind the hoop
    grip(vm, Math.abs(k) * 0.8);
  },
  projectile: null, // the net stays on the stick
};

// How much smaller than life the camera body is drawn, so one hand can hold it.
const SHELL = 0.78;

const camera = {
  id: 'camera',
  label: 'Camera',
  verb: 'Photograph',
  noun: 'photographed',
  hint: 'Photograph a building or a bug, at any range',
  reticle: 'frame',
  slot: 3,
  // A photograph records whatever is in the frame, near or far, bug or building.
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
        new THREE.PlaneGeometry(0.135, 0.097).translate(-0.035, -0.012, 0.063),
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
   * The live view, once a frame: the world drawn again from the lens into a small
   * texture and hung on the back of the camera. Only this tool has one, and only
   * while it is the tool in hand - it is a second pass over the whole map, which is
   * worth it for the one thing here that is supposed to be looking at something.
   */
  live(vm, scene, lens) {
    const screen = vm.getObjectByName('screen');
    if (!screen) return;
    const texture = scene.film(lens);
    if (screen.material.map !== texture) {
      screen.material.map = texture;
      screen.material.color.set('#ffffff');
      screen.material.needsUpdate = true;
    }
  },
};

const bubbles = {
  id: 'bubbles',
  label: 'Bubble wand',
  verb: 'Bubble',
  noun: 'bubbled',
  hint: 'Float a bubble onto a bug; it rises and drifts',
  slot: 4,
  // Soap on a beetle is a catch; soap on a wall is a clean wall.
  targets: 'bugs',
  // A bubble is lighter than the air it is thrown through: it slows almost at once
  // and then climbs, which is why it is lobbed high and aimed early.
  flight: { speed: 12, arc: 2.2, gravity: -1.1, drag: 1.5 },
  reticle: 'soft',
  hold: { x: 0.2, y: -0.28, z: -0.52, along: [0.05, 1, 0.32], back: [0.4, -0.3, 1] },
  grip: { x: 0, y: 0, z: 0, rx: 0, ry: 0, rz: -Math.PI / 2 },
  viewmodel() {
    return viewmodel(g => {
      const wand = armed(g, this, tool => { tool.name = 'wand'; });

      const cap = rodPart(0.03, 0.028, 0.17, '#2f7fb8'); // the bottle cap it screws into
      cap.position.y = -0.03;
      wand.add(cap);
      const stick = rodPart(0.01, 0.012, 0.3, '#59b0e6');
      stick.position.y = 0.2;
      wand.add(stick);

      const head = new THREE.Group();
      head.position.y = 0.36;
      head.rotation.x = -0.3;
      head.name = 'head';
      wand.add(head);
      const ring = part(new THREE.TorusGeometry(0.062, 0.0075, 6, 18), '#59b0e6');
      ring.rotation.x = Math.PI / 2;
      ring.name = 'ring';
      head.add(ring);
      // The soap film across the ring, which stretches as the wand is waved.
      const film = part(new THREE.CircleGeometry(0.057, 18), '#cdeaff',
        { transparent: true, opacity: 0.3, side: THREE.DoubleSide });
      film.rotation.x = Math.PI / 2;
      film.name = 'film';
      head.add(film);
      muzzle(head, 0, 0, 0);
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
  projectile() {
    const g = new THREE.Group();
    g.add(part(new THREE.SphereGeometry(0.11, 16, 12), '#bfe6ff', { transparent: true, opacity: 0.4 }));
    const rim = part(new THREE.SphereGeometry(0.111, 16, 12), '#ffffff',
      { transparent: true, opacity: 0.22, side: THREE.BackSide });
    const sheen = part(new THREE.SphereGeometry(0.035, 10, 8), '#ffffff', { transparent: true, opacity: 0.65 });
    sheen.position.set(0.042, 0.045, 0.04);
    g.add(rim, sheen);
    g.userData.wobble = true;
    return g;
  },
};

const dart = {
  id: 'dart',
  label: 'Tracking dart',
  verb: 'Tag',
  noun: 'tagged',
  hint: 'Tag a building; hit it again for details',
  reticle: 'scope',
  slot: 5,
  targets: 'buildings',
  // Heavy, fast and barely lobbed: the flattest thing in the bag.
  flight: { speed: 34, arc: 0.6, gravity: 7, drag: 0.05 },
  // The pistol grip hangs down out of the fist, so the shaft through it points up.
  hold: { x: 0.19, y: -0.24, z: -0.5, along: [0.05, 1, 0.1], back: [0.45, -0.3, 1] },
  // Pointed down the view, a few degrees off it. Held square across the frame - which
  // is what this was - the barrel aimed sixteen degrees wide of the reticle and the
  // dart left it sideways; end-on it would be a dark blob instead. These are the
  // angles that put the barrel eleven degrees off the aim with the scope on top,
  // solved from the pose rather than guessed at, so the flank still reads.
  grip: { x: 0, y: 0, z: 0, rx: -3.124, ry: 0.117, rz: -1.599 },
  restGrip: 0.88,
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
  projectile() {
    const g = new THREE.Group();
    const body = rodPart(0.009, 0.009, 0.24, '#2b2d31');
    body.rotation.x = Math.PI / 2;
    g.add(body);
    const tip = part(new THREE.ConeGeometry(0.019, 0.07, 10).rotateX(Math.PI / 2).translate(0, 0, 0.155), '#ff8a1f');
    g.add(tip);
    const collar = part(new THREE.TorusGeometry(0.012, 0.004, 5, 10), '#9aa4b0');
    collar.position.z = 0.1;
    g.add(collar);
    for (let i = 0; i < 3; i++) {
      const fin = part(new THREE.BoxGeometry(0.004, 0.05, 0.07).translate(0, 0.025, -0.1), '#d8dadf');
      fin.rotation.z = (i * Math.PI * 2) / 3;
      g.add(fin);
    }
    g.userData.aim = true; // points along its flight
    return g;
  },
};

// ------------------------------------------------- what the buildings are for

/**
 * A framing nailer: the construction trade's answer to the tracking dart. Nails go
 * into buildings and nothing else, they go fast and nearly flat, and the magazine
 * means they go one after another - a wall of a warehouse can be pinned at a run in a
 * way a single dart cannot.
 */
const nailer = {
  id: 'nailer',
  label: 'Nail gun',
  verb: 'Pin',
  noun: 'pinned',
  hint: 'Drive a nail into a building; hit it again for details',
  reticle: 'cross',
  slot: 6,
  targets: 'buildings',
  // Fired rather than thrown: the fastest and flattest thing here, and heavy enough
  // that the air does nothing to it.
  flight: { speed: 70, arc: 0.1, gravity: 3.5, drag: 0 },
  hold: { x: 0.19, y: -0.24, z: -0.5, along: [0.05, 1, 0.1], back: [0.45, -0.3, 1] },
  grip: { x: 0, y: 0, z: 0, rx: -3.124, ry: 0.117, rz: -1.599 },
  restGrip: 0.88,
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
  projectile() {
    const g = new THREE.Group();
    const shank = rodPart(0.006, 0.006, 0.12, '#c9ced6');
    shank.rotation.x = Math.PI / 2;
    g.add(shank);
    g.add(part(new THREE.ConeGeometry(0.007, 0.03, 8).rotateX(Math.PI / 2).translate(0, 0, 0.072), '#e6eaef'));
    const head = part(new THREE.CylinderGeometry(0.014, 0.014, 0.006, 10), '#aab2bb');
    head.rotation.x = Math.PI / 2;
    head.position.z = -0.062;
    g.add(head);
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
  hint: 'Hook a building to be pulled onto its roof; from up there, hook lower to come down',
  reticle: 'hook',
  slot: 7,
  targets: 'buildings',
  // A line paid out taut: no lob and no drop worth speaking of.
  flight: { speed: 55, arc: 0.05, gravity: 0.6, drag: 0 },
  line: '#cfd6de', // the line stays drawn, out and back
  // A winch rather than a rod: faster, right up to what it caught, from any range,
  // and it sets the walker on top of it. Catching nothing else is the point of the
  // tool, so arriving is all it does (walk.js).
  reel: { speed: 17, stop: 0.25, max: Infinity, onto: true },
  climbs: true, // ... and nothing is tagged or caught when it lands
  hold: { x: 0.19, y: -0.24, z: -0.5, along: [0.05, 1, 0.1], back: [0.45, -0.3, 1] },
  grip: { x: 0, y: 0, z: 0, rx: -3.124, ry: 0.117, rz: -1.599 },
  restGrip: 0.88,
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
  projectile() {
    const g = new THREE.Group();
    g.add(part(new THREE.ConeGeometry(0.02, 0.09, 8).rotateX(Math.PI / 2).translate(0, 0, 0.05), '#9aa4b0'));
    const shaft = rodPart(0.01, 0.01, 0.1, '#6f7884');
    shaft.rotation.x = Math.PI / 2;
    g.add(shaft);
    // Three flukes, opened out: what it looks like once it has left the barrel.
    for (let i = 0; i < 3; i++) {
      const fluke = part(new THREE.ConeGeometry(0.008, 0.075, 6).translate(0, 0.038, 0), '#b9bec6');
      fluke.rotation.set(2.3, 0, (i * Math.PI * 2) / 3);
      fluke.position.z = -0.03;
      g.add(fluke);
    }
    g.userData.aim = true;
    return g;
  },
};

export const TOOLS = { rod, net, camera, bubbles, dart, nailer, grapple };

/** The tools in slot order, which is the order the number keys pick them in. */
export const TOOL_IDS = Object.values(TOOLS).sort((a, b) => a.slot - b.slot).map(t => t.id);

/** What a tool is any use against, for the aim and for what the HUD says. */
export const hits = (tool, what) => (tool.targets || 'both') === 'both' || tool.targets === what;

/** A tool that is swung rather than thrown: it has a reach and nothing leaves it. */
export const isMelee = tool => !tool.projectile && tool.reach != null;
export const DEFAULT_TOOL = 'rod';

export function toolFor(id) { return TOOLS[id] || TOOLS[DEFAULT_TOOL]; }
export { idle as idleTool };
