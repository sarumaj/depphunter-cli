// Where the walker is standing, shown on the map.
//
// Walk mode and the map are one place seen two ways, and until now only one of them
// said where you were: leaving the street put you back above a city with no sign of
// the person who had been in it. This is that sign - a figure at the walker's feet,
// facing the way they were facing, on the block they were on.
//
// It is a marker rather than a model, so it is drawn at a size on the screen and not
// a size on the map (the same arithmetic as pins.js: an orthographic camera's zoom is
// pixels per world unit, so dividing by it holds a marker still while the map is
// pulled about). And it is drawn twice: once solid, and once faintly with the depth
// test off, so a walker standing in a street between two towers is still visible
// through the one in front. A marker that says where you are has to be seen from
// wherever you are looking.

import * as THREE from './vendor/three.module.min.js';
import { mergeGeometries } from './vendor/BufferGeometryUtils.js';

const PX = 34;     // how tall the figure tries to be on the screen
const MIN = 0.06;  // never smaller than this in map units
const MAX = 5;     // nor bigger, however far the map is pulled away
const GHOST = 0.3; // how strongly it shows through what stands in front of it

let parts = null;

export class Avatar {
  constructor(scene) {
    this.scene = scene;
    this.group = new THREE.Group();
    this.group.visible = false;
    this.stance = null; // {x, z, feet, yaw}
    this.zoom = 0;      // the zoom the current scale was worked out for
    this.materials = [];
    scene.scene.add(this.group);
  }

  /**
   * Where the walker stands: {x, z, feet, yaw} in layout coordinates, or null when
   * they have never been out there.
   */
  set(stance, color) {
    this.stance = stance;
    if (!stance) {
      this.group.visible = false;
      this.scene.requestRender();
      return;
    }
    if (!this.group.children.length) this.build(color);
    else if (color && this.color !== color) this.paint(color);
    this.group.position.set(stance.x, stance.feet, stance.z);
    this.group.rotation.y = stance.yaw;
    this.zoom = 0; // the figure is sized for the camera, which may have moved
    this.follow();
  }

  show(on) {
    this.group.visible = on && !!this.stance;
    if (this.group.visible) this.follow();
    this.scene.requestRender();
  }

  /** Re-sizes for the camera, if it has moved enough to be worth it. */
  follow() {
    if (!this.group.visible || !this.group.children.length) return;
    const zoom = this.scene.camera.zoom;
    if (Math.abs(zoom - this.zoom) < this.zoom * 0.04) return;
    this.zoom = zoom;
    this.group.scale.setScalar(Math.min(MAX, Math.max(MIN, PX / zoom)));
    this.scene.requestRender();
  }

  // A figure a unit tall, facing -z, and the same figure again behind whatever hides
  // it. Two meshes, not two scenes: the faint one is drawn first and the solid one
  // over it, so where nothing is in the way only the solid one is seen.
  build(color) {
    parts ||= geometry();
    this.color = color;
    for (const ghost of [true, false]) {
      const material = this.scene.bendable(new THREE.MeshBasicMaterial({
        color, transparent: true, opacity: ghost ? GHOST : 1,
        depthTest: !ghost, depthWrite: false,
      }));
      const mesh = new THREE.Mesh(parts, material);
      mesh.frustumCulled = false;
      mesh.renderOrder = ghost ? 8 : 9;
      this.materials.push(material);
      this.group.add(mesh);
    }
  }

  paint(color) {
    this.color = color;
    for (const m of this.materials) m.color.set(color);
  }

  clear() {
    for (const m of this.materials) m.dispose();
    this.materials = [];
    this.group.clear();
  }

  dispose() {
    this.clear();
    this.scene.scene.remove(this.group);
  }
}

// The figure: a tapered body with a head on it, standing on a ring, and an arrow on
// the ground in front saying which way it is looking. One unit tall, nose along -z,
// its feet at the origin - which is where the walker's are.
function geometry() {
  const body = new THREE.CylinderGeometry(0.17, 0.3, 0.62, 10).translate(0, 0.31, 0);
  const head = new THREE.SphereGeometry(0.22, 12, 8).translate(0, 0.78, 0);
  const ring = new THREE.TorusGeometry(0.42, 0.055, 6, 18).rotateX(Math.PI / 2).translate(0, 0.02, 0);
  // The arrow: a flat head just clear of the ring, so the ring reads as the spot and
  // the arrow as the heading rather than the two running together.
  const arrow = new THREE.ConeGeometry(0.2, 0.38, 3)
    .rotateX(-Math.PI / 2).rotateY(Math.PI) // point along -z, lying flat
    .translate(0, 0.03, -0.72);
  // One buffer: they share a material and never move apart, so there is nothing to
  // be had from keeping them as four draws.
  return mergeGeometries([body, head, ring, arrow]);
}
