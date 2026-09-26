// The names over the map.
//
// Labels are HTML laid over the canvas, chosen greedily: the most important first,
// each one kept only where it covers nothing already placed and is on screen. The
// camera is a stand-in here - a flat projection of the map's ground plane onto the
// screen - which is all Labels asks of MapScene.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { Labels, covers } = await import('../static/labels.js');

const SCALE = 20; // pixels per map unit

/** A screen of the given size, and a camera looking straight down onto it. */
function screen(width = 800, height = 600) {
  const root = { clientWidth: width, clientHeight: height, append() {} };
  const scene = { project: (x, _y, z) => ({ x: x * SCALE, y: z * SCALE }), requestRender() {} };
  return new Labels(root, scene);
}

let nth = 0;
/** A layout box for a node of `kind` named `name`, at (x, z), `w` units across. */
const box = (kind, node, name, x, z, w = 8, extra = {}) => ({
  kind, x, z, y: 0, w, d: w, h: 0.3,
  node: { id: `n${nth++}`, kind: node, name, depth: 1, ...extra },
});

/**
 * Where a label element covers the screen. Labels positions an element by its anchor
 * (left, and top two pixels above it) and centres it there; the rectangle it keeps
 * clear is its text at 6.6 px a character plus 14 of padding, 18 high, ending 2 px
 * above the anchor. That is what "overlap" means here.
 */
function rect(el) {
  const w = el.textContent.length * 6.6 + 14;
  const x = parseFloat(el.style.left), y = parseFloat(el.style.top) + 2;
  return { x: x - w / 2, y: y - 20, w, h: 18 };
}
const overlaps = (a, b) => a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;

