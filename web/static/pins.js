// Where the findings are, seen from above.
//
// In walk mode a finding is a beetle on the street, which you have to be in the
// street to find. The isometric view has no streets to walk, so the same findings are
// pins: one over every building that carries any, in the color of the worst of them,
// standing taller the more there are. From across the map the red ones are where to
// go next; up close the panel says what they are.
//
// A finding lands on exactly one pin, the same way it lands on exactly one bug: the
// nearest building drawn for the node it belongs to. A directory that is collapsed
// carries what is below it, and one that is expanded does not carry it twice.
//
// A pin is a marker, so it is drawn at a size on the screen rather than a size on the
// map: the isometric camera is orthographic, and a world unit is exactly its zoom in
// pixels, so dividing by the zoom holds a pin the same size however far out the map
// is pulled. follow is what keeps up with that.

import * as THREE from './vendor/three.module.min.js';
import { boxFor } from './bugs.js';
import { rankOf, severityColors, worse } from './findings.js';

const PIN_PX = 30;    // how tall a pin tries to be on the screen
const HEAD = 0.26;    // the diamond on top, in pin units
const STEM = 0.55;    // the stalk, before the count stretches it
const GROWTH = 0.22;  // how much taller each doubling of the count makes it
const MIN = 0.05;     // never smaller than this in map units
const MAX = 4;        // nor bigger, however far the map is pulled away

let parts = null;

export class Pins {
  constructor(scene) {
    this.scene = scene;
    this.group = new THREE.Group();
    this.group.visible = false;
    this.materials = new Map();
    this.meshes = [];  // [{ stems, heads, items }]
    this.zoom = 0;     // the zoom the current matrices were composed for
    scene.scene.add(this.group);
  }

  show(on) {
    this.group.visible = on;
    if (on) this.follow();
    this.scene.requestRender();
  }

  /** One pin per building that carries findings. index is what findings.js built. */
  place(index, boxes) {
    this.clear();
    if (!index?.all.length || !boxes?.length) return;
    const colors = severityColors();
    parts ||= geometry();

    const byNode = new Map();
    for (const b of boxes) {
      if (b.kind !== 'land' && b.node) byNode.set(b.node.id, b);
    }
    // Tally onto the box each finding is drawn at: how many, and the worst of them.
    const tally = new Map();
    for (const f of index.all) {
      const box = boxFor(byNode, index.place(f));
      if (!box) continue;
      const at = tally.get(box);
      if (at) {
        at.count++;
        at.worst = worse(at.worst, f.severity);
      } else {
        tally.set(box, { count: 1, worst: f.severity });
      }
    }

    // Grouped by severity, because a color is a material and a material is a draw.
    const bySeverity = new Map();
    for (const [box, at] of tally) {
      const sev = at.worst || 'unknown';
      if (!bySeverity.has(sev)) bySeverity.set(sev, []);
      bySeverity.get(sev).push({ box, count: at.count });
    }
    // Worst last, so a critical pin draws over the low one behind it.
    for (const sev of [...bySeverity.keys()].sort((a, b) => rankOf(a) - rankOf(b))) {
      const items = bySeverity.get(sev);
      const material = this.material(colors[sev] || colors.unknown);
      const stems = new THREE.InstancedMesh(parts.stem, material, items.length);
      const heads = new THREE.InstancedMesh(parts.head, material, items.length);
      for (const mesh of [stems, heads]) {
        mesh.frustumCulled = false;
        this.group.add(mesh);
      }
      this.meshes.push({ stems, heads, items });
    }
    this.zoom = 0;
    this.follow();
  }

  /**
   * Re-sizes the pins for the camera they are seen with, if it has moved enough to be
   * worth it. Called after every render, so it has to be cheap when nothing changed.
   */
  follow() {
    const zoom = this.scene.camera.zoom;
    if (!this.meshes.length || !this.group.visible) return;
    if (Math.abs(zoom - this.zoom) < this.zoom * 0.04) return;
    this.zoom = zoom;
    const m = new THREE.Matrix4(), q = new THREE.Quaternion(), p = new THREE.Vector3(), s = new THREE.Vector3();
    // A marker is a size on the screen, not on the map - that is what makes it a
    // marker - but it stops growing eventually, or a map pulled right out is nothing
    // but pins.
    const want = Math.min(MAX, Math.max(MIN, PIN_PX / zoom));
    for (const { stems, heads, items } of this.meshes) {
      items.forEach((it, i) => {
        const b = it.box;
        const k = want;
        const tall = k * (STEM + GROWTH * Math.log2(1 + it.count));
        const top = b.y + b.h;
        stems.setMatrixAt(i, m.compose(p.set(b.x, top, b.z), q, s.set(k, tall, k)));
        heads.setMatrixAt(i, m.compose(p.set(b.x, top + tall + k * HEAD, b.z), q, s.setScalar(k)));
      });
      stems.instanceMatrix.needsUpdate = true;
      heads.instanceMatrix.needsUpdate = true;
    }
    this.scene.requestRender();
  }

  // One material per color: six colors for however many pins there are.
  material(color) {
    let m = this.materials.get(color);
    if (!m) {
      m = this.scene.bendable(new THREE.MeshBasicMaterial({ color }));
      this.materials.set(color, m);
    }
    return m;
  }

  clear() {
    for (const mesh of this.group.children) mesh.dispose();
    this.group.clear();
    for (const m of this.materials.values()) m.dispose();
    this.materials.clear();
    this.meshes = [];
  }

  dispose() {
    this.clear();
    this.scene.scene.remove(this.group);
  }
}

// A pin, a unit across: a thin stalk off the roof with a diamond on the end of it.
// The stalk stands on the origin so an instance can stretch it to the height it needs.
function geometry() {
  return {
    stem: new THREE.CylinderGeometry(0.045, 0.045, 1, 5).translate(0, 0.5, 0),
    head: new THREE.OctahedronGeometry(HEAD).scale(0.9, 1.3, 0.9),
  };
}
