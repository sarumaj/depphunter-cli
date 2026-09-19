// three.js rendering of a layout: one InstancedMesh for every box, tubes for edges.
// Renders on demand only; nothing animates unless the camera or the state changes.

import * as THREE from './vendor/three.module.min.js';
import { OrbitControls } from './vendor/OrbitControls.js';

const ISO_POLAR = Math.acos(1 / Math.sqrt(3)); // true isometric elevation (35.26°)

export class MapScene {
  constructor(container) {
    this.container = container;
    this.renderer = new THREE.WebGLRenderer({ antialias: true });
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    container.appendChild(this.renderer.domElement);

    this.scene = new THREE.Scene();
    this.camera = new THREE.OrthographicCamera(-1, 1, 1, -1, -2000, 4000);
    this.camera.zoom = 20;


    this.controls = new OrbitControls(this.camera, this.renderer.domElement);
    this.controls.enableDamping = false;
    this.controls.screenSpacePanning = true;
    this.controls.mouseButtons = { LEFT: THREE.MOUSE.PAN, MIDDLE: THREE.MOUSE.DOLLY, RIGHT: THREE.MOUSE.ROTATE };
    this.controls.touches = { ONE: THREE.TOUCH.PAN, TWO: THREE.TOUCH.DOLLY_ROTATE };
    this.controls.minPolarAngle = 0.15;
    this.controls.maxPolarAngle = 1.35;
    this.controls.minZoom = 0.5;
    this.controls.maxZoom = 400;
    this.controls.zoomToCursor = true;
    this.controls.addEventListener('change', () => this.requestRender());

    this.unitBox = shadedBox();
    // Unlit: the top face shows exactly the encoded colour; fixed per-face shading
    // (vertex colours, multiplied with the instance colour) gives the 3D form.
    this.material = new THREE.MeshBasicMaterial({ vertexColors: true });
    this.mesh = null;
    this.edgeGroup = new THREE.Group();
    this.scene.add(this.edgeGroup);
    this.outline = null;
    this.raycaster = new THREE.Raycaster();
    this.onRender = null;

    this.setIso(0);
    new ResizeObserver(() => this.resize()).observe(container);
    this.resize();
  }

  resize() {
    const { clientWidth: w, clientHeight: h } = this.container;
    this.renderer.setSize(w, h, false);
    Object.assign(this.camera, { left: -w / 2, right: w / 2, top: h / 2, bottom: -h / 2 });
    this.camera.updateProjectionMatrix();
    this.requestRender();
  }

  requestRender() {
    if (this.pending) return;
    this.pending = true;
    requestAnimationFrame(() => {
      this.pending = false;
      this.renderer.render(this.scene, this.camera);
      this.onRender?.();
    });
  }

  setBackground(color) {
    this.scene.background = new THREE.Color(color);
    this.requestRender();
  }

  /** Replace all boxes. colors: array of CSS colours, one per box. */
  setBoxes(boxes, colors) {
    if (this.mesh) {
      this.scene.remove(this.mesh);
      this.mesh.dispose();
    }
    this.boxes = boxes;
    const mesh = new THREE.InstancedMesh(this.unitBox, this.material, boxes.length);
    const m = new THREE.Matrix4(), q = new THREE.Quaternion(), p = new THREE.Vector3(), s = new THREE.Vector3();
    boxes.forEach((b, i) => {
      m.compose(p.set(b.x, b.y, b.z), q, s.set(b.w, Math.max(b.h, 0.01), b.d));
      mesh.setMatrixAt(i, m);
    });
    mesh.computeBoundingSphere();
    this.mesh = mesh;
    this.setColors(colors);
    this.scene.add(mesh);
    this.requestRender();
  }

  setColors(colors) {
    const c = new THREE.Color();
    colors.forEach((col, i) => this.mesh.setColorAt(i, c.set(col)));
    if (this.mesh.instanceColor) this.mesh.instanceColor.needsUpdate = true;
    this.requestRender();
  }

  /** Box index under a client-space point, or -1. */
  pick(clientX, clientY) {
    if (!this.mesh) return -1;
    const r = this.renderer.domElement.getBoundingClientRect();
    const ndc = new THREE.Vector2(((clientX - r.left) / r.width) * 2 - 1, -((clientY - r.top) / r.height) * 2 + 1);
    this.raycaster.setFromCamera(ndc, this.camera);
    const hit = this.raycaster.intersectObject(this.mesh, false)[0];
    return hit ? hit.instanceId : -1;
  }

  setOutline(box, color) {
    if (this.outline) {
      this.scene.remove(this.outline);
      this.outline.geometry.dispose();
      this.outline = null;
    }
    if (box) {
      const geo = new THREE.EdgesGeometry(new THREE.BoxGeometry(box.w + 0.08, box.h + 0.08, box.d + 0.08));
      this.outline = new THREE.LineSegments(geo, new THREE.LineBasicMaterial({ color, depthTest: false, transparent: true }));
      this.outline.position.set(box.x, box.y + box.h / 2, box.z);
      this.outline.renderOrder = 10;
      this.scene.add(this.outline);
    }
    this.requestRender();
  }

