// three.js rendering of a layout: one InstancedMesh for every box, tubes for edges.
// Renders on demand only; nothing animates unless the camera or the state changes.
//
// Walk mode (walk.js) views the same scene through a perspective camera and bends the
// flat map onto a small planet centred under the walker: every material shares the
// `curve` uniforms, and a vertex shader wraps world positions around the sphere.
// Large flat boxes (land, terraces, districts) are drawn from a tessellated copy in
// walk mode so their tops follow the curve instead of cutting through it as chords.
// city.js dresses both views up as a city (facades, streets, props); walk mode adds
// sky and water.

import * as THREE from './vendor/three.module.min.js';
import { OrbitControls } from './vendor/OrbitControls.js';
import { kindCode, CITY_VERT_HEAD, CITY_VERT_BODY, CITY_FRAG_HEAD, CITY_FRAG_BODY, makeSky, waterMaterial, makeProps, setNight, roadUniforms, setRoads } from './city.js';

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
    this.walkCamera = new THREE.PerspectiveCamera(70, 1, 0.02, 3000);
    this.walkCamera.rotation.order = 'YXZ';
    this.walking = false;
    // center: the walker's position on the flat map; radius: the planet's; night:
    // the dark theme's city; time: seconds, for clouds and water.
    this.curve = {
      uCenter: { value: new THREE.Vector3() }, uRadius: { value: 40 }, uBend: { value: 0 },
      uNight: { value: 0 }, uTime: { value: 0 },
    };
    this.roads = roadUniforms(); // the street network of walk mode (city.js)

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
    // Pan and zoom stay near the map (setLimits): the view can never be lost in the void.
    this.limits = null;
    this.controls.addEventListener('change', () => {
      this.clampView();
      this.requestRender();
    });

    this.unitBox = shadedBox();
    // Unlit: the top face shows exactly the encoded color; fixed per-face shading
    // (vertex colors, multiplied with the instance color) gives the 3D form.
    this.material = this.bendable(new THREE.MeshBasicMaterial({ vertexColors: true }), true);
    this.mesh = null;
    this.ground = null; // tessellated large boxes, shown in walk mode
    this.props = null;  // trees, bushes, lamps and ramps
    this.planet = new THREE.Mesh(new THREE.SphereGeometry(1, 96, 48), waterMaterial(this.curve));
    this.sky = makeSky(this.curve);
    this.planet.visible = this.sky.visible = false;
    // The isometric map's sea: a flat plane at the islands' feet, far larger than any
    // view of the map can show (sized by setLimits); walk mode has the planet instead.
    this.sea = new THREE.Mesh(new THREE.PlaneGeometry(1, 1).rotateX(-Math.PI / 2), waterMaterial(this.curve));
    this.sea.position.y = SEA_LEVEL;
    this.sea.renderOrder = -1;
    this.scene.add(this.planet, this.sky, this.sea);
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
    this.updateMinZoom();
    this.walkCamera.aspect = w / Math.max(1, h);
    this.walkCamera.updateProjectionMatrix();
    this.requestRender();
  }

  requestRender() {
    if (this.pending) return;
    this.pending = true;
    requestAnimationFrame(() => {
      this.pending = false;
      this.renderNow();
      this.onRender?.();
    });
  }

  /** The camera in use: isometric, or the walker's eyes. */
  get view() {
    return this.walking ? this.walkCamera : this.camera;
  }

  /**
   * colors: {water, sky, skyTop, sea}: the map's background, and walk mode's horizon,
   * zenith and sea. A dark zenith makes walk mode a night city.
   */
  setBackground(colors) {
    this.colors = colors;
    // The street and lawn shading's reference colors (city.js tint).
    if (colors.ground) this.roads.uGroundRef.value.set(colors.ground);
    if (colors.land) this.roads.uLandRef.value.set(colors.land);
    this.scene.background = new THREE.Color(this.walking ? colors.sky : colors.water);
    this.planet.material.color.set(colors.sea);
    this.sea.material.color.set(colors.sea);
    this.sky.material.uniforms.uTop.value.set(colors.skyTop);
    this.sky.material.uniforms.uHorizon.value.set(colors.sky);
    const night = new THREE.Color(colors.skyTop).getHSL({}).l < 0.15;
    this.curve.uNight.value = night ? 1 : 0;
    if (this.props) setNight(this.props, night);
    if (this.walking) this.scene.fog.color.set(colors.sky);
    this.requestRender();
  }

  /**
   * Patches a material to bend its vertices onto the planet while walking; city
   * materials (the boxes) also get walk mode's facades and streets.
   */
  bendable(material, city = false) {
    material.onBeforeCompile = shader => {
      Object.assign(shader.uniforms, this.curve);
      let body = BENT_PROJECT_VERTEX;
      if (city) {
        Object.assign(shader.uniforms, this.roads);
        body += CITY_VERT_BODY;
        shader.fragmentShader = CITY_FRAG_HEAD + shader.fragmentShader
          .replace('#include <color_fragment>', '#include <color_fragment>\n' + CITY_FRAG_BODY);
      }
      shader.vertexShader = BEND_GLSL + (city ? CITY_VERT_HEAD : '') + shader.vertexShader.replace('#include <project_vertex>', body);
    };
    material.customProgramCacheKey = () => (city ? 'bend-city' : 'bend');
    return material;
  }

  /**
   * Switches between the isometric map and walk mode. radius: the planet's; the
   * walker's camera and centre are set with setWalker.
   */
  setWalking(on, radius) {
    this.walking = on;
    this.curve.uBend.value = on ? 1 : 0;
    if (radius) this.setRadius(radius);
    this.controls.enabled = !on;
    this.scene.fog = on ? new THREE.Fog(this.colors.sky, 30, 160) : null;
    this.planet.visible = this.sky.visible = on;
    this.sea.visible = !on;
    if (on && this.groundColors) this.setColors(...this.groundColors); // deferred while off
    this.setBackground(this.colors);
    this.showGround();
    if (this.outlined) this.setOutline(...this.outlined);
  }

  setRadius(r) {
    this.curve.uRadius.value = r;
    // The water surface sits at the bottom of the land boxes (layout LAND_H).
    this.planet.scale.setScalar(r - WATER_DEPTH);
  }

  /** Places the walker: flat-map feet position, eye height, yaw and pitch (radians). */
  setWalker(x, feet, z, eye, yaw, pitch) {
    this.curve.uTime.value = performance.now() / 1000;
    this.sky.position.set(x, feet + eye, z);
    this.curve.uCenter.value.set(x, 0, z);
    this.planet.position.set(x, -this.curve.uRadius.value, z);
    this.walkCamera.position.set(x, feet + eye, z);
    this.walkCamera.rotation.set(pitch, yaw, 0);
    this.walkCamera.updateMatrixWorld();
  }

  /** Replace all boxes. colors: array of CSS colors, one per box. */
  setBoxes(boxes, colors) {
    // New buffers: nothing of the old colors survives for setColors to compare against.
    this.shown = null;
    this.groundPainted = false;
    if (this.mesh) {
      this.scene.remove(this.mesh);
      this.mesh.geometry.dispose();
      this.mesh.dispose();
    }
    this.boxes = boxes;
    const geo = this.unitBox.clone();
    geo.setAttribute('aKind', new THREE.InstancedBufferAttribute(Float32Array.from(boxes, kindCode), 1));
    geo.setAttribute('aFade', new THREE.InstancedBufferAttribute(new Float32Array(boxes.length), 1));
    const mesh = new THREE.InstancedMesh(geo, this.material, boxes.length);
    const m = new THREE.Matrix4(), q = new THREE.Quaternion(), p = new THREE.Vector3(), s = new THREE.Vector3();
    boxes.forEach((b, i) => {
      m.compose(p.set(b.x, b.y, b.z), q, s.set(b.w, Math.max(b.h, 0.01), b.d));
      mesh.setMatrixAt(i, m);
    });
    mesh.computeBoundingSphere();
    this.mesh = mesh;
    if (this.ground) {
      this.scene.remove(this.ground);
      this.ground.geometry.dispose();
    }
    this.ground = new THREE.Mesh(tessellate(boxes.filter(isLarge)), this.material);
    this.ground.frustumCulled = false; // bent vertices leave the flat bounding sphere
    this.scene.add(this.ground);
    if (this.props) {
      this.scene.remove(this.props);
      // Each layout gets its own prop materials and ramp geometry; the instanced
      // plants and lamps share their geometry across layouts (city.js), so only their
      // instance buffers go.
      for (const m of this.props.children) {
        m.material.dispose();
        if (m.isInstancedMesh) m.dispose();
        else m.geometry.dispose();
      }
    }
    this.props = makeProps(boxes, m => this.bendable(m));
    setRoads(this.roads, boxes); // both views draw the streets
    if (this.colors) setNight(this.props, this.curve.uNight.value > 0);
    this.scene.add(this.props);
    this.setColors(colors);
    this.scene.add(mesh);
    this.showGround();
    this.setLimits(boxes);
  }

  /**
   * Bounds the isometric view to the map: the point the camera looks at stays within
   * the map plus a margin, and zooming out stops when the whole map is a fraction of
   * the screen.
   */
  setLimits(boxes) {
    let minX = Infinity, maxX = -Infinity, minZ = Infinity, maxZ = -Infinity, maxY = 0;
    for (const b of boxes) {
      minX = Math.min(minX, b.x - b.w / 2); maxX = Math.max(maxX, b.x + b.w / 2);
      minZ = Math.min(minZ, b.z - b.d / 2); maxZ = Math.max(maxZ, b.z + b.d / 2);
      maxY = Math.max(maxY, b.y + b.h);
    }
    if (!boxes.length) { minX = minZ = -1; maxX = maxZ = 1; }
    this.limits = { minX, maxX, minZ, maxZ, maxY };
    this.sea.position.set((minX + maxX) / 2, SEA_LEVEL, (minZ + maxZ) / 2);
    this.sea.scale.setScalar(SEA_SIZE * Math.max(50, maxX - minX, maxZ - minZ));
    this.updateMinZoom();
    this.clampView();
  }

  updateMinZoom() {
    if (!this.limits) return;
    this.controls.minZoom = this.fitZoom(this.limits) * MIN_ZOOM_SHARE;
    if (this.camera.zoom < this.controls.minZoom) {
      this.camera.zoom = this.controls.minZoom;
      this.camera.updateProjectionMatrix();
    }
  }

  clampView() {
    const b = this.limits;
    if (!b || this.walking) return;
    const m = PAN_MARGIN_MIN + PAN_MARGIN * Math.max(b.maxX - b.minX, b.maxZ - b.minZ);
    const t = this.controls.target;
    const d = new THREE.Vector3(
      clamp(t.x, b.minX - m, b.maxX + m) - t.x,
      clamp(t.y, -1, b.maxY) - t.y,
      clamp(t.z, b.minZ - m, b.maxZ + m) - t.z,
    );
    if (d.lengthSq() === 0) return;
    // Moving both keeps the view direction, so the controls' next update agrees.
    t.add(d);
    this.camera.position.add(d);
    this.camera.updateMatrixWorld();
  }

  // In walk mode the tessellated ground replaces the large instances, which are
  // shrunk to nothing; the isometric map shows the instances only.
  showGround() {
    if (!this.mesh) return;
    this.ground.visible = this.walking;
    this.mesh.frustumCulled = !this.walking;
    const m = new THREE.Matrix4(), q = new THREE.Quaternion(), p = new THREE.Vector3(), s = new THREE.Vector3();
    this.boxes.forEach((b, i) => {
      if (!isLarge(b)) return;
      const k = this.walking ? 0 : 1;
      this.mesh.setMatrixAt(i, m.compose(p.set(b.x, b.y, b.z), q, s.set(b.w * k, Math.max(b.h, 0.01) * k, b.d * k)));
    });
    this.mesh.instanceMatrix.needsUpdate = true;
    this.requestRender();
  }

  /**
   * colors: CSS colors, one per box. faded: optional flags, one per box: faded boxes
   * (dimmed by a selection or filter) drop their facade and roof detail and show
   * their plain color, so what is in focus stands out.
   *
   * Only boxes whose color or fade changed are written. Pointing at a building
   * recolors two boxes, and repainting the ground's hundreds of thousands of vertex
   * colors for that took longer than a frame; the callers hand over fresh arrays
   * every time and do not keep them, so the last ones serve as the comparison.
   */
  setColors(colors, faded = []) {
    parsed.clear();
    const was = this.shown, wasFaded = this.shownFade;
    const changed = [];
    for (let i = 0; i < colors.length; i++) {
      if (!was || colors[i] !== was[i] || !faded[i] !== !wasFaded[i]) changed.push(i);
    }
    this.shown = colors;
    this.shownFade = faded;
    if (changed.length) {
      const fade = this.mesh.geometry.getAttribute('aFade');
      for (const i of changed) {
        this.mesh.setColorAt(i, parseColor(colors[i]));
        fade.setX(i, faded[i] ? 1 : 0);
      }
      if (this.mesh.instanceColor) this.mesh.instanceColor.needsUpdate = true;
      fade.needsUpdate = true;
    }
    // The ground's vertex colors are its face shade times its box's color. It is only
    // drawn while walking, so outside walk mode the (much larger) buffer is left for
    // setWalking to refresh.
    this.groundColors = [colors, faded];
    if (this.walking) {
      this.paintGround(colors, faded, this.groundPainted ? changed : null);
      this.groundPainted = true;
    } else {
      this.groundPainted = false;
    }
    this.requestRender();
  }

  /** Paints the ground's vertex colors, for the given box indexes or for all of them. */
  paintGround(colors, faded, only) {
    const g = this.ground.geometry, ranges = g.userData.ranges;
    const shade = g.getAttribute('shade'), box = g.getAttribute('box');
    const col = g.getAttribute('color'), gFade = g.getAttribute('aFade');
    const c = new THREE.Color();
    const paint = (from, to) => {
      for (let v = from; v < to; v++) {
        const i = box.getX(v);
        c.copy(parseColor(colors[i])).multiplyScalar(shade.getX(v));
        col.setXYZ(v, c.r, c.g, c.b);
        gFade.setX(v, faded[i] ? 1 : 0);
      }
    };
    if (only) {
      let any = false;
      for (const i of only) {
        const r = ranges.get(i);
        if (!r) continue; // a box too small to be part of the ground
        paint(r[0], r[1]);
        col.addUpdateRange(r[0] * 3, (r[1] - r[0]) * 3);
        gFade.addUpdateRange(r[0], r[1] - r[0]);
        any = true;
      }
      if (!any) return; // nothing the ground shows changed: no upload at all
    } else {
      paint(0, box.count);
      // No ranges: the whole buffer goes to the GPU.
      col.clearUpdateRanges();
      gFade.clearUpdateRanges();
    }
    col.needsUpdate = true;
    gFade.needsUpdate = true;
  }

  /** Box index under a client-space point, or -1. */
  pick(clientX, clientY) {
    if (!this.mesh) return -1;
    if (this.walking) return -1; // walk.js aims along the bent view instead
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
    this.outlined = [box, color];
    if (box) {
      // The map shows the selection through whatever hides it; in walk mode, where
      // big blocks surround the walker, only the visible edges.
      const geo = outlineGeometry(box.w + 0.08, box.h + 0.08, box.d + 0.08);
      this.outline = new THREE.LineSegments(geo, this.bendable(new THREE.LineBasicMaterial({ color, depthTest: this.walking, transparent: true })));
      this.outline.position.set(box.x, box.y - 0.04, box.z); // the geometry's base is at y=0
      this.outline.renderOrder = 10;
      this.outline.frustumCulled = false;
      this.scene.add(this.outline);
    }
    this.requestRender();
  }

  /** arcs: [{from: box, to: box, color, count}] - drawn as raised curves with an arrow head at the target. */
  setArcs(arcs) {
    for (const c of this.edgeGroup.children) c.geometry.dispose();
    this.edgeGroup.clear();
    const mats = new Map();
    const mat = color => {
      if (!mats.has(color)) mats.set(color, this.bendable(new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.9 })));
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
      const tube = new THREE.Mesh(new THREE.TubeGeometry(curve, 32, r, 5, false), mat(a.color));
      tube.frustumCulled = false; // bounds are flat; walk mode draws the tube bent
      this.edgeGroup.add(tube);

      const head = new THREE.Mesh(new THREE.ConeGeometry(r * 3, r * 7, 8), mat(a.color));
      const tangent = curve.getTangent(0.97);
      head.position.copy(curve.getPoint(0.97));
      head.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), tangent);
      head.frustumCulled = false;
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

    this.camera.zoom = clamp(this.fitZoom(b) * margin, this.controls.minZoom, this.controls.maxZoom);
    this.camera.updateProjectionMatrix();
    this.controls.update();
    this.requestRender();
  }

  /** The zoom at which a world-space box exactly fills the view, as currently oriented. */
  fitZoom(b) {
    this.camera.updateMatrixWorld();
    const inv = this.camera.matrixWorldInverse;
    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
    for (const x of [b.minX, b.maxX]) for (const y of [0, b.maxY]) for (const z of [b.minZ, b.maxZ]) {
      const v = new THREE.Vector3(x, y, z).applyMatrix4(inv);
      minX = Math.min(minX, v.x); maxX = Math.max(maxX, v.x);
      minY = Math.min(minY, v.y); maxY = Math.max(maxY, v.y);
    }
    const { clientWidth: w, clientHeight: h } = this.container;
    return Math.min(w / Math.max(1e-6, maxX - minX), h / Math.max(1e-6, maxY - minY));
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
    this.renderer.render(this.scene, this.view);
    return this.renderer.domElement;
  }

  /**
   * World -> CSS pixel position within the container; null when behind the camera.
   * Labels call this four times per box on every frame, so it borrows scratch
   * vectors instead of allocating.
   */
  project(x, y, z) {
    const v = _p.set(x, y, z);
    if (this.walking) {
      this.bend(v);
      // Behind the walker, or beyond the horizon (below the water surface).
      if (_q.copy(v).applyMatrix4(this.walkCamera.matrixWorldInverse).z > -this.walkCamera.near) return null;
      const c = this.planet.position, eye = this.walkCamera.position;
      if (occludedBySphere(eye, v, c, this.planet.scale.x)) return null;
    }
    v.project(this.view);
    if (v.z > 1 || v.z < -1) return null;
    const { clientWidth: w, clientHeight: h } = this.container;
    return { x: (v.x + 1) / 2 * w, y: (1 - v.y) / 2 * h };
  }

  /**
   * Flat-map point -> where walk mode draws it (in place). Mirrors BEND_GLSL: the point
   * keeps its height above the surface and its distance along it from the centre.
   */
  bend(v) {
    const c = this.curve.uCenter.value, R = this.curve.uRadius.value;
    const dx = v.x - c.x, dz = v.z - c.z, r = Math.hypot(dx, dz);
    if (r < 1e-6) return v;
    const th = Math.min(r / R, Math.PI), rr = R + v.y;
    return v.set(c.x + dx / r * rr * Math.sin(th), rr * Math.cos(th) - R, c.z + dz / r * rr * Math.sin(th));
  }

  /** Inverse of bend: a point in walk-mode space -> the flat-map point it shows (in place). */
  unbend(v) {
    const c = this.curve.uCenter.value, R = this.curve.uRadius.value;
    const dx = v.x - c.x, dy = v.y + R, dz = v.z - c.z;
    const horizontal = Math.hypot(dx, dz);
    const h = Math.hypot(horizontal, dy) - R;
    if (horizontal < 1e-9) return v.set(c.x, h, c.z);
    const r = Math.atan2(horizontal, dy) * R;
    return v.set(c.x + dx / horizontal * r, h, c.z + dz / horizontal * r);
  }
}

