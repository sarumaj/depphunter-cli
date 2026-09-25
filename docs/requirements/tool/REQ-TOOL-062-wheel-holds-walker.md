---
id: REQ-TOOL-062
uuid: e39c235d-add7-4235-bb59-d6f1ea6e6e77
title: Walker held while the wheel is up
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
  - manual
---

## Statement

While the wheel is up, the walker **shall** be held: no movement, aim, fire,
tank use, wind use, drowning or bites **shall** advance, and the city behind the
wheel **shall** be blurred.

## Rationale

The wheel covers the view; changing hands should not burn a tank, drown anybody
or hand a bug a free bite.

## Acceptance criteria

1. A walker who opens the wheel mid-flight does not lose fuel while it is up.
2. The view behind the wheel is blurred.
