---
id: REQ-FND-024
uuid: ab4c765d-525c-4a2d-ad36-1a353dcfdb25
title: Side panel lists findings worst first
scope: fnd
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M13
verification:
  - unit
  - manual
---

## Statement

The side panel **shall** list the findings of the selected node worst first,
each row showing its title, reference and location, and offer the findings below
it behind a button.

## Rationale

The reader looks at the most serious finding first.

## Acceptance criteria

1. The findings set is ordered by severity, most serious first.
2. A directory lists its own findings and a button "Show the N below this one".
