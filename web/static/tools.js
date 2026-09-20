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
 * object (no hard terminator), because geometry with a sharp light on it reads as
 * facets rather than as form. warm reddens the vertices towards the far end of the
 * shape: skin carries more blood at the knuckles and fingertips than at the wrist,
 * and that gradient is most of what makes a hand look like flesh rather than plastic.
 */
function shade(geo, { ambient = 0.5, gain = 0.66, warm = 0 } = {}) {
  const n = geo.attributes.normal, pos = geo.attributes.position;
  geo.computeBoundingBox();
  const { min, max } = geo.boundingBox;
  const span = Math.max(1e-6, max.z - min.z);
  const c = new Float32Array(n.count * 3);
  for (let i = 0; i < n.count; i++) {
    const d = n.getX(i) * LIGHT.x + n.getY(i) * LIGHT.y + n.getZ(i) * LIGHT.z;
    const s = ambient + gain * Math.pow(0.5 + 0.5 * d, 1.35);
    const t = warm * (pos.getZ(i) - min.z) / span;
    c[i * 3] = s * (1 + 0.22 * t);
    c[i * 3 + 1] = s * (1 - 0.06 * t);
    c[i * 3 + 2] = s * (1 - 0.16 * t);
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(c, 3));
  return geo;
}

/** One shaded part. opts go to the material (transparency, opacity, side). */
function part(geo, color, opts, shading) {
  return new THREE.Mesh(shade(geo, shading), new THREE.MeshBasicMaterial({ color, vertexColors: true, ...opts }));
}

// A tube between two points, for rod blanks, handles and strap runs.
function rodPart(r0, r1, len, color) {
  return part(new THREE.CylinderGeometry(r0, r1, len, 10), color);
}

/**
 * limb builds a smooth tube through a list of elliptical cross-sections along z,
 * each {z, rx, ry} and optionally offset by {x, y}. It is what every part of an arm
 * is: a forearm is one that swells and flattens towards the wrist, a finger three
 * short ones that taper, a palm a wide flat one. Both ends are closed and the normals
 * are averaged, so shade's light runs around it without a seam.
 */
function limb(sections, seg = 14) {
  const pos = [], idx = [];
  for (const s of sections) {
    for (let i = 0; i < seg; i++) {
      const a = (i / seg) * Math.PI * 2;
      pos.push((s.x || 0) + Math.cos(a) * s.rx, (s.y || 0) + Math.sin(a) * s.ry, s.z);
    }
  }
  for (let r = 0; r < sections.length - 1; r++) {
    for (let i = 0; i < seg; i++) {
      const a = r * seg + i, b = r * seg + (i + 1) % seg;
      idx.push(a, b, a + seg, b, b + seg, a + seg);
    }
  }
  const first = sections[0], last = sections[sections.length - 1];
  const capA = pos.length / 3;
  pos.push(first.x || 0, first.y || 0, first.z);
  const capB = capA + 1;
  pos.push(last.x || 0, last.y || 0, last.z);
  const tail = (sections.length - 1) * seg;
  for (let i = 0; i < seg; i++) {
    idx.push(capA, i, (i + 1) % seg);
    idx.push(capB, tail + (i + 1) % seg, tail + i);
  }
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
  g.setIndex(idx);
  g.computeVertexNormals();
  return g;
}

/** A soft pad - the heel of a thumb, the ball of a palm - as a squashed sphere. */
const pad = (rx, ry, rz, x, y, z) =>
  new THREE.SphereGeometry(1, 12, 8).scale(rx, ry, rz).translate(x, y, z);

// ------------------------------------------------------------------ the hand

// A finger's three bones, as fractions of its length, and how thick it is at each
// joint. Real fingers taper and their joints bulge; a tube that does neither is a
// sausage, which is what the hand looked like before.
const PHALANX = [0.42, 0.33, 0.25];

/**
 * One finger: three tapering segments on nested joints, each joint carrying its rest
 * angle so a grip can close the whole chain, and a nail on the last one.
 */
function finger(len, girth, curl) {
  const root = new THREE.Group();
  let host = root, z = 0;
  for (let j = 0; j < 3; j++) {
    const seg = len * PHALANX[j];
    const r0 = girth * (1 - j * 0.1), r1 = girth * (1 - (j + 1) * 0.12);
    const bone = new THREE.Group();
    bone.position.z = z;
    bone.rotation.x = curl[j];
    bone.userData.rest = curl[j];
    bone.userData.joint = j;
    // The knuckle at the near end, then the shaft, tapering.
    bone.add(part(limb([
      { z: -r0 * 0.5, rx: r0 * 0.88, ry: r0 * 0.92 },
      { z: 0, rx: r0 * 1.05, ry: r0 * 1.08 },
      { z: seg * 0.45, rx: (r0 + r1) * 0.5, ry: (r0 + r1) * 0.52 },
      { z: seg, rx: r1, ry: r1 * 1.04 },
    ], 12), SKIN, undefined, { warm: 1 }));
    if (j === 2) {
      // The tip is rounded, and carries a nail.
      bone.add(part(new THREE.SphereGeometry(r1, 12, 8).scale(1, 1.04, 1.1).translate(0, 0, seg), SKIN,
        undefined, { warm: 1 }));
      bone.add(part(pad(r1 * 0.62, r1 * 0.3, seg * 0.42, 0, r1 * 0.75, seg * 0.62), '#e6c3ad'));
    }
    host.add(bone);
    host = bone;
    z = seg;
  }
  return root;
}

