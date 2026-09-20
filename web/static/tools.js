// What walk mode puts in your hands. A tool is a viewmodel - a hand and the thing it
// holds, drawn in front of the camera - an animation for using it, and whatever
// travels to the target. Tagging a module is the same act throughout; only the
// gesture changes, so none of this touches the map or the graph.
//
// The map has no lights, so a cylinder drawn plainly is a flat silhouette. Every part
// here has a soft key light baked into its vertex colors instead (shade): the light
// travels with the object, which is exactly right for something held in front of the
// camera, and it costs nothing at draw time.
//
// Each tool provides:
//   viewmodel(scene)  the group parented to the walk camera, posed at rest
//   pose(vm, u, now)  u through the swing, 0..1
//   projectile(scene) what flies, or null when the tool acts at once (the camera)
//   reticle           the aim helper's style, so it fits the tool rather than an
//                     ever-present crosshair
//   verb / noun       what the HUD and its messages call the act and its tally

import * as THREE from './vendor/three.module.min.js';

const SKIN = '#cb9264', SKIN_DARK = '#a46f48', SLEEVE = '#3f6d8f', SLEEVE_DARK = '#2c5670';

// Where the baked light comes from: over the walker's left shoulder, which is where a
// first-person view expects it.
const LIGHT = new THREE.Vector3(-0.38, 0.84, 0.39).normalize();

/**
 * shade bakes that light into a geometry's vertex colors. It wraps around the whole
 * object (no hard terminator), because low-poly geometry with a sharp light on it
 * reads as facets rather than as form.
 */
function shade(geo, ambient = 0.5, gain = 0.66) {
  const n = geo.attributes.normal;
  const c = new Float32Array(n.count * 3);
  for (let i = 0; i < n.count; i++) {
    const d = n.getX(i) * LIGHT.x + n.getY(i) * LIGHT.y + n.getZ(i) * LIGHT.z;
    const s = ambient + gain * Math.pow(0.5 + 0.5 * d, 1.35);
    c[i * 3] = c[i * 3 + 1] = c[i * 3 + 2] = s;
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(c, 3));
  return geo;
}

/** One shaded part. opts go to the material (transparency, opacity, side). */
function part(geo, color, opts) {
  return new THREE.Mesh(shade(geo), new THREE.MeshBasicMaterial({ color, vertexColors: true, ...opts }));
}

// A tube between two points, for rod blanks, handles and strap runs.
function rodPart(r0, r1, len, color) {
  return part(new THREE.CylinderGeometry(r0, r1, len, 10), color);
}

// ------------------------------------------------------------------ the hand

/**
 * A first-person hand: a palm with a knuckle ridge, four fingers of two segments each
 * curled over the grip, a two-segment thumb across them, and a wrist that runs back
 * into a sleeve, so the arm does not end in nothing. The finger groups are named, so a
 * gesture can flex them.
 */
function hand(mirror = 1) {
  const g = new THREE.Group();

  const palm = part(new THREE.BoxGeometry(0.094, 0.048, 0.12), SKIN);
  palm.position.z = 0.005;
  g.add(palm);
  // The heel of the hand, a little narrower and in shadow under the palm.
  g.add(part(new THREE.BoxGeometry(0.082, 0.044, 0.055).translate(0, -0.004, -0.072), SKIN_DARK));
  // The knuckles: a ridge across the front of the palm, which is what makes a fist
  // read as a fist rather than as a block.
  g.add(part(new THREE.SphereGeometry(0.03, 10, 6).scale(1.5, 0.72, 0.78).translate(0, 0.016, 0.052), SKIN));

  const fingers = new THREE.Group();
  fingers.position.set(0, 0.006, 0.055);
  for (let i = 0; i < 4; i++) {
    const f = new THREE.Group();
    // The middle fingers are the longest; the little finger sits lowest.
    const len = 0.046 - Math.abs(i - 1.35) * 0.005;
    f.position.set((i - 1.5) * 0.0235 * mirror, -Math.abs(i - 1.3) * 0.003, 0);
    f.add(part(new THREE.BoxGeometry(0.02, 0.022, len).translate(0, 0, len * 0.5), SKIN));
    const tip = new THREE.Group();
    tip.position.z = len;
    tip.rotation.x = 1.6;
    tip.add(part(new THREE.BoxGeometry(0.019, 0.021, len * 0.62).translate(0, 0, len * 0.31), SKIN_DARK));
    f.add(tip);
    f.rotation.x = -1.28 - i * 0.05;
    f.userData.rest = f.rotation.x;
    fingers.add(f);
  }
  fingers.name = 'fingers';
  g.add(fingers);

  const thumb = new THREE.Group();
  thumb.position.set(mirror * 0.044, 0.012, 0.022);
  thumb.rotation.set(-0.95, 0, mirror * 0.6);
  thumb.add(part(new THREE.BoxGeometry(0.024, 0.024, 0.042).translate(0, 0, 0.021), SKIN));
  const thumbTip = new THREE.Group();
  thumbTip.position.z = 0.042;
  thumbTip.rotation.x = 0.75;
  thumbTip.add(part(new THREE.BoxGeometry(0.022, 0.022, 0.034).translate(0, 0, 0.017), SKIN_DARK));
  thumb.add(thumbTip);
  thumb.name = 'thumb';
  g.add(thumb);

  // Wrist, sleeve and a cuff band where the two meet.
  const wrist = rodPart(0.036, 0.042, 0.06, SKIN_DARK);
  wrist.rotation.x = Math.PI / 2;
  wrist.position.z = -0.095;
  g.add(wrist);
  const sleeve = rodPart(0.05, 0.058, 0.16, SLEEVE);
  sleeve.rotation.x = Math.PI / 2;
  sleeve.position.z = -0.2;
  g.add(sleeve);
  const cuff = rodPart(0.053, 0.053, 0.03, SLEEVE_DARK);
  cuff.rotation.x = Math.PI / 2;
  cuff.position.z = -0.128;
  g.add(cuff);
  return g;
}

