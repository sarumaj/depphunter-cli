---
id: REQ-TOOL-008
uuid: e6588af7-dcac-41ae-9e47-3bef93d864ed
title: Hand grown from a skeleton, nothing downloaded
scope: tool
type: constraint
priority: must
status: not-implemented
source:
  - docs/REQUIREMENTS.md M14
verification:
  - inspection
---

## Statement

`tools/hand.py` **shall** build the hand itself from a skeleton of joints with a
radius each, grown into flesh by Blender's Skin modifier and smoothed, and
**shall not** download a model.

## Rationale

A rigged hand that can be posed per tool and redistributed under this
repository's license was considered not available to fetch.

## Acceptance criteria

1. `tools/hand.py` uses the Skin modifier on a joint skeleton.
2. `tools/hand.py` makes no network request.

## Notes

Not implemented: `tools/hand.py` downloads the MIT-licensed `generic-hand` model
of the WebXR Input Profiles project from npm (pinned by version and checksum),
turns and scales it, adds a forearm and rebuilds its rig. The design log's M14
text no longer matches the script.
