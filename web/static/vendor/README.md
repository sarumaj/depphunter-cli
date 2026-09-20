# Vendored libraries

Served from the embedded file system so the UI works offline and needs no JS
toolchain.

| File | Source | License |
|---|---|---|
| `three.module.min.js` | three@0.170.0 `build/three.module.min.js` | MIT (`three.LICENSE`) |
| `OrbitControls.js` | three@0.170.0 `examples/jsm/controls/OrbitControls.js` | MIT (`three.LICENSE`) |
| `highlight.min.js` | @highlightjs/cdn-assets@11.10.0 `es/highlight.min.js` | BSD-3-Clause (`highlight.LICENSE`) |
| `highlight-powershell.min.js` | @highlightjs/cdn-assets@11.10.0 `es/languages/powershell.min.js` (not in the common bundle) | BSD-3-Clause (`highlight.LICENSE`) |
| `potpack.js` | potpack@2.1.0 `index.js` (rectangle packing for terraces) | ISC (`potpack.LICENSE`) |
| `fzf.es.js` | fzf@0.5.2 `dist/fzf.es.js` (fuzzy search, fzf's algorithm) | BSD-3-Clause (`fzf.LICENSE`) |
| `GLTFLoader.js` | three@0.170.0 `examples/jsm/loaders/GLTFLoader.js` (reads `hand.glb`) | MIT (`three.LICENSE`) |
| `BufferGeometryUtils.js` | three@0.170.0 `examples/jsm/utils/BufferGeometryUtils.js` (GLTFLoader needs it) | MIT (`three.LICENSE`) |
| `SkeletonUtils.js` | three@0.170.0 `examples/jsm/utils/SkeletonUtils.js` (cloning a rigged model) | MIT (`three.LICENSE`) |

Local modification: `OrbitControls.js`, `GLTFLoader.js`,
`BufferGeometryUtils.js` and `SkeletonUtils.js` import `./three.module.min.js`
instead of the bare specifier `three` (and each other by file name), because the
Content-Security-Policy forbids the inline import map a bare specifier would
need.

`../hand.glb` is built by `tools/hand.py` from the `generic-hand` model in
`@webxr-input-profiles/assets@1.0.20`
(`dist/profiles/generic-hand/right.glb`), which is MIT licensed - Copyright (c)
2019 Amazon, `webxr-input-profiles.LICENSE`. The script re-orients and rescales
the model, adds the forearm it has no use for and rebuilds its rig; the hand
itself is theirs.
