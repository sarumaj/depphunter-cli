// body.glb, from scripts/body.py: the walker's body in each style's outfit (body.js),
// and whole as `hand_rig` the hand held before the eye (hands.js) - the same hand the
// body's arms end in.
//
// Implements: REQ-TOOL-010

import { GLTFLoader } from '../vendor/GLTFLoader.js';
import { STATIC } from '../core/data.js';

let loading = null;

/**
 * Starts loading the model, once; resolves with its scene, or null if it will not
 * load. A static export has no server to fetch from and carries the model inline.
 */
export function loadBody() {
  loading ||= new GLTFLoader().loadAsync(STATIC?.body || 'body.glb').then(gltf => {
    gltf.scene.updateMatrixWorld(true);
    return gltf.scene;
  }).catch(err => {
    console.error('body model:', err);
    return null;
  });
  return loading;
}
