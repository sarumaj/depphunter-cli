// The programs compiled ahead of the frames that need them (MapScene.warmUp).
//
// The compiling is WebGL's; what is checked here is what it is asked for - which
// materials, in which style, with or without the fog - and that the scene is left as
// it was found.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { MapScene } = await import('../static/scene.js');
const THREE = await import('../static/vendor/three.module.min.js');

// Enough of a MapScene to warm: a scene with the boxes and a prop, and a renderer that
// records what it is asked to compile.
function fake() {
  const scene = new THREE.Scene();
  const boxes = new THREE.Mesh(new THREE.BoxGeometry(), new THREE.MeshBasicMaterial());
  const tree = new THREE.Mesh(new THREE.BoxGeometry(), new THREE.MeshBasicMaterial());
  scene.add(boxes, tree);
  const self = {
    scene, mesh: boxes, walking: false, style: 'city', walkCamera: new THREE.PerspectiveCamera(),
    compiled: [],
    renderer: {
      compileAsync(root, camera, target) {
        self.compiled.push({ root, style: self.style, fog: !!target.fog });
        return Promise.resolve();
      },
    },
    precompile: MapScene.prototype.precompile,
  };
  return { self, boxes, tree };
}

describe('shaders compiled ahead', () => {
  // Verifies: REQ-PERF-012
  it('compiles walk mode\'s fog for everything, then the boxes in every other style', async () => {
    const { self, boxes, tree } = fake();
    const versions = [boxes.material.version, tree.material.version];
    await MapScene.prototype.warmUp.call(self);
    assert.deepEqual(self.compiled.map(c => [c.root === self.scene ? 'scene' : 'boxes', c.style, c.fog]), [
      ['scene', 'city', true],
      ['boxes', 'circuit', false],
      ['boxes', 'galaxy', false],
    ]);
    assert.equal(self.style, 'city', 'the style was left changed');
    assert.equal(self.scene.fog, null, 'the fog was left on');
    assert.ok(boxes.material.version > versions[0] && tree.material.version > versions[1],
      'the materials were not sent back to their current programs');
  });

  // Verifies: REQ-PERF-012
  it('warms nothing while walking, and stops when a later layout starts its own', async () => {
    const walking = fake();
    walking.self.walking = true;
    await MapScene.prototype.warmUp.call(walking.self);
    assert.equal(walking.self.compiled.length, 0);

    const { self } = fake();
    const first = MapScene.prototype.warmUp.call(self);
    const second = MapScene.prototype.warmUp.call(self);
    await Promise.all([first, second]);
    assert.equal(self.compiled.length, 3, 'two warm-ups ran side by side');
  });
});