describe('labels', () => {
  // Verifies: REQ-MAP-042
  it('keep off what the walker is holding', () => {
    // The same crowd twice: once over an open screen, once with the walker's hands up
    // across its lower right quarter - the camera, say, held up showing a photograph.
    const boxes = [];
    for (let i = 0; i < 6; i++) {
      for (let j = 0; j < 6; j++) boxes.push(box('district', 'dir', `d${i}${j}`, 4 + i * 6, 4 + j * 5, 6));
    }
    const hands = { x: 400, y: 300, w: 400, h: 300 };
    const open = screen(), held = screen();
    open.set(boxes, null, null);
    open.draw();
    held.set(boxes, null, null);
    held.draw([hands]);
    assert.ok(open.visible().some(el => overlaps(rect(el), hands)), 'nothing would have been under the hands anyway');
    const over = held.visible().filter(el => overlaps(rect(el), hands));
    assert.deepEqual(over.map(el => el.textContent), [], 'a label was placed over the hands');
    assert.ok(held.visible().length > 0, 'the hands took every label off the screen');
  });

  // Verifies: REQ-MAP-042
  it('keep off what the walker is holding, to its outline and not its box', () => {
    // A mask of an 800x600 screen, 8x6 cells of 100px, the way WebGL reads it back -
    // bottom row first. Something is drawn in the bottom-right cell alone: the camera
    // held low, say, and nothing else in the way.
    const w = 8, h = 6, data = new Uint8Array(w * h * 4);
    data[(0 * w + 7) * 4 + 3] = 255; // row 0 is the bottom of the screen
    const mask = { data, w, h, width: 800, height: 600 };
    assert.ok(covers(mask, { x: 720, y: 530, w: 40, h: 18 }), 'the held camera is not covered');
    assert.ok(!covers(mask, { x: 720, y: 30, w: 40, h: 18 }), 'the top of the screen counts as the camera');
    assert.ok(!covers(mask, { x: 100, y: 530, w: 40, h: 18 }), 'the bottom left counts as the camera');
    assert.ok(!covers(null, { x: 0, y: 0, w: 800, h: 600 }), 'no mask covers something');

    // The whole crowd, with the camera in that one corner: labels everywhere else.
    const boxes = [];
    for (let i = 0; i < 6; i++) {
      for (let j = 0; j < 6; j++) boxes.push(box('district', 'dir', `m${i}${j}`, 4 + i * 6, 4 + j * 5, 6));
    }
    const open = screen(), held = screen();
    open.set(boxes, null, null);
    open.draw();
    held.set(boxes, null, null);
    held.draw(mask);
    const corner = { x: 700, y: 500, w: 100, h: 100 };
    assert.ok(held.visible().every(el => !overlaps(rect(el), corner)), 'a label was placed over the camera');
    assert.equal(held.visible().filter(el => !overlaps(rect(el), corner)).length,
      open.visible().filter(el => !overlaps(rect(el), corner)).length, 'the camera took labels from elsewhere');
  });

  // Verifies: REQ-MAP-042
  it('never overlap one another', () => {
    // A crowd of districts on a jittered grid tighter than a label, so that most of
    // them collide with a neighbor and have to be dropped.
    let seed = 7;
    const rand = () => (seed = (seed * 1103515245 + 12345) % 2 ** 31) / 2 ** 31;
    const boxes = [];
    for (let i = 0; i < 12; i++) {
      for (let j = 0; j < 12; j++) {
        boxes.push(box('district', 'dir', `dir-${i}-${j}`, 3 + i * 2.6 + rand(), 2 + j * 2.2 + rand(), 6));
      }
    }
    const labels = screen();
    labels.set(boxes, null, null);
    labels.draw();
    const shown = labels.visible();
    assert.ok(shown.length > 5, `only ${shown.length} labels placed`);
    assert.ok(shown.length < boxes.length, 'nothing was dropped: the test crowds nothing');
    const rectangles = shown.map(rect);
    for (let i = 0; i < rectangles.length; i++) {
      for (let j = i + 1; j < rectangles.length; j++) {
        assert.ok(!overlaps(rectangles[i], rectangles[j]), `"${shown[i].textContent}" overlaps "${shown[j].textContent}"`);
      }
      // ... and all of them are on screen.
      assert.ok(rectangles[i].x + rectangles[i].w >= 0 && rectangles[i].x <= 800 && rectangles[i].y + rectangles[i].h >= 0 && rectangles[i].y <= 600);
    }
  });

  // Verifies: REQ-MAP-042
  it('keep the selection over a region in the same place', () => {
    // The selected directory's terrace is also a region with a name of its own, and
    // an island sits on the same spot: the selection's label is the one kept.
    const terrace = box('terrace', 'dir', 'src', 20, 15, 12);
    const island = box('land', 'ecosystem', 'npm', 20, 15, 12);
    const labels = screen();
    labels.set([island, terrace], { arcs: [], selBox: terrace }, { name: 'selected-src' });
    labels.draw();
    const texts = labels.visible().map(el => el.textContent);
    assert.ok(texts.includes('selected-src'), `the selection was dropped for ${texts}`);
    assert.ok(!texts.includes('npm'), 'the island covers the selection');
  });

  // Verifies: REQ-MAP-042
  it('name arc ends before islands, and islands before directories', () => {
    const at = (x, z) => [
      box('district', 'dir', 'deep', x, z, 12, { depth: 3 }),
      box('land', 'ecosystem', 'pypi', x, z, 12),
    ];
    const sel = box('building', 'file', 'main.go', 5, 5, 1);
    const end = box('package', 'package', 'requests', 30, 20, 1);
    const labels = screen();
    labels.set([...at(30, 20), ...at(10, 25)], { arcs: [{ from: sel, to: end }], selBox: sel }, sel.node);
    labels.draw();
    const texts = labels.visible().map(el => el.textContent).sort();
    // At (30, 20) the arc's end wins over both regions; at (10, 25) the island wins
    // over the directory.
    assert.deepEqual(texts, ['main.go', 'pypi', 'requests']);
  });

  // Verifies: REQ-MAP-042
  it('stop at 160', () => {
    // Three hundred islands, each far from every other on a very large screen: every
    // one of them could be named, and only the first 160 are.
    const boxes = Array.from({ length: 300 }, (_, i) => box('land', 'ecosystem', `e${i}`, (i % 20) * 20, Math.floor(i / 20) * 5));
    const labels = new Labels(
      { clientWidth: 1e6, clientHeight: 1e6, append() {} },
      { project: (x, _y, z) => ({ x: 100 + x * SCALE, y: 100 + z * SCALE }), requestRender() {} },
    );
    labels.set(boxes, null, null);
    labels.draw();
    assert.equal(labels.visible().length, 160);
  });
});
