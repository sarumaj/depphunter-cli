---
id: REQ-CITY-024
uuid: 3cb1e874-f32b-4f16-96d4-e569159de4b5
title: Park planting sampled off paths
scope: city
type: non-functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M10
verification:
  - manual
  - inspection
---

## Statement

Trees and bushes in parks **shall** be planted only where the street shader
draws lawn, off the gravel paths, on a jittered grid of at most 60 000 samples
per map.

## Rationale

Plants must not stand on paths or streets; the sample cap bounds the cost on
huge maps.

## Acceptance criteria

1. No tree or bush stands on a park path or a street.
2. A very large map plants no more than 60 000 samples.