// Where a shaft passes through the fist, in the hand's own coordinates: the tools put
// their grips here, so the hand holds them rather than floating beside them.
export const GRIP = { x: 0, y: 0.015, z: 0.025 };

/** Curls the fingers of every hand towards a fist: 0 at rest, 1 gripping hard. */
function grip(vm, amount) {
  vm.traverse(o => {
    if (o.name !== 'fingers') return;
    for (const f of o.children) f.rotation.x = f.userData.rest - amount * 0.3;
  });
}

// ------------------------------------------------------------------ the viewmodel

const REST = { x: 0.26, y: -0.24, z: -0.55, rx: 0.1, ry: -0.25, rz: 0 };

// viewmodel places a hand and its tool where a first-person view expects them: low
// and to the right, angled towards the middle of the screen.
function viewmodel(build) {
  const g = new THREE.Group();
  g.position.set(REST.x, REST.y, REST.z);
  g.rotation.set(REST.rx, REST.ry, REST.rz);
  build(g);
  g.traverse(o => { o.frustumCulled = false; o.renderOrder = 10; });
  g.userData.restY = REST.y;
  return g;
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
  viewmodel() {
    return viewmodel(g => {
      g.add(hand());
      // The rod, on a pivot in the fist, so the grip runs through the hand and the
      // whole thing swings from there.
      const rod = new THREE.Group();
      rod.position.set(GRIP.x, GRIP.y, GRIP.z);
      rod.rotation.set(-1.2, 0, 0.22);
      rod.name = 'rod';
      g.add(rod);

      const cork = rodPart(0.022, 0.024, 0.2, '#c69a63');
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
  viewmodel() {
    return viewmodel(g => {
      g.add(hand());
      const net = new THREE.Group();
      net.position.set(GRIP.x, GRIP.y, GRIP.z);
      net.rotation.set(-1.28, 0, 0.18);
      net.name = 'net';
      g.add(net);

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

const camera = {
  id: 'camera',
  label: 'Camera',
  verb: 'Photograph',
  noun: 'photographed',
  hint: 'Click: photograph a building, twice for details',
  reticle: 'frame',
  viewmodel() {
    return viewmodel(g => {
      // The right hand is on the camera's own grip; the left comes up under the lens,
      // which is how a camera is actually held.
      const right = hand();
      right.position.set(0.1, 0.095, -0.115);
      right.rotation.set(0.25, -0.85, -0.4);
      g.add(right);
      const left = hand(-1);
      left.position.set(-0.15, 0.03, -0.31);
      left.rotation.set(0.3, 1.0, 0.5);
      left.name = 'left';
      g.add(left);

      const cam = new THREE.Group();
      cam.position.set(-0.04, 0.12, -0.19);
      cam.name = 'cam';
      g.add(cam);

      const body = part(new THREE.BoxGeometry(0.27, 0.165, 0.11), '#2c3138');
      cam.add(body);
      cam.add(part(new THREE.BoxGeometry(0.06, 0.15, 0.115).translate(0.105, -0.005, 0.005), '#20242a')); // the grip
      cam.add(part(new THREE.BoxGeometry(0.27, 0.03, 0.112).translate(0, 0.045, 0.002), '#3a4149'));      // a leatherette band
      // The prism hump, with the viewfinder behind it.
      cam.add(part(new THREE.BoxGeometry(0.09, 0.05, 0.08).translate(-0.02, 0.105, 0.005), '#262b31'));
      cam.add(part(new THREE.BoxGeometry(0.05, 0.035, 0.012).translate(-0.02, 0.1, 0.056), '#0d0f12'));
      const barrel = rodPart(0.062, 0.07, 0.115, '#1a1d21');
      barrel.rotation.x = Math.PI / 2;
      barrel.position.z = -0.11;
      cam.add(barrel);
      const focus = rodPart(0.073, 0.073, 0.03, '#2f353c'); // the focus ring, ribbed by its facets
      focus.rotation.x = Math.PI / 2;
      focus.position.z = -0.115;
      cam.add(focus);
      const hood = rodPart(0.078, 0.07, 0.03, '#15181b');
      hood.rotation.x = Math.PI / 2;
      hood.position.z = -0.175;
      cam.add(hood);
      const glass = part(new THREE.SphereGeometry(0.056, 14, 8, 0, Math.PI * 2, 0, Math.PI / 2), '#5c9be0');
      glass.rotation.x = -Math.PI / 2;
      glass.position.z = -0.168;
      cam.add(glass);
      const button = rodPart(0.019, 0.019, 0.022, '#e4402f');
      button.position.set(0.1, 0.09, 0.01);
      button.name = 'button';
      cam.add(button);
      const dial = rodPart(0.028, 0.028, 0.02, '#40474f');
      dial.position.set(0.045, 0.092, -0.01);
      cam.add(dial);
      // The flash, which brightens for a frame when the shutter goes.
      const flash = part(new THREE.BoxGeometry(0.05, 0.016, 0.02).translate(-0.02, 0.13, 0.0), '#e8eef6');
      flash.name = 'flash';
      cam.add(flash);
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
    grip(vm, k * 0.6);
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
  viewmodel() {
    return viewmodel(g => {
      g.add(hand());
      const wand = new THREE.Group();
      wand.position.set(GRIP.x, GRIP.y, GRIP.z);
      wand.rotation.set(-1.28, 0, 0.15);
      wand.name = 'wand';
      g.add(wand);

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
  viewmodel() {
    return viewmodel(g => {
      g.add(hand());
      const gun = new THREE.Group();
      // Held across the view rather than pointed down it: a launcher seen end-on is a
      // dark blob, and half the point of it is that it looks like something.
      gun.position.set(GRIP.x - 0.035, GRIP.y - 0.015, GRIP.z + 0.01);
      gun.rotation.set(-0.06, 0.62, 0.07);
      gun.name = 'gun';
      g.add(gun);

      const body = part(new THREE.BoxGeometry(0.078, 0.078, 0.2).translate(0, 0.05, -0.12), '#4d5761');
      gun.add(body);
      const barrel = rodPart(0.028, 0.032, 0.3, '#39424c');
      barrel.rotation.x = Math.PI / 2;
      barrel.position.set(0, 0.05, -0.3);
      gun.add(barrel);
      const muzzle = rodPart(0.036, 0.034, 0.04, '#23292f');
      muzzle.rotation.x = Math.PI / 2;
      muzzle.position.set(0, 0.05, -0.44);
      gun.add(muzzle);
      // A charged air cylinder along the barrel, on the side that faces the view.
      const tank = rodPart(0.024, 0.024, 0.18, '#4c8ab2');
      tank.rotation.x = Math.PI / 2;
      tank.position.set(-0.042, 0.03, -0.2);
      gun.add(tank);
      const valve = rodPart(0.014, 0.014, 0.03, '#b9bec6');
      valve.rotation.x = Math.PI / 2;
      valve.position.set(-0.042, 0.03, -0.3);
      gun.add(valve);
      // The grip, running down through the fist.
      gun.add(part(new THREE.BoxGeometry(0.052, 0.14, 0.05).translate(0, -0.045, 0.01), '#262b31'));
      gun.add(part(new THREE.BoxGeometry(0.02, 0.035, 0.012).translate(0, 0.015, -0.055), '#15181b')); // the trigger guard
      // The scope, on two mounts.
      const scope = rodPart(0.022, 0.022, 0.19, '#2f353c');
      scope.rotation.x = Math.PI / 2;
      scope.position.set(0, 0.115, -0.2);
      gun.add(scope);
      const bell = rodPart(0.03, 0.026, 0.045, '#2f353c');
      bell.rotation.x = Math.PI / 2;
      bell.position.set(0, 0.115, -0.31);
      gun.add(bell);
      const lens = part(new THREE.CircleGeometry(0.026, 14), '#7fb8ff');
      lens.position.set(0, 0.115, -0.333);
      gun.add(lens);
      for (const z of [-0.14, -0.26]) {
        gun.add(part(new THREE.BoxGeometry(0.016, 0.04, 0.018).translate(0, 0.085, z), '#2b3138'));
      }
      // A magazine of darts, one of them showing.
      const mag = part(new THREE.BoxGeometry(0.05, 0.05, 0.05).translate(0, 0.095, -0.08), '#39424c');
      gun.add(mag);
      const loaded = rodPart(0.008, 0.008, 0.06, '#ff8a1f');
      loaded.rotation.x = Math.PI / 2;
      loaded.position.set(0, 0.095, -0.125);
      loaded.name = 'loaded';
      gun.add(loaded);
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
    grip(vm, k);
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
