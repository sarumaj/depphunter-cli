---
id: REQ-UI-005
uuid: f1863dd7-db1c-4941-98da-5d50d2a3d687
title: Introduction remembered per browser
scope: ui
type: functional
priority: must
status: implemented
verification:
  - ui
---

## Statement

The UI **shall** remember in browser storage that the introduction was seen,
independently of the repository, and **shall not** open it again on a later
visit unless asked to from the help.

## Rationale

What it explains is the tool, not the project.

## Acceptance criteria

1. A second load in the same browser, of any repository, does not open the
   introduction.
2. The help's button still opens it.