const WATER_DEPTH = 0.45;
// The isometric sea: just above the land boxes' base (layout LAND_H below the
// mainland), so shores meet the water without a sliver of box bottom, and this many
// map sizes across.
const SEA_LEVEL = -WATER_DEPTH + 0.03, SEA_SIZE = 40;
// Zooming out stops when the map fills this share of the view; panning stops when
// the view's centre is this far beyond the map (a share of its size, plus a minimum).
const MIN_ZOOM_SHARE = 0.35, PAN_MARGIN = 0.25, PAN_MARGIN_MIN = 6;

const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

// Colors arrive as CSS strings, one per box, and every ground vertex reads its box's
// again; parsing one takes long enough to be worth doing once per color. setColors
// empties the cache on entry, so it never outlives a single pass.
const parsed = new Map();
function parseColor(css) {
  let c = parsed.get(css);
  if (!c) parsed.set(css, c = new THREE.Color(css));
  return c;
}

// Wraps a world position around a sphere of radius uRadius touching the flat map at
// uCenter: distance along the surface and height above it are preserved, so vertical
// edges stay straight (radial) and flat faces curve with the planet.
const BEND_GLSL = `
uniform vec3 uCenter;
uniform float uRadius;
uniform float uBend;
vec3 bendWorld(vec3 p) {
  vec2 d = p.xz - uCenter.xz;
  float r = length(d);
  if (uBend < 0.5 || r < 1e-5) return p;
  float th = min(r / uRadius, 3.14159265);
  float rr = uRadius + p.y;
  vec2 o = d / r * rr * sin(th);
  return vec3(uCenter.x + o.x, rr * cos(th) - uRadius, uCenter.z + o.y);
}
`;

