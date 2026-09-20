// The models the map's plants are drawn from.
//
// props.glb is prepared by tools/props.py out of a CC0 low poly nature pack: a trunk
// and a crown per tree, a bush and a stone, standing on the origin at the size they
// are planted. city.js shades and instances them; everything about how they are
// colored, tinted and placed stays there, because that is the map's business and
// not the model's.
//
// The map draws without them - city.js keeps the blobs it always had as a stand-in -
// so nothing waits on this load.

import * as THREE from './vendor/three.module.min.js';
import { GLTFLoader } from './vendor/GLTFLoader.js';
import { STATIC } from './data.js';

let parts = null;    // name -> geometry, once loaded
let loading = null;  // the load in flight

/**
 * Starts loading the models, and resolves with them once they are in. Calling it
 * again while it is loading joins the same load; a load that fails resolves null and
 * leaves the map on its stand-ins.
 */
export function loadPlants() {
  // A static export has no server to fetch from and carries the models inline.
  loading ||= new GLTFLoader().loadAsync(STATIC?.props || 'props.glb').then(gltf => {
    const found = new Map();
    gltf.scene.updateMatrixWorld(true);
    gltf.scene.traverse(o => {
      if (!o.isMesh) return;
      // Whatever transform the file put on the node is baked in here, so what comes
      // out is one geometry standing where it will be drawn.
      const geo = o.geometry.clone().applyMatrix4(o.matrixWorld);
      for (const name of Object.keys(geo.attributes)) {
        if (name !== 'position') geo.deleteAttribute(name);
      }
      found.set(o.name, geo);
    });
    parts = found.size ? found : null;
    return parts;
  }).catch(err => {
    console.error('prop models:', err);
    return null;
  });
  return loading;
}

/** The loaded models, or null while they are not in yet. */
export const plants = () => parts;
