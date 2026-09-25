---
id: REQ-TOOL-008
uuid: e6588af7-dcac-41ae-9e47-3bef93d864ed
title: Hand model prepared from a pinned upstream hand
scope: tool
type: constraint
priority: must
status: implemented
verification:
  - inspection
---

## Statement

`tools/hand.py` **shall** prepare the walker's hand from the MIT-licensed
`generic-hand` model of the WebXR Input Profiles project, fetched from npm and
pinned by version and checksum; it **shall** add the forearm the model lacks,
rebuild its rig as a skeleton with one bone per joint named after the WebXR
joints, and export `web/static/hand.glb`. The UI **shall not** download a model
at run time.

## Rationale

A hand modelled and rigged by people who model hands is better than any hand a
script can grow, and an earlier version of the script, which grew one from a
joint skeleton with the Skin modifier, showed as much. What the script owns is
what the upstream model does not provide: the axes and scale of the viewmodel,
a forearm, and a rig whose fingers carry their tips along when they curl. The
upstream license is permissive and compatible with this repository's (see
REQ-DIST-005).

## Acceptance criteria

1. `tools/hand.py` verifies the downloaded package against a pinned checksum
   before using it.
2. The exported rig has one bone per phalanx, named after the WebXR joints.
3. The model's license is listed in `web/static/vendor/README.md`.
4. Loading the map makes no request for a hand model outside the embedded
   assets.

## Notes

The script's module docstring records the decision. The bone names remain the
contract with `hands.js` (REQ-TOOL-009).
