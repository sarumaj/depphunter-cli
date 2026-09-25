---
id: REQ-EXP-010
uuid: e64946e4-0fb6-48e1-bd4d-4c24ca7a9d9f
title: Command-line HTML export uses the configured view
scope: exp
type: functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

`--export html` **shall** embed the view settings resolved from the
configuration (flags, environment, config files, defaults).

## Rationale

The command line has no screen whose view could be carried.

## Acceptance criteria

1. `--export html --theme dark` produces a page whose embedded config has theme
   `dark`.
