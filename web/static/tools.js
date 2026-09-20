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
//   projectile(scene) what flies, or null when the tool acts at once (the camera)
//   reticle           the aim helper's style, so it fits the tool rather than an
//                     ever-present crosshair
//   verb / noun       what the HUD and its messages call the act and its tally

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

// A tube between two points, for rod blanks, handles and strap runs.
function rodPart(r0, r1, len, color) {
  return part(new THREE.CylinderGeometry(r0, r1, len, 12), color);
}

// Skin, and the sleeve the arm comes out of.
const SKIN = '#c98d63';

const skinMaterial = () => new THREE.MeshPhongMaterial({ color: SKIN, shininess: 8, specular: 0x141414 });

/**
 * The lights the walk camera carries for its own hands: a key over the left shoulder,
 * a dim fill from the other side so nothing goes black, and enough ambient that the
 * shadowed side still reads. They are parented to the camera, so they travel with the
 * view - and they reach nothing else, because the map is drawn with unlit materials.
 */
export function viewLights() {
  const g = new THREE.Group();
  const key = new THREE.DirectionalLight(0xfff4e6, 2.1);
  key.position.set(-0.5, 0.9, 0.6);
  const fill = new THREE.DirectionalLight(0xbcd2ea, 0.75);
  fill.position.set(0.8, -0.2, 0.35);
  g.add(key, fill, new THREE.AmbientLight(0xffffff, 0.55));
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
    const tool = new THREE.Group();
    tool.position.set(grip.x, grip.y, grip.z);
    tool.rotation.set(grip.rx, grip.ry, grip.rz);
    build(tool);
    holder.add(tool);
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
  hint: 'Click: cast at a building, land it twice for details',
  reticle: 'bobber',
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
    rod.rotation.x = -1.15 + k * 1.15;
    rod.rotation.z = 0.25 - k * 0.12;
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
  speed: 24,
  arc: 1.6,
};

const net = {
  id: 'net',
  label: 'Butterfly net',
  verb: 'Net',
  noun: 'netted',
  hint: 'Click: net a building, net it twice for details',
  reticle: 'hoop',
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

      const head = new THREE.Group();
      head.position.y = 0.47;
      head.rotation.x = -0.25;
      head.name = 'head';
      net.add(head);
      const hoop = part(new THREE.TorusGeometry(0.135, 0.0075, 6, 22), '#d8dee6');
      hoop.position.y = 0.12;
      hoop.rotation.x = Math.PI / 2;
      head.add(hoop);
      // The bag, with two stiffening rings so it is not one smooth cone.
      const bag = part(new THREE.ConeGeometry(0.132, 0.24, 16, 1, true), '#eef3f8',
        { transparent: true, opacity: 0.42, side: THREE.DoubleSide });
      bag.position.y = 0.24;
      bag.rotation.x = Math.PI;
      head.add(bag);
      for (const [y, r] of [[0.19, 0.105], [0.145, 0.07]]) {
        const ring = part(new THREE.TorusGeometry(r, 0.0035, 5, 16), '#dfe6ee',
          { transparent: true, opacity: 0.55 });
        ring.position.y = y;
        ring.rotation.x = Math.PI / 2;
        head.add(ring);
      }
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
    net.rotation.z = 0.2 + k * 0.35;
    head.rotation.z = -k * 0.45; // the bag lags behind the hoop
    grip(vm, Math.abs(k) * 0.8);
  },
  projectile() {
    const g = new THREE.Group();
    g.add(part(new THREE.TorusGeometry(0.16, 0.012, 6, 20), '#d8dee6'));
    const bag = part(new THREE.ConeGeometry(0.155, 0.28, 14, 1, true), '#eef3f8',
      { transparent: true, opacity: 0.38, side: THREE.DoubleSide });
    bag.position.z = -0.14;
    bag.rotation.x = Math.PI / 2;
    g.add(bag);
    const ring = part(new THREE.TorusGeometry(0.085, 0.005, 5, 14), '#dfe6ee', { transparent: true, opacity: 0.5 });
    ring.position.z = -0.13;
    g.add(ring);
    g.userData.spin = 6;
    return g;
  },
  speed: 20,
  arc: 1.1,
};

// How much smaller than life the camera body is drawn, so one hand can hold it.
const SHELL = 0.78;

const camera = {
  id: 'camera',
  label: 'Camera',
  verb: 'Photograph',
  noun: 'photographed',
  hint: 'Click: photograph a building, twice for details',
  reticle: 'frame',
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
  speed: 0,
  arc: 0,
};

const bubbles = {
  id: 'bubbles',
  label: 'Bubble wand',
  verb: 'Bubble',
  noun: 'bubbled',
  hint: 'Click: send a bubble at a building, twice for details',
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
    wand.rotation.z = 0.15 + Math.sin(u * Math.PI * 2) * 0.25;
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
  speed: 12, // a bubble takes its time
  arc: 2.2,
};

const dart = {
  id: 'dart',
  label: 'Tracking dart',
  verb: 'Tag',
  noun: 'tagged',
  hint: 'Click: tag a building, hit it again for details',
  reticle: 'scope',
  // Held across the view rather than pointed down it: a launcher seen end-on is a
  // dark blob, and half the point of it is that it looks like something.
  // The pistol grip hangs down out of the fist, so the shaft through it points up.
  hold: { x: 0.19, y: -0.24, z: -0.5, along: [0.05, 1, 0.1], back: [0.45, -0.3, 1] },
  grip: { x: 0, y: 0, z: 0, rx: 3.9, ry: 0, rz: -Math.PI / 2 },
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
      const muzzle = rodPart(0.036, 0.034, 0.04, '#23292f');
      muzzle.rotation.x = Math.PI / 2;
      muzzle.position.set(0, 0.05, -0.44);
      frame.add(muzzle);
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
  speed: 34,
  arc: 0.6,
};

export const TOOLS = { rod, net, camera, bubbles, dart };
export const TOOL_IDS = Object.keys(TOOLS);
export const DEFAULT_TOOL = 'rod';

export function toolFor(id) { return TOOLS[id] || TOOLS[DEFAULT_TOOL]; }
export { idle as idleTool };