  /** arcs: [{from: box, to: box, color, count}] — drawn as raised curves with an arrow head at the target. */
  setArcs(arcs) {
    for (const c of this.edgeGroup.children) c.geometry.dispose();
    this.edgeGroup.clear();
    const mats = new Map();
    const mat = color => {
      if (!mats.has(color)) mats.set(color, new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.9 }));
      return mats.get(color);
    };
    for (const a of arcs) {
      const p0 = new THREE.Vector3(a.from.x, a.from.y + a.from.h, a.from.z);
      const p2 = new THREE.Vector3(a.to.x, a.to.y + a.to.h, a.to.z);
      const dist = p0.distanceTo(p2);
      const mid = p0.clone().add(p2).multiplyScalar(0.5);
      mid.y = Math.max(p0.y, p2.y) + 1.5 + dist * 0.3;
      const curve = new THREE.QuadraticBezierCurve3(p0, mid, p2);
      const r = 0.05 + 0.035 * Math.log2(1 + a.count);
      this.edgeGroup.add(new THREE.Mesh(new THREE.TubeGeometry(curve, 32, r, 5, false), mat(a.color)));

      const head = new THREE.Mesh(new THREE.ConeGeometry(r * 3, r * 7, 8), mat(a.color));
      const tangent = curve.getTangent(0.97);
      head.position.copy(curve.getPoint(0.97));
      head.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), tangent);
      this.edgeGroup.add(head);
    }
    this.requestRender();
  }

  /** Orient the camera isometrically; quarter = number of 90° turns. */
  setIso(quarter) {
    this.quarter = ((quarter % 4) + 4) % 4;
    const az = Math.PI / 4 + this.quarter * Math.PI / 2;
    const t = this.controls.target;
    const polar = Math.PI / 2 - ISO_POLAR;
    const d = 1000;
    this.camera.position.set(
      t.x + d * Math.sin(polar) * Math.sin(az),
      t.y + d * Math.cos(polar),
      t.z + d * Math.sin(polar) * Math.cos(az),
    );
    this.camera.lookAt(t);
    this.controls.update();
    this.requestRender();
  }

  /** Centre and zoom on a world-space box {minX,maxX,minZ,maxZ,maxY}. */
  fit(b, margin = 0.88) {
    const center = new THREE.Vector3((b.minX + b.maxX) / 2, 0, (b.minZ + b.maxZ) / 2);
    const offset = this.camera.position.clone().sub(this.controls.target);
    this.controls.target.copy(center);
    this.camera.position.copy(center).add(offset);
    this.camera.lookAt(center);
    this.camera.updateMatrixWorld();

    const inv = this.camera.matrixWorldInverse;
    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
    for (const x of [b.minX, b.maxX]) for (const y of [0, b.maxY]) for (const z of [b.minZ, b.maxZ]) {
      const v = new THREE.Vector3(x, y, z).applyMatrix4(inv);
      minX = Math.min(minX, v.x); maxX = Math.max(maxX, v.x);
      minY = Math.min(minY, v.y); maxY = Math.max(maxY, v.y);
    }
    const { clientWidth: w, clientHeight: h } = this.container;
    this.camera.zoom = Math.min(w / (maxX - minX), h / (maxY - minY)) * margin;
    this.camera.updateProjectionMatrix();
    this.controls.update();
    this.requestRender();
  }

  /** Pan (keeping zoom) so a world point is centred. */
  centerOn(x, y, z) {
    const target = new THREE.Vector3(x, y, z);
    const offset = this.camera.position.clone().sub(this.controls.target);
    this.controls.target.copy(target);
    this.camera.position.copy(target).add(offset);
    this.controls.update();
    this.requestRender();
  }

  /**
   * Renders at once and returns the canvas. Read it before yielding to the browser:
   * the drawing buffer is not preserved between frames.
   */
  renderNow() {
    this.renderer.render(this.scene, this.camera);
    return this.renderer.domElement;
  }

  /** World -> CSS pixel position within the container; null when behind the camera. */
  project(x, y, z) {
    const v = new THREE.Vector3(x, y, z).project(this.camera);
    if (v.z > 1 || v.z < -1) return null;
    const { clientWidth: w, clientHeight: h } = this.container;
    return { x: (v.x + 1) / 2 * w, y: (1 - v.y) / 2 * h };
  }
}

// Unit box with its base at y=0 and a brightness per face: top 1, sides as if lit from
// the north-west, bottom darkest. Values are linear-space multipliers.
function shadedBox() {
  const geo = new THREE.BoxGeometry(1, 1, 1).translate(0, 0.5, 0);
  const shade = [0.62, 0.62, 1, 0.3, 0.78, 0.78]; // +x, -x, +y, -y, +z, -z (BoxGeometry group order)
  // BoxGeometry is indexed with 4 vertices per face, faces in the order above.
  const perVertex = [];
  for (let f = 0; f < 6; f++) for (let v = 0; v < 4; v++) perVertex.push(shade[f], shade[f], shade[f]);
  geo.setAttribute('color', new THREE.Float32BufferAttribute(perVertex, 3));
  return geo;
}
