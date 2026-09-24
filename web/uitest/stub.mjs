// Enough of a browser to import the map's modules outside one.
//
// The UI is plain ES modules with no build step, which is what makes this possible at
// all: the same files the server embeds are the files these tests import. Three
// globals are touched while the modules are being evaluated - data.js reads the page
// for an embedded export and the address for a session token, and three.js looks for
// a canvas - and nothing here needs any of them to do anything.
//
// It lives beside web/static rather than in it because web/embed.go embeds that whole
// directory, and a test has no business inside the binary.

globalThis.document ??= {
  getElementById: () => null,
  createElement: () => ({ style: {}, getContext: () => null }),
  createElementNS: () => ({ style: {}, getContext: () => null }),
};
globalThis.window ??= { devicePixelRatio: 1, addEventListener() {} };
globalThis.location ??= { search: '' };
// The palettes are read off the page's own custom properties (colors.js,
// findings.js). Nothing here is about the colors, so every one of them is empty and
// the modules fall back to what they were written with.
globalThis.document.documentElement ??= {};
globalThis.getComputedStyle ??= () => ({ getPropertyValue: () => '' });

// Boxes are named so that a node can be made for each without the tests spelling one
// out; the real layout always carries one, and city.js reads it to find which terrace
// a box stands on.
let nth = 0;

/**
 * A box of the layout, with the defaults the tests do not care about filled in. `node`
 * may be given to stand a box on a terrace: pass the terrace's node as its parentNode.
 */
export function box(kind, x, z, w, d, opts = {}) {
  const node = { id: `n${nth++}`, kind: kind === 'building' ? 'file' : 'dir', parentNode: null };
  return { kind, x, z, w, d, y: 0, h: 0.2, node, ...opts };
}

/**
 * What Routes needs of MapScene: somewhere to hang its meshes, a material pass-through
 * (the real one patches materials to bend around the planet) and a no-op redraw.
 */
export function scene() {
  const added = [];
  return {
    scene: { add: o => added.push(o), remove: o => added.splice(added.indexOf(o), 1) },
    bendable: m => m,
    requestRender() {},
    added,
  };
}

/** Every vertex of every mesh a Routes laid down, as {x, y, z}. */
export function vertices(routes) {
  const out = [];
  for (const mesh of routes.group.children) {
    const p = mesh.geometry.getAttribute('position');
    for (let i = 0; i < p.count; i++) out.push({ x: p.getX(i), y: p.getY(i), z: p.getZ(i) });
  }
  return out;
}