// Where the hand ends and the arm begins.
const WRIST = new THREE.Vector3(0, 0, -0.05);

/**
 * The forearm and the sleeve over it: a flattened wrist swelling into the belly of the
 * arm, then a rolled cuff. It falls away to the side as it goes back, so it leaves the
 * frame at the corner the way an arm attached to the viewer does, instead of receding
 * to a point in the middle of the view.
 */
function arm(mirror = 1) {
  const g = new THREE.Group();
  const back = (t, f) => ({ z: -t * 0.5, x: mirror * t * t * 0.34, y: -t * t * 0.78, ...f });
  g.add(part(limb([
    { z: 0.06, rx: 0.042, ry: 0.032 },
    back(0.0, { rx: 0.040, ry: 0.031 }),
    back(0.16, { rx: 0.048, ry: 0.040 }),
    back(0.34, { rx: 0.062, ry: 0.054 }),
    back(0.56, { rx: 0.071, ry: 0.064 }),
    back(0.8, { rx: 0.069, ry: 0.063 }),
  ]), SKIN, undefined, { warm: 0.35 }));
  g.add(part(limb([
    back(0.3, { rx: 0.076, ry: 0.069 }),
    back(0.38, { rx: 0.083, ry: 0.076 }),
    back(0.7, { rx: 0.088, ry: 0.082 }),
    back(1.1, { rx: 0.086, ry: 0.081 }),
    back(1.6, { rx: 0.083, ry: 0.079 }), // far enough back that its end is never in frame
  ]), SLEEVE));
  const at = back(0.31, {});
  const cuff = part(new THREE.TorusGeometry(0.08, 0.017, 8, 18), SLEEVE_DARK);
  cuff.position.set(at.x, at.y, at.z);
  cuff.rotation.set(0.95, mirror * -0.35, 0);
  g.add(cuff);
  return g;
}

/**
 * A first-person hand: a palm with the pads at its thumb and its edge, four fingers of
 * three bones, and an opposed thumb of two. The joint groups carry their rest angles,
 * so a gesture can close the hand around whatever it holds.
 */
function hand(mirror = 1) {
  const g = new THREE.Group();

  // The palm: wide across the knuckles, narrow and thin at the wrist, with the ball
  // of the thumb on one side and the heel of the hand on the other.
  g.add(part(limb([
    { z: -0.055, rx: 0.040, ry: 0.030 },
    { z: -0.01, rx: 0.048, ry: 0.027 },
    { z: 0.03, rx: 0.052, ry: 0.024 },
    { z: 0.062, rx: 0.050, ry: 0.022 },
  ]), SKIN, undefined, { warm: 0.6 }));
  g.add(part(pad(0.022, 0.02, 0.042, mirror * 0.032, -0.004, 0.0), SKIN, undefined, { warm: 0.5 }));
  g.add(part(pad(0.016, 0.017, 0.038, -mirror * 0.036, -0.004, 0.005), SKIN_DARK));
  // The knuckles, a ridge of four across the front of the palm.
  for (let i = 0; i < 4; i++) {
    g.add(part(pad(0.0125, 0.011, 0.014, (i - 1.5) * 0.0235 * mirror, 0.008, 0.058), SKIN, undefined, { warm: 1 }));
  }

  const fingers = new THREE.Group();
  fingers.position.set(0, 0.002, 0.058);
  for (let i = 0; i < 4; i++) {
    // Index to little: shorter, thinner, and set a little lower along the knuckle arc.
    const len = 0.082 - Math.abs(i - 1.3) * 0.006 - (i === 3 ? 0.008 : 0);
    const f = finger(len, 0.0115 - i * 0.0006, [-1.2 - i * 0.05, 1.05, 0.7]);
    f.position.set((i - 1.5) * 0.0235 * mirror, -Math.abs(i - 1.35) * 0.0035, 0);
    f.rotation.y = mirror * (i - 1.5) * 0.045; // they fan out a little
    fingers.add(f);
  }
  fingers.name = 'fingers';
  g.add(fingers);

  // The thumb sits lower and across, opposed to the fingers, and has two bones.
  const thumb = new THREE.Group();
  thumb.position.set(mirror * 0.042, -0.004, 0.005);
  thumb.rotation.set(-0.5, mirror * -0.35, mirror * 0.95);
  const base = new THREE.Group();
  base.rotation.x = -0.35;
  Object.assign(base.userData, { rest: -0.35, joint: 0, give: 0.55 });
  base.add(part(limb([
    { z: -0.008, rx: 0.016, ry: 0.017 },
    { z: 0.022, rx: 0.0155, ry: 0.0165 },
    { z: 0.046, rx: 0.0145, ry: 0.0155 },
  ], 12), SKIN, undefined, { warm: 0.8 }));
  const tip = new THREE.Group();
  tip.position.z = 0.046;
  tip.rotation.x = 0.62;
  Object.assign(tip.userData, { rest: 0.62, joint: 1, give: 0.7 });
  tip.add(part(limb([
    { z: -0.006, rx: 0.0145, ry: 0.0152 },
    { z: 0.018, rx: 0.0135, ry: 0.014 },
    { z: 0.034, rx: 0.0115, ry: 0.012 },
  ], 12), SKIN, undefined, { warm: 1 }));
  tip.add(part(new THREE.SphereGeometry(0.0115, 12, 8).scale(1, 1.05, 1.15).translate(0, 0, 0.034), SKIN,
    undefined, { warm: 1 }));
  tip.add(part(pad(0.008, 0.004, 0.015, 0, 0.009, 0.022), '#e6c3ad'));
  base.add(tip);
  thumb.add(base);
  thumb.name = 'thumb';
  g.add(thumb);
  return g;
}

