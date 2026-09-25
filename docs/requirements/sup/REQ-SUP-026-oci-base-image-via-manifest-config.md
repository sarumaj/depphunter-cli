---
id: REQ-SUP-026
uuid: dd9267d0-0656-41f0-b51e-97e10d9e0862
title: Base image from the manifest and config blob
scope: sup
type: functional
priority: must
status: partial
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The index client **shall** resolve the base image of a container image from the
base-image annotations of its manifest or the labels of its config blob,
following a multi-platform index to one manifest, and **shall not** fetch any
layer.

## Rationale

An image has no dependency list; its base image is what it inherits
vulnerabilities from.

## Acceptance criteria

1. The base image named by a config label is returned.
2. The base image named by a manifest annotation is returned, with its digest in
   preference to its tag.
3. No request addresses a layer blob.

## Notes

Partial: only images on Docker Hub are asked. An image on any other registry is
reported as from an index only the repository names and is never asked, even
with `--trust-index` or a registry credential on this machine.
