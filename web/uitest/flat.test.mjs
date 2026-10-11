// The map drawn without WebGL (flat.js).

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const { FlatRenderer } = await import('../static/map/flat.js');
const { Flames } = await import('../static/hunt/flames.js');

// A canvas as far as the renderer asks of one: an image to fill and somewhere to stroke.
function canvas() {
  const drawn = { image: null, strokes: [] };
  const context = {
    createImageData: (w, h) => ({ width: w, height: h, data: new Uint8ClampedArray(w * h * 4) }),
    putImageData: image => { drawn.image = image; },
    beginPath() {}, moveTo(x, y) { drawn.strokes.push([x, y]); }, lineTo(x, y) { drawn.strokes.push([x, y]); }, stroke() {},
  };
  return { width: 0, height: 0, getContext: () => context, drawn };
}

// The color of the pixel at (x, y), as 0xRRGGBB.
const pixel = (image, x, y) => {
  const i = (y * image.width + x) * 4;
  return (image.data[i] << 16) | (image.data[i + 1] << 8) | image.data[i + 2];
};

// Straight down onto the map, a unit a pixel, the origin in the middle of a 40 by 40 view.
function above() {
  const camera = new THREE.OrthographicCamera(-20, 20, 20, -20, -100, 100);
  camera.position.set(0, 50, 0);
  camera.up.set(0, 0, -1);
  camera.lookAt(0, 0, 0);
  return camera;
}

const slab = (color, x, y, size, options = {}) => {
  const mesh = new THREE.Mesh(new THREE.BoxGeometry(size, 1, size), new THREE.MeshBasicMaterial({ color, ...options }));
  mesh.position.set(x, y, 0);
  return mesh;
};

describe('the map without WebGL', () => {
  // Verifies: REQ-MAP-066
  it('fills each box in its color, the nearer one over the farther whatever order they come in', () => {
    const target = canvas(), renderer = new FlatRenderer(target);
    renderer.setSize(40, 40);
    for (const order of [['low', 'high'], ['high', 'low']]) {
      const scene = new THREE.Scene();
      const boxes = { low: slab(0xff0000, 0, 0, 20), high: slab(0x0000ff, 5, 3, 10) };
      for (const name of order) scene.add(boxes[name]);
      renderer.render(scene, above(), { background: new THREE.Color(0x00ff00) });
      const image = target.drawn.image;
      assert.equal(pixel(image, 2, 2), 0x00ff00, 'the background is not where nothing stands');
      assert.equal(pixel(image, 14, 20), 0xff0000, 'the low box is not drawn in its color');
      assert.equal(pixel(image, 25, 20), 0x0000ff, `the box above is hidden by the one under it, drawn ${order.join(' then ')}`);
      assert.equal(pixel(image, 33, 20), 0x00ff00, 'a box is drawn past its own edge');
    }
  });

  // Verifies: REQ-MAP-066
  it('draws every instance in its own color, shaded by the face, and leaves out what it is told to', () => {
    const target = canvas(), renderer = new FlatRenderer(target);
    renderer.setSize(40, 40);
    const geometry = new THREE.BoxGeometry(8, 1, 8);
    // Half as bright on every face, as scene.js's box shades its sides.
    geometry.setAttribute('color', new THREE.Float32BufferAttribute(new Array(geometry.getAttribute('position').count * 3).fill(0.5), 3));
    const mesh = new THREE.InstancedMesh(geometry, new THREE.MeshBasicMaterial({ vertexColors: true }), 2);
    mesh.setMatrixAt(0, new THREE.Matrix4().makeTranslation(-10, 0, 0));
    mesh.setMatrixAt(1, new THREE.Matrix4().makeTranslation(10, 0, 0));
    mesh.setColorAt(0, new THREE.Color(1, 0, 0));
    mesh.setColorAt(1, new THREE.Color(0, 0, 1));
    const sea = slab(0xffffff, 0, -5, 40);
    const scene = new THREE.Scene().add(mesh, sea);
    renderer.render(scene, above(), { background: new THREE.Color(0, 0, 0), skip: new Set([sea]) });
    const image = target.drawn.image;
    // Half of full red in linear light is 188 of 255 on the screen.
    assert.equal(pixel(image, 10, 20), 0xbc0000);
    assert.equal(pixel(image, 30, 20), 0x0000bc);
    assert.equal(pixel(image, 20, 20), 0x000000, 'what was to be left out was drawn');
    assert.equal(renderer.info.render.calls, 1);
  });

  // Verifies: REQ-MAP-066
  it('lays what is seen through over what is behind it, and strokes the outline on top', () => {
    const target = canvas(), renderer = new FlatRenderer(target);
    renderer.setSize(40, 40);
    const outline = new THREE.LineSegments(
      new THREE.BufferGeometry().setAttribute('position', new THREE.Float32BufferAttribute([-10, 0, 0, 10, 0, 0], 3)),
      new THREE.LineBasicMaterial({ color: 0xffffff }));
    const scene = new THREE.Scene().add(slab(0xffffff, 0, 3, 10, { transparent: true, opacity: 0.5 }), slab(0x000000, 0, 0, 20), outline);
    renderer.render(scene, above(), { background: new THREE.Color(0, 0, 0) });
    const mixed = pixel(target.drawn.image, 20, 20) & 0xff;
    assert.ok(mixed > 120 && mixed < 135, `half white over black came out ${mixed}`);
    assert.deepEqual(target.drawn.strokes.map(at => at.map(Math.round)), [[10, 20], [30, 20]]);
  });

  // Verifies: REQ-MAP-066
  it('draws a fire in its own colors over the roof it burns on, seen through at its edges', () => {
    const target = canvas(), renderer = new FlatRenderer(target);
    renderer.setSize(80, 80);
    const scene = new THREE.Scene();
    const flames = new Flames({ scene, bendable: m => m }, 1);
    flames.show(true);
    flames.place([{ id: 'a', heat: 1 }], () => ({ x: 0, z: 0, y: 1, h: 0, w: 12, d: 12 }));
    flames.step(0.4);
    // From the side, as the map's camera sees a flame standing: 16 pixels a unit.
    const camera = new THREE.OrthographicCamera(-2.5, 2.5, 4, -1, -100, 100);
    camera.position.set(0, 1.5, 50);
    camera.lookAt(0, 1.5, 0);
    renderer.render(scene, camera, { background: new THREE.Color(0, 0, 0) });
    const image = target.drawn.image;
    // Over black, smoke is next to nothing; a flame is red over green over blue, more or less of
    // the dark showing through it - and never the grey its paint would be if read as a shade.
    let fire = 0, solid = 0, grey = 0;
    for (let y = 0; y < 80; y++) for (let x = 0; x < 80; x++) {
      const c = pixel(image, x, y), r = c >> 16, g = (c >> 8) & 0xff, b = c & 0xff;
      if (r < 60) continue;
      if (r > g && g > b) { fire++; if (r > 200) solid++; }
      else if (Math.abs(r - g) < 12 && Math.abs(g - b) < 12) grey++;
    }
    assert.ok(fire > 20, `only ${fire} pixels of flame`);
    assert.ok(solid > 0 && solid < fire, 'a flame has a hard edge: nothing of what is behind it shows through');
    assert.equal(grey, 0, 'a flame is drawn grey');
  });
});
