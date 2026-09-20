// What walk mode puts in your hands. A tool is a viewmodel - a hand and the thing it
// holds, drawn in front of the camera - an animation for using it, and whatever
// travels to the target. Nothing here is lit: the map has no lights, so shape comes
// from choosing a darker color for what would be in shadow, the way the city's own
// geometry does. Tagging a module is the same act throughout; only the
// gesture changes, so none of this touches the map or the graph.
//
// Each tool provides:
//   viewmodel(scene)  the group parented to the walk camera, posed at rest
//   pose(vm, u, now)  u < 0 idle (breathing), 0..1 through the swing
//   projectile(scene) what flies, or null when the tool acts at once (the camera)
//   reticle           the aim helper's style, so it fits the tool rather than an
//                     ever-present crosshair
//   verb / noun       what the HUD and its messages call the act and its tally

import * as THREE from './vendor/three.module.min.js';

const SKIN = '#c98d5e', SKIN_DARK = '#a9724a';

// hand builds a simple first-person hand: a palm, a thumb and three fingers, curled
// as if holding something. Anything it holds is added at the grip.
function hand(mirror = 1) {
  const g = new THREE.Group();
  const mat = new THREE.MeshBasicMaterial({ color: SKIN });
  const dark = new THREE.MeshBasicMaterial({ color: SKIN_DARK });
  const palm = new THREE.Mesh(new THREE.BoxGeometry(0.09, 0.05, 0.12), mat);
  g.add(palm);
  for (let i = 0; i < 3; i++) { // fingers, curled over the grip
    const f = new THREE.Mesh(new THREE.BoxGeometry(0.022, 0.05, 0.05), dark);
    f.position.set((i - 1) * 0.026, 0.03, 0.06);
    f.rotation.x = -0.9;
    g.add(f);
  }
  const thumb = new THREE.Mesh(new THREE.BoxGeometry(0.025, 0.045, 0.05), dark);
  thumb.position.set(mirror * 0.05, 0.015, 0.03);
  thumb.rotation.z = mirror * 0.7;
  g.add(thumb);
  // A cuff, so the arm does not end in nothing.
  const cuff = new THREE.Mesh(new THREE.CylinderGeometry(0.055, 0.06, 0.1, 8), new THREE.MeshBasicMaterial({ color: '#3f6d8f' }));
  cuff.rotation.x = Math.PI / 2;
  cuff.position.z = -0.1;
  g.add(cuff);
  return g;
}

// viewmodel places a hand and its tool where a first-person view expects them: low
// and to the right, angled towards the middle of the screen.
function viewmodel(build, { x = 0.26, y = -0.24, z = -0.55 } = {}) {
  const g = new THREE.Group();
  g.position.set(x, y, z);
  g.rotation.set(0.1, -0.25, 0);
  build(g);
  for (const m of g.children) m.frustumCulled = false;
  g.traverse(o => { o.frustumCulled = false; o.renderOrder = 10; });
  return g;
}

// idle gives the held tool a breath: a slow bob, so the view is never quite dead.
function idle(vm, now) {
  vm.position.y = vm.userData.restY + Math.sin(now / 900) * 0.006;
  vm.rotation.z = Math.sin(now / 1300) * 0.02;
}

