// The models the map is drawn from.
//
// Two files, both prepared in Blender by the scripts in tools/ and both read the
// same way: props.glb holds the plants (a trunk and a crown per tree, a bush and a
// stone), bug.glb the beetle a finding walks the streets as. What comes out here is
// geometry standing where it will be drawn and nothing else - how it is shaded,
// tinted, instanced and placed stays with the code that draws it, because that is
// the map's business and not the model's.
//
// The map draws without either of them - city.js keeps the blobs it always had, and
// bugs.js the beetle it drew out of spheres - so nothing waits on these loads.

import { GLTFLoader } from './vendor/GLTFLoader.js';
import { STATIC } from './data.js';

const files = new Map(); // key -> { parts, loading }

/**
 * Starts a load, and resolves with its parts once they are in. Calling it again
 * while it is loading joins the same load; a load that fails resolves null and
 * leaves whatever asked for it on its stand-ins.
 *
 * `key` names the model both here and in a static export, which has no server to
 * fetch from and carries the file inline instead.
 */
function load(key, file) {
  let at = files.get(key);
  if (!at) files.set(key, at = { parts: null, loading: null });
  at.loading ||= new GLTFLoader().loadAsync(STATIC?.[key] || file).then(gltf => {
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
    at.parts = found.size ? found : null;
    return at.parts;
  }).catch(err => {
    console.error(`${file}:`, err);
    return null;
  });
  return at.loading;
}

const got = key => files.get(key)?.parts || null;

/** The plants: trunks, crowns, a bush and a stone (city.js). */
export const loadPlants = () => load('props', 'props.glb');
export const plants = () => got('props');

/** The beetle: its wing cases, its dark front end and its legs (bugs.js). */
export const loadBugs = () => load('bug', 'bug.glb');
export const bugParts = () => got('bug');
