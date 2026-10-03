// Where the walker is standing, shown on the map: a figure at their feet, facing the
// way they were facing, on the block they were on, so that leaving the street does
// not put you back above a city with no sign of who was in it.
//
// It is a marker rather than a model, drawn at a size on the screen and not a size on
// the map (the same arithmetic as pins.js: an orthographic camera's zoom is pixels
// per world unit, so dividing by it holds a marker still while the map is pulled
// about). It is drawn twice - once solid, once faintly with the depth test off - so a
// walker in a street between two towers is still visible through the one in front.
//
// And it pulses, so it is found at a glance on a busy map: bright rings spread over the
// ground from its feet and fade, one after another, seen through whatever
// stands in front of it. A pulse is a redraw, so the map keeps drawing (setAnimated)
// only while the figure is shown; with reduced motion the ring stands still.
//
// Implements: REQ-MAP-065

import * as THREE from '../vendor/three.module.min.js';
import { mergeGeometries } from '../vendor/BufferGeometryUtils.js';
import { reducedMotion } from './walkbase.js';

const PX = 34;     // how tall the figure tries to be on the screen
const MIN = 0.06;  // never smaller than this in map units
const MAX = 5;     // nor bigger, however far the map is pulled away
const GHOST = 0.3; // how strongly it shows through what stands in front of it
const PULSE = 1;   // seconds a ring takes to spread from the feet and fade
const BEACON = '#2ec5ff'; // its color: bright on grass, asphalt and the dark of space alike, which ink is not
const SPREAD = [0.45, 2.2]; // from and to what radius, as the figure is tall

let parts = null;
let canopyParts = null;

export class Avatar {
  constructor(scene) {
    this.scene = scene;
    this.group = new THREE.Group();
    this.group.visible = false;
    this.pulse = [];    // the rings spreading from the feet
    this.stance = null; // {x, z, feet, yaw, canopy}
    this.canopy = [];   // the canopy over the figure, for a walker who left under one
    this.zoom = 0;      // the zoom the current scale was worked out for
    this.materials = [];
    this.beacons = [];  // the pulse's, which keep their own color
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
      this.scene.setAnimated(false, 'avatar');
      this.scene.requestRender();
      return;
    }
    if (!this.group.children.length) this.build(color);
    else if (color && this.color !== color) this.paint(color);
    for (const mesh of this.canopy) mesh.visible = !!stance.canopy;
    this.group.position.set(stance.x, stance.feet, stance.z);
    this.group.rotation.y = stance.yaw;
    this.zoom = 0; // the figure is sized for the camera, which may have moved
    this.follow();
  }

  show(on) {
    this.group.visible = on && !!this.stance;
    if (this.group.visible) this.follow();
    this.scene.setAnimated(this.group.visible && !reducedMotion(), 'avatar');
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
  //
  // A walker who left the street hanging under a parachute is drawn under one: the
  // canopy is a second pair of meshes over the figure, in the same two materials, shown
  // only while the stance says so.
  // Implements: REQ-TOOL-079
  build(color) {
    parts ||= geometry();
    canopyParts ||= canopyGeometry();
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
      const canopy = new THREE.Mesh(canopyParts, material);
      canopy.frustumCulled = false;
      canopy.renderOrder = mesh.renderOrder;
      canopy.visible = !!this.stance?.canopy;
      this.canopy.push(canopy);
      this.group.add(canopy);
    }
    // The pulse: two rings half a beat apart, posed as they are drawn, from the clock,
    // so they need nothing but redraws.
    this.pulse = [0, 0.5].map(phase => {
      const material = this.scene.bendable(new THREE.MeshBasicMaterial({
        color: BEACON, transparent: true, depthTest: false, depthWrite: false, side: THREE.DoubleSide,
      }));
      this.beacons.push(material);
      const ring = new THREE.Mesh(new THREE.RingGeometry(0.7, 1, 40).rotateX(-Math.PI / 2).translate(0, 0.02, 0), material);
      ring.frustumCulled = false;
      ring.renderOrder = 7;
      ring.onBeforeRender = () => {
        const t = reducedMotion() ? 0.35 + phase * 0.3 : (performance.now() / 1000 / PULSE + phase) % 1;
        ring.scale.setScalar(SPREAD[0] + (SPREAD[1] - SPREAD[0]) * t);
        material.opacity = (1 - t) ** 1.2;
        ring.updateMatrixWorld();
      };
      this.group.add(ring);
      return ring;
    });
  }

  paint(color) {
    this.color = color;
    for (const m of this.materials) m.color.set(color);
  }

  clear() {
    for (const m of [...this.materials, ...this.beacons]) m.dispose();
    this.materials = [];
    this.beacons = [];
    this.canopy = [];
    this.pulse = [];
    this.group.clear();
  }

  dispose() {
    this.scene.setAnimated(false, 'avatar');
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

// The canopy, at the figure's scale: an arched wing over its head, as wide as the
// figure is tall and a little over, with a line down from each tip to the shoulders.
function canopyGeometry() {
  const arc = 1.1, radius = 1.4;
  const wing = new THREE.TorusGeometry(radius, 0.09, 6, 18, arc)
    .rotateZ(Math.PI / 2 - arc / 2) // the arch centered over the head
    .scale(1, 1, 4)                 // its tube drawn out into a chord
    .translate(0, 2.35 - radius, 0);
  const lines = [-1, 1].map(side => {
    const tip = new THREE.Vector3(side * radius * Math.sin(arc / 2), 2.35 - radius + radius * Math.cos(arc / 2) - 0.05, 0);
    const shoulder = new THREE.Vector3(side * 0.16, 0.6, 0);
    const run = new THREE.Vector3().subVectors(tip, shoulder);
    const line = new THREE.CylinderGeometry(0.025, 0.025, run.length(), 5);
    line.applyQuaternion(new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 1, 0), run.clone().normalize()));
    return line.translate((tip.x + shoulder.x) / 2, (tip.y + shoulder.y) / 2, 0);
  });
  return mergeGeometries([wing, ...lines]);
}