// swing eases a gesture: quick out, slower back, which is how a throw feels.
const ease = u => (u < 0.35 ? u / 0.35 : 1 - (u - 0.35) / 0.65);

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
      const pole = new THREE.Mesh(
        new THREE.CylinderGeometry(0.006, 0.014, 0.9, 6),
        new THREE.MeshBasicMaterial({ color: '#8a5a2b' }));
      pole.rotation.set(-1.15, 0, 0.25);
      pole.position.set(-0.02, 0.16, -0.18);
      pole.name = 'pole';
      g.add(pole);
      const reel = new THREE.Mesh(new THREE.CylinderGeometry(0.035, 0.035, 0.03, 10),
        new THREE.MeshBasicMaterial({ color: '#4a5560' }));
      reel.rotation.z = Math.PI / 2;
      reel.position.set(-0.05, 0.03, -0.06);
      g.add(reel);
    });
  },
  // The rod loads backwards, then snaps forward: the cast is in the wrist.
  pose(vm, u) {
    const pole = vm.getObjectByName('pole');
    const k = u < 0.3 ? -u / 0.3 : (u - 0.3) / 0.7; // back, then through
    pole.rotation.x = -1.15 + k * 0.9;
    vm.rotation.x = 0.1 - k * 0.25;
    vm.position.z = -0.55 + Math.max(0, k) * 0.06;
  },
  projectile() {
    const g = new THREE.Group();
    const float = new THREE.Mesh(new THREE.SphereGeometry(0.05, 10, 8),
      new THREE.MeshBasicMaterial({ color: '#e4402f' }));
    const cap = new THREE.Mesh(new THREE.SphereGeometry(0.051, 10, 8, 0, Math.PI * 2, 0, Math.PI / 2),
      new THREE.MeshBasicMaterial({ color: '#f2f4f7' }));
    g.add(float, cap);
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
      const handle = new THREE.Mesh(new THREE.CylinderGeometry(0.012, 0.016, 0.55, 6),
        new THREE.MeshBasicMaterial({ color: '#c8a26a' }));
      handle.rotation.set(-1.25, 0, 0.2);
      handle.position.set(-0.01, 0.1, -0.12);
      handle.name = 'handle';
      g.add(handle);
      const hoop = new THREE.Mesh(new THREE.TorusGeometry(0.13, 0.008, 6, 20),
        new THREE.MeshBasicMaterial({ color: '#d8dee6' }));
      hoop.position.set(0.02, 0.38, -0.36);
      hoop.rotation.x = 0.4;
      hoop.name = 'hoop';
      g.add(hoop);
      const bag = new THREE.Mesh(new THREE.ConeGeometry(0.125, 0.22, 14, 1, true),
        new THREE.MeshBasicMaterial({ color: '#eef3f8', transparent: true, opacity: 0.45, side: THREE.DoubleSide }));
      bag.position.copy(hoop.position);
      bag.position.y -= 0.1;
      bag.rotation.x = Math.PI + 0.4;
      bag.name = 'bag';
      g.add(bag);
    });
  },
  // A sweep across the view, the way a net is actually swung.
  pose(vm, u) {
    const k = ease(u);
    vm.rotation.z = -k * 1.1;
    vm.rotation.y = -0.25 + k * 0.5;
    vm.position.x = 0.26 - k * 0.22;
  },
  projectile() {
    const g = new THREE.Group();
    const hoop = new THREE.Mesh(new THREE.TorusGeometry(0.16, 0.012, 6, 18),
      new THREE.MeshBasicMaterial({ color: '#d8dee6' }));
    const bag = new THREE.Mesh(new THREE.ConeGeometry(0.15, 0.26, 12, 1, true),
      new THREE.MeshBasicMaterial({ color: '#eef3f8', transparent: true, opacity: 0.4, side: THREE.DoubleSide }));
    bag.position.z = -0.13;
    bag.rotation.x = Math.PI / 2;
    g.add(hoop, bag);
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
      g.add(hand());
      const body = new THREE.Mesh(new THREE.BoxGeometry(0.26, 0.17, 0.12),
        new THREE.MeshBasicMaterial({ color: '#2c3138' }));
      body.position.set(-0.04, 0.09, -0.16);
      body.name = 'body';
      g.add(body);
      const lens = new THREE.Mesh(new THREE.CylinderGeometry(0.06, 0.07, 0.1, 14),
        new THREE.MeshBasicMaterial({ color: '#1a1d21' }));
      lens.rotation.x = Math.PI / 2;
      lens.position.set(-0.04, 0.09, -0.24);
      g.add(lens);
      const glass = new THREE.Mesh(new THREE.CircleGeometry(0.05, 14),
        new THREE.MeshBasicMaterial({ color: '#7fb8ff' }));
      glass.position.set(-0.04, 0.09, -0.29);
      g.add(glass);
      const button = new THREE.Mesh(new THREE.CylinderGeometry(0.018, 0.018, 0.02, 8),
        new THREE.MeshBasicMaterial({ color: '#e4402f' }));
      button.position.set(0.04, 0.185, -0.15);
      button.name = 'button';
      g.add(button);
    });
  },
  // The shutter is pressed and the camera kicks back: no throw, no travel.
  pose(vm, u) {
    const k = ease(u);
    const button = vm.getObjectByName('button');
    button.position.y = 0.185 - k * 0.012;
    vm.position.z = -0.55 - k * 0.03;
    vm.rotation.x = 0.1 + k * 0.12;
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
      const stick = new THREE.Mesh(new THREE.CylinderGeometry(0.008, 0.01, 0.3, 6),
        new THREE.MeshBasicMaterial({ color: '#59b0e6' }));
      stick.rotation.set(-1.3, 0, 0.15);
      stick.position.set(-0.01, 0.08, -0.14);
      g.add(stick);
      const ring = new THREE.Mesh(new THREE.TorusGeometry(0.06, 0.008, 6, 16),
        new THREE.MeshBasicMaterial({ color: '#59b0e6' }));
      ring.position.set(0.01, 0.22, -0.26);
      ring.rotation.x = 0.3;
      ring.name = 'ring';
      g.add(ring);
    });
  },
  // A gentle wave, as if blowing through the ring.
  pose(vm, u) {
    const k = ease(u);
    vm.rotation.y = -0.25 + Math.sin(u * Math.PI * 2) * 0.18;
    vm.position.y = vm.userData.restY + k * 0.04;
    vm.getObjectByName('ring').scale.setScalar(1 + k * 0.25);
  },
  projectile() {
    const g = new THREE.Group();
    const skin = new THREE.Mesh(new THREE.SphereGeometry(0.11, 14, 10),
      new THREE.MeshBasicMaterial({ color: '#bfe6ff', transparent: true, opacity: 0.45 }));
    const sheen = new THREE.Mesh(new THREE.SphereGeometry(0.04, 8, 6),
      new THREE.MeshBasicMaterial({ color: '#ffffff', transparent: true, opacity: 0.6 }));
    sheen.position.set(0.04, 0.04, 0.04);
    g.add(skin, sheen);
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
      const barrel = new THREE.Mesh(new THREE.CylinderGeometry(0.03, 0.035, 0.42, 10),
        new THREE.MeshBasicMaterial({ color: '#3a4049' }));
      barrel.rotation.x = Math.PI / 2;
      barrel.position.set(-0.02, 0.08, -0.26);
      barrel.name = 'barrel';
      g.add(barrel);
      const scope = new THREE.Mesh(new THREE.CylinderGeometry(0.018, 0.018, 0.16, 8),
        new THREE.MeshBasicMaterial({ color: '#20242a' }));
      scope.rotation.x = Math.PI / 2;
      scope.position.set(-0.02, 0.13, -0.24);
      g.add(scope);
    });
  },
  pose(vm, u) {
    const k = ease(u);
    vm.position.z = -0.55 - k * 0.05; // recoil straight back
    vm.rotation.x = 0.1 + k * 0.18;
  },
  projectile() {
    const g = new THREE.Group();
    const mat = new THREE.MeshBasicMaterial({ color: '#2b2d31' });
    const body = new THREE.Mesh(new THREE.CylinderGeometry(0.008, 0.008, 0.26, 6).rotateX(Math.PI / 2), mat);
    const tip = new THREE.Mesh(new THREE.ConeGeometry(0.018, 0.06, 8).rotateX(Math.PI / 2).translate(0, 0, 0.16),
      new THREE.MeshBasicMaterial({ color: '#ff8a1f' }));
    const finA = new THREE.Mesh(new THREE.BoxGeometry(0.07, 0.003, 0.06).translate(0, 0, -0.11),
      new THREE.MeshBasicMaterial({ color: '#d8dadf' }));
    const finB = new THREE.Mesh(new THREE.BoxGeometry(0.003, 0.07, 0.06).translate(0, 0, -0.11),
      new THREE.MeshBasicMaterial({ color: '#d8dadf' }));
    g.add(body, tip, finA, finB);
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
