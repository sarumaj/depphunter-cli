// The map drawn without WebGL: a stand-in for three.js's WebGLRenderer that paints the
// same scene into a 2D canvas, a triangle at a time, on the CPU.
//
// A browser may have no WebGL to give - turned off, blocklisted for its driver, lost
// to a remote desktop - and the map is still a map: boxes, their colors, what is
// selected and what depends on what. So the scene is built as it always is, cameras,
// picking and labels being arithmetic that needs no GPU, and only the drawing is
// done here. It draws what the isometric view is made of and nothing else: every mesh
// in its flat color, shaded by its faces, and the lines over them. No shader runs, so
// there are no facades, streets or water - and no walk mode, which is all shader.
//
// It keeps a depth for every pixel rather than sorting the boxes back to front: boxes
// of different sizes standing in each other's way have no order that a sort by any
// one distance gets right, and a wrong one draws a far building over a near one.
//
// Implements: REQ-MAP-066

import * as THREE from '../vendor/three.module.min.js';

// Linear light to the bytes a canvas wants, as three.js's own output does it.
const SRGB = new Uint8Array(1024);
for (let i = 0; i < SRGB.length; i++) {
  const v = i / (SRGB.length - 1);
  SRGB[i] = Math.round(255 * (v <= 0.0031308 ? v * 12.92 : 1.055 * Math.pow(v, 1 / 2.4) - 0.055));
}
const byte = v => SRGB[v <= 0 ? 0 : v >= 1 ? 1023 : (v * 1023 + 0.5) | 0];
// A pixel as the image's buffer holds one, read as a 32-bit word: alpha, blue, green,
// red from the top down wherever the bytes are little-endian, which is everywhere a
// browser runs.
const pack = (r, g, b) => (0xff000000 | (byte(b) << 16) | (byte(g) << 8) | byte(r)) >>> 0;

const WHITE = new THREE.Color(1, 1, 1);
const _viewProjection = new THREE.Matrix4(), _object = new THREE.Matrix4(), _instance = new THREE.Matrix4();
// One edge of the triangle being filled: the rows it spans, where it starts and how far it moves a row.
const EDGES = [0, 1, 2].map(() => ({ from: 0, to: 0, x: 0, slope: 0 }));

export class FlatRenderer {
  /** `canvas`: where to draw; one is made where none is given. */
  constructor(canvas = document.createElement('canvas')) {
    this.domElement = canvas;
    this.context = canvas.getContext('2d');
    this.pixelRatio = 1;
    this.size = { w: 0, h: 0 }; // in CSS pixels
    this.image = null;  // the frame, as the canvas takes it
    this.color = null;  // ... and as words, a pixel each
    this.depth = null;  // how far away what each pixel shows is
    this.points = new Float32Array(0); // a mesh's vertices on the screen
    this.autoClear = true;
    // What the readout (stats.js) asks a renderer for.
    this.info = { autoReset: true, render: { calls: 0, triangles: 0 }, reset() { this.render.calls = this.render.triangles = 0; } };
  }

  getPixelRatio() { return this.pixelRatio; }

  setPixelRatio(ratio) {
    this.pixelRatio = ratio;
    this.setSize(this.size.w, this.size.h);
  }

  setSize(w, h) {
    this.size = { w, h };
    const width = Math.max(1, Math.floor(w * this.pixelRatio)), height = Math.max(1, Math.floor(h * this.pixelRatio));
    if (this.domElement.width !== width) this.domElement.width = width;
    if (this.domElement.height !== height) this.domElement.height = height;
  }

  /** There is nothing to compile: asked for by the warm-up, done at once. */
  compileAsync() { return Promise.resolve(); }

  /**
   * Draws `scene` as `camera` sees it. `background`: the color behind everything, in
   * linear light as three.js keeps its colors; `skip`: objects left out, with all
   * that hangs from them - what only a shader can draw.
   */
  render(scene, camera, { background = WHITE, skip = new Set() } = {}) {
    const W = this.domElement.width, H = this.domElement.height, context = this.context;
    if (!context || !W || !H) return;
    if (!this.image || this.image.width !== W || this.image.height !== H) {
      this.image = context.createImageData(W, H);
      this.color = new Uint32Array(this.image.data.buffer);
      this.depth = new Float32Array(W * H);
    }
    this.color.fill(pack(background.r, background.g, background.b));
    this.depth.fill(Infinity);
    scene.updateMatrixWorld();
    camera.updateMatrixWorld();
    _viewProjection.multiplyMatrices(camera.projectionMatrix, camera.matrixWorldInverse);
    // Solid things first, into the depth; then what is seen through, over them, in
    // the order three.js would take it; then the lines, which the canvas draws itself.
    const solid = [], sheer = [], lines = [];
    const gather = object => {
      if (!object.visible || skip.has(object)) return;
      if (object.layers.test(camera.layers)) {
        if (object.isMesh) (materialOf(object)?.transparent ? sheer : solid).push(object);
        else if (object.isLineSegments) lines.push(object);
      }
      for (const child of object.children) gather(child);
    };
    gather(scene);
    for (const mesh of solid) this.mesh(mesh, W, H);
    sheer.sort((a, b) => a.renderOrder - b.renderOrder);
    for (const mesh of sheer) this.mesh(mesh, W, H);
    context.putImageData(this.image, 0, 0);
    for (const line of lines) this.line(line, W, H);
  }