// three.js's project_vertex chunk with the bend between model and view transforms.
const BENT_PROJECT_VERTEX = `
vec4 mvPosition = vec4(transformed, 1.0);
#ifdef USE_INSTANCING
  mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelMatrix * mvPosition;
mvPosition.xyz = bendWorld(mvPosition.xyz);
mvPosition = viewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;
`;

// Scratch vectors for project and occludedBySphere, which run per label per frame.
const _p = new THREE.Vector3(), _q = new THREE.Vector3(), _d = new THREE.Vector3(), _oc = new THREE.Vector3();

// Whether the segment eye->p passes through the sphere (centre c, radius r).
function occludedBySphere(eye, p, c, r) {
  const d = _d.subVectors(p, eye), len = d.length();
  d.divideScalar(len);
  const oc = _oc.subVectors(eye, c);
  const b = oc.dot(d), disc = b * b - (oc.lengthSq() - r * r);
  if (disc < 0) return false;
  const t = -b - Math.sqrt(disc);
  return t > 0 && t < len - 0.01;
}

const isLarge = b => Math.max(b.w, b.d) > 1.5;

// Face brightness, as in shadedBox: top, +x, -x, +z, -z.
const SHADE = { top: 1, px: 0.62, nx: 0.62, pz: 0.78, nz: 0.78 };

