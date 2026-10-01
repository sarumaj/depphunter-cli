// The boxes' depth drawn ahead of the frame (MapScene.depthFirst).
//
// The drawing is WebGL's; checked here is what is drawn in the pass - the meshes on
// its layer, with a material that writes depth and no color - and that the camera and
// the scene are left as they were, with the renderer set not to clear that depth.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { MapScene } = await import('../static/map/scene.js');
const THREE = await import('../static/vendor/three.module.min.js');

function fake() {
  const scene = new THREE.Scene();
  const self = {
    scene, passes: [],
    bendable: m => m,
    renderer: {
      autoClear: true,
      render(s, camera) { self.passes.push({ layers: camera.layers.mask, material: s.overrideMaterial }); },
    },
  };
  return self;
}

describe('depth pre-pass', () => {
  // Verifies: REQ-PERF-013
  it('draws the boxes\' layer depth only, then leaves the depth for the frame', () => {
    const self = fake(), camera = new THREE.PerspectiveCamera();
    const mask = camera.layers.mask;
    MapScene.prototype.depthFirst.call(self, camera);
    assert.equal(self.passes.length, 1);
    const [pass] = self.passes;
    assert.equal(pass.layers, 1 << 1, 'the pass drew more than the boxes\' layer');
    assert.equal(pass.material.colorWrite, false);
    assert.equal(pass.material.depthWrite, true);
    assert.ok(pass.material.polygonOffsetFactor > 0, 'the pass is not pushed behind the frame\'s own drawing');
    assert.equal(camera.layers.mask, mask);
    assert.equal(self.scene.overrideMaterial, null);
    assert.equal(self.renderer.autoClear, false, 'the frame would clear the depth it was given');
  });
});