  /** One mesh, every instance of it: its triangles in the material's color, by the face. */
  mesh(mesh, W, H) {
    const material = materialOf(mesh), geometry = mesh.geometry;
    const position = geometry?.getAttribute('position');
    if (!material || !position || material.visible === false) return;
    const index = geometry.index?.array, p = position.array, stride = position.itemSize;
    const tint = material.color || WHITE;
    // Painted on the vertices: a box's shade by the face (scene.js shadedBox), a
    // flame's color and how far it is seen through (flames.js).
    const paint = material.vertexColors ? geometry.getAttribute('color') : null;
    const shade = paint?.array, channels = paint?.itemSize ?? 3;
    const alpha = material.transparent ? material.opacity : 1;
    const test = material.depthTest !== false, write = !material.transparent && material.depthWrite !== false;
    const side = material.side ?? THREE.FrontSide;
    const count = position.count, triangles = (index ? index.length : count) / 3 | 0;
    if (this.points.length < count * 3) this.points = new Float32Array(count * 3);
    const s = this.points, instances = mesh.isInstancedMesh ? mesh.count : 1, tints = mesh.instanceColor?.array;
    _object.multiplyMatrices(_viewProjection, mesh.matrixWorld);
    this.info.render.calls++;
    for (let n = 0; n < instances; n++) {
      const m = (mesh.isInstancedMesh ? _instance.multiplyMatrices(_object, _instance.fromArray(mesh.instanceMatrix.array, n * 16)) : _object).elements;
      // The vertices on the screen, and whether any of the instance is on it at all.
      let left = Infinity, right = -Infinity, top = Infinity, bottom = -Infinity;
      for (let v = 0; v < count; v++) {
        const x = p[v * stride], y = p[v * stride + 1], z = p[v * stride + 2];
        // An orthographic camera's w is 1; a perspective one's is divided out.
        const w = m[3] * x + m[7] * y + m[11] * z + m[15] || 1;
        const sx = ((m[0] * x + m[4] * y + m[8] * z + m[12]) / w * 0.5 + 0.5) * W;
        const sy = (0.5 - (m[1] * x + m[5] * y + m[9] * z + m[13]) / w * 0.5) * H;
        s[v * 3] = sx; s[v * 3 + 1] = sy; s[v * 3 + 2] = (m[2] * x + m[6] * y + m[10] * z + m[14]) / w;
        if (sx < left) left = sx;
        if (sx > right) right = sx;
        if (sy < top) top = sy;
        if (sy > bottom) bottom = sy;
      }
      if (!(right >= 0 && left <= W && bottom >= 0 && top <= H)) continue;
      const r = tint.r * (tints ? tints[n * 3] : 1), g = tint.g * (tints ? tints[n * 3 + 1] : 1), b = tint.b * (tints ? tints[n * 3 + 2] : 1);
      const plain = pack(r, g, b);
      for (let t = 0; t < triangles; t++) {
        const va = index ? index[t * 3] : t * 3, vb = index ? index[t * 3 + 1] : t * 3 + 1, vc = index ? index[t * 3 + 2] : t * 3 + 2;
        const a = va * 3, b1 = vb * 3, c = vc * 3;
        // Which way round it is on the screen says which side faces the eye: the y
        // axis points down here, so a front face runs clockwise.
        const area = (s[b1] - s[a]) * (s[c + 1] - s[a + 1]) - (s[c] - s[a]) * (s[b1 + 1] - s[a + 1]);
        if (area === 0 || (side === THREE.FrontSide && area > 0) || (side === THREE.BackSide && area < 0)) continue;
        // A triangle is one color all over: the mean of its corners', which for a
        // box's face is the face's own shade and for a flame is near enough.
        let word = plain, sheer = alpha;
        if (shade) {
          const i = va * channels, j = vb * channels, k = vc * channels;
          word = pack(r * (shade[i] + shade[j] + shade[k]) / 3, g * (shade[i + 1] + shade[j + 1] + shade[k + 1]) / 3, b * (shade[i + 2] + shade[j + 2] + shade[k + 2]) / 3);
          if (channels === 4 && material.transparent) sheer *= (shade[i + 3] + shade[j + 3] + shade[k + 3]) / 3;
        }
        if (sheer <= 0) continue;
        this.triangle(s[a], s[a + 1], s[a + 2], s[b1], s[b1 + 1], s[b1 + 2], s[c], s[c + 1], s[c + 2], word, sheer, test, write, W, H);
        this.info.render.triangles++;
      }
    }
  }