// The hole through a closed fist, in the hand's own coordinates: between the palm and
// the curled fingertips. A tool's shaft has to pass through exactly here, or the hand
// is holding air next to it.
const FIST = new THREE.Vector3(0, 0.002, 0.05);

/**
 * Puts a hand on a tool, gripping it. Every tool that is held by a shaft builds it
 * along its own +y, so the hand is turned a quarter turn about z - which points its
 * fingers' curl axis along the shaft - and slid until the fist's hole is at `y` on it.
 * The hand becomes part of the tool, so the two move together however it is swung.
 */
function gripHand(tool, y, mirror = 1) {
  const h = hand(mirror);
  h.rotation.set(0, 0, mirror * Math.PI / 2);
  const hole = FIST.clone().applyEuler(h.rotation);
  h.position.set(-hole.x, y - hole.y, -hole.z);
  tool.add(h);
  attachArm(h, mirror, tool);
  return h;
}

/**
 * Hangs an arm off a hand's wrist. The arm follows the hand, because it is the same
 * arm - but it is counter-rotated by however the hand is turned, so that at rest it
 * falls away towards the corner of the frame whatever angle the tool is held at. A
 * launcher held across the view must not take the shoulder with it.
 */
function attachArm(h, mirror, tool) {
  const a = arm(mirror);
  a.position.copy(WRIST);
  h.updateMatrix();
  const m = h.matrix.clone();
  if (tool) {
    tool.updateMatrix();
    m.premultiply(tool.matrix);
  }
  a.quaternion.setFromRotationMatrix(m).invert();
  h.add(a);
  return a;
}

// How far each joint gives when the hand closes: the knuckle least, the middle most.
// A hand that bent every joint equally would close like a claw.
const JOINT_GIVE = [0.22, 0.4, 0.3];

/** Curls every joint of every finger towards a fist: 0 at rest, 1 gripping hard. */
function grip(vm, amount) {
  vm.traverse(j => {
    const { rest, joint, give = 1 } = j.userData;
    if (rest === undefined) return;
    j.rotation.x = rest - amount * JOINT_GIVE[joint] * give;
  });
}

// ------------------------------------------------------------------ the viewmodel

const REST = { x: 0.235, y: -0.2, z: -0.55, rx: 0.1, ry: -0.25, rz: 0 };

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
      // The rod, on a pivot where the hand grips it, so the two swing together.
      const rod = new THREE.Group();
      rod.position.set(0, 0.02, 0.03);
      rod.rotation.set(-1.2, 0, 0.22);
      rod.name = 'rod';
      g.add(rod);
      gripHand(rod, -0.02);

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
      const net = new THREE.Group();
      net.position.set(0, 0.02, 0.03);
      net.rotation.set(-1.28, 0, 0.18);
      net.name = 'net';
      g.add(net);
      gripHand(net, -0.03);

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
      attachArm(right, 1);
      const left = hand(-1);
      left.position.set(-0.15, 0.03, -0.31);
      left.rotation.set(0.3, 1.0, 0.5);
      left.name = 'left';
      g.add(left);
      attachArm(left, -1);

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
      const wand = new THREE.Group();
      wand.position.set(0, 0.02, 0.03);
      wand.rotation.set(-1.28, 0, 0.15);
      wand.name = 'wand';
      g.add(wand);
      gripHand(wand, -0.03);

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
      const gun = new THREE.Group();
      // Held across the view rather than pointed down it: a launcher seen end-on is a
      // dark blob, and half the point of it is that it looks like something.
      gun.position.set(-0.03, 0.035, 0.04);
      gun.rotation.set(-0.06, 0.62, 0.07);
      gun.name = 'gun';
      g.add(gun);
      gripHand(gun, -0.045); // the fist is round the pistol grip, under the receiver

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