/**
 * One geometry for many boxes, their tops gridded and their sides split along the
 * length (heights need no split: the bend keeps verticals straight). Bottoms are
 * left out; they face the planet. Attributes: position, normal, color (set by
 * setColors), shade (face brightness), box (the box index), and for city.js aKind,
 * aBoxCenter (base centre) and aBoxSize. userData.ranges maps a box index to its
 * vertex range, so one box can be repainted without walking the whole buffer.
 */
function tessellate(boxes) {
  const area = boxes.reduce((a, b) => a + b.w * b.d, 0);
  const cell = Math.max(0.75, Math.sqrt(area / 150000)); // bounds the vertex count
  const pos = [], shade = [], box = [], index = [], normal = [], kind = [], center = [], size = [];
  let cur;
  const quadGrid = (i, s, n, nu, nv, at) => {
    const base = pos.length / 3;
    for (let v = 0; v <= nv; v++) for (let u = 0; u <= nu; u++) {
      pos.push(...at(u / nu, v / nv));
      shade.push(s);
      box.push(i);
      normal.push(...n);
      kind.push(kindCode(cur));
      center.push(cur.x, cur.y, cur.z);
      size.push(cur.w, Math.max(cur.h, 0.01), cur.d);
    }
    for (let v = 0; v < nv; v++) for (let u = 0; u < nu; u++) {
      const a = base + v * (nu + 1) + u, b = a + 1, c = a + nu + 1, d = c + 1;
      index.push(a, c, b, b, c, d);
    }
  };
  const ranges = new Map();
  for (const b of boxes) {
    const start = pos.length / 3;
    const x0 = b.x - b.w / 2, x1 = b.x + b.w / 2, z0 = b.z - b.d / 2, z1 = b.z + b.d / 2;
    const y0 = b.y, y1 = b.y + Math.max(b.h, 0.01);
    const nx = Math.max(1, Math.ceil(b.w / cell)), nz = Math.max(1, Math.ceil(b.d / cell));
    const i = b.i;
    cur = b;
    quadGrid(i, SHADE.top, [0, 1, 0], nx, nz, (u, v) => [x0 + u * b.w, y1, z0 + v * b.d]);
    quadGrid(i, SHADE.pz, [0, 0, 1], nx, 1, (u, v) => [x0 + u * b.w, y1 - v * (y1 - y0), z1]);
    quadGrid(i, SHADE.nz, [0, 0, -1], nx, 1, (u, v) => [x1 - u * b.w, y1 - v * (y1 - y0), z0]);
    quadGrid(i, SHADE.px, [1, 0, 0], nz, 1, (u, v) => [x1, y1 - v * (y1 - y0), z1 - u * b.d]);
    quadGrid(i, SHADE.nx, [-1, 0, 0], nz, 1, (u, v) => [x0, y1 - v * (y1 - y0), z0 + u * b.d]);
    ranges.set(i, [start, pos.length / 3]);
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(new Float32Array(pos.length), 3));
  geo.setAttribute('shade', new THREE.Float32BufferAttribute(shade, 1));
  geo.setAttribute('box', new THREE.Float32BufferAttribute(box, 1));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(normal, 3));
  geo.setAttribute('aKind', new THREE.Float32BufferAttribute(kind, 1));
  geo.setAttribute('aBoxCenter', new THREE.Float32BufferAttribute(center, 3));
  geo.setAttribute('aBoxSize', new THREE.Float32BufferAttribute(size, 3));
  geo.setAttribute('aFade', new THREE.Float32BufferAttribute(new Float32Array(pos.length / 3), 1));
  geo.setIndex(index);
  geo.userData.ranges = ranges;
  return geo;
}

// A box's 12 edges as line segments, split into short pieces so they bend with the
// planet in walk mode. Base at y=0, centred on x and z like the boxes.
function outlineGeometry(w, h, d) {
  const pts = [];
  const edge = (a, b) => {
    const n = Math.max(1, Math.ceil(Math.hypot(b[0] - a[0], b[2] - a[2]) / 0.75));
    for (let k = 0; k < n; k++) {
      const t0 = k / n, t1 = (k + 1) / n;
      for (const t of [t0, t1]) pts.push(a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t);
    }
  };
  const x = w / 2, z = d / 2;
  for (const y of [0, h]) {
    edge([-x, y, -z], [x, y, -z]); edge([x, y, -z], [x, y, z]);
    edge([x, y, z], [-x, y, z]); edge([-x, y, z], [-x, y, -z]);
  }
  for (const [sx, sz] of [[-1, -1], [1, -1], [1, 1], [-1, 1]]) edge([sx * x, 0, sz * z], [sx * x, h, sz * z]);
  return new THREE.BufferGeometry().setAttribute('position', new THREE.Float32BufferAttribute(pts, 3));
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