  /**
   * Fills one triangle, a row at a time: the pixels whose centers are inside it, each
   * only where nothing nearer has been drawn. Edges shared with the triangle next
   * door are drawn by exactly one of the two, so no seam shows and nothing is blended
   * twice.
   */
  triangle(x0, y0, z0, x1, y1, z1, x2, y2, z2, word, alpha, test, write, W, H) {
    const det = (x1 - x0) * (y2 - y0) - (x2 - x0) * (y1 - y0);
    if (!det) return;
    // The depth is a plane over the screen: so much a pixel across, so much a row down.
    const across = ((z1 - z0) * (y2 - y0) - (z2 - z0) * (y1 - y0)) / det;
    const down = ((z2 - z0) * (x1 - x0) - (z1 - z0) * (x2 - x0)) / det;
    edge(EDGES[0], x0, y0, x1, y1);
    edge(EDGES[1], x1, y1, x2, y2);
    edge(EDGES[2], x2, y2, x0, y0);
    const first = Math.max(0, Math.ceil(Math.min(y0, y1, y2) - 0.5)), end = Math.min(H - 1, Math.ceil(Math.max(y0, y1, y2) - 0.5) - 1);
    const color = this.color, depth = this.depth;
    const red = word & 0xff, green = (word >>> 8) & 0xff, blue = (word >>> 16) & 0xff;
    for (let y = first; y <= end; y++) {
      const row = y + 0.5;
      let from = Infinity, to = -Infinity;
      for (const e of EDGES) {
        if (row < e.from || row >= e.to) continue;
        const x = e.x + (row - e.from) * e.slope;
        if (x < from) from = x;
        if (x > to) to = x;
      }
      const start = Math.max(0, Math.ceil(from - 0.5)), stop = Math.min(W - 1, Math.ceil(to - 0.5) - 1);
      let z = z0 + (start + 0.5 - x0) * across + (row - y0) * down;
      for (let i = y * W + start, last = y * W + stop; i <= last; i++, z += across) {
        if (test && z >= depth[i]) continue;
        if (alpha >= 1) color[i] = word;
        else {
          const under = color[i];
          color[i] = (0xff000000
            | (((under >>> 16) & 0xff) + (blue - ((under >>> 16) & 0xff)) * alpha) << 16
            | (((under >>> 8) & 0xff) + (green - ((under >>> 8) & 0xff)) * alpha) << 8
            | ((under & 0xff) + (red - (under & 0xff)) * alpha)) >>> 0;
        }
        if (write) depth[i] = z;
      }
    }
  }

  /** Line segments - the outline of what is selected - stroked over the frame. */
  line(line, W, H) {
    const position = line.geometry?.getAttribute('position'), context = this.context;
    if (!position || !line.material) return;
    const m = _object.multiplyMatrices(_viewProjection, line.matrixWorld).elements, p = position.array;
    const at = v => {
      const x = p[v * 3], y = p[v * 3 + 1], z = p[v * 3 + 2], w = m[3] * x + m[7] * y + m[11] * z + m[15] || 1;
      return [((m[0] * x + m[4] * y + m[8] * z + m[12]) / w * 0.5 + 0.5) * W, (0.5 - (m[1] * x + m[5] * y + m[9] * z + m[13]) / w * 0.5) * H];
    };
    context.beginPath();
    for (let v = 0; v + 1 < position.count; v += 2) {
      context.moveTo(...at(v));
      context.lineTo(...at(v + 1));
    }
    context.strokeStyle = line.material.color?.getStyle() || '#000';
    // WebGL draws every line a pixel wide whatever its material asks; this need not.
    context.lineWidth = (line.material.linewidth || 1) * Math.max(1, this.pixelRatio);
    context.lineCap = 'round';
    context.globalAlpha = line.material.transparent ? line.material.opacity : 1;
    context.stroke();
    context.globalAlpha = 1;
  }
}

/** Whether this browser will give a page WebGL at all. */
export function hasWebGL() {
  try {
    const canvas = document.createElement('canvas');
    return !!(canvas.getContext('webgl2') || canvas.getContext('webgl'));
  } catch {
    return false;
  }
}

const materialOf = mesh => (Array.isArray(mesh.material) ? mesh.material[0] : mesh.material);

// Sets `e` to the edge from (xa, ya) to (xb, yb), top to bottom.
function edge(e, xa, ya, xb, yb) {
  if (ya > yb) { [xa, ya, xb, yb] = [xb, yb, xa, ya]; }
  e.from = ya;
  e.to = yb;
  e.x = xa;
  e.slope = yb > ya ? (xb - xa) / (yb - ya) : 0;
}
