---
id: REQ-CFG-009
uuid: aa7f6b14-c96d-4361-8400-6d2c0791a898
title: Exclude globs add up
scope: cfg
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The `exclude` globs from the config files, from `DEPPHUNTER_EXCLUDE`
(comma-separated) and from each `--exclude` flag **shall** be combined, in that
order, rather than replace one another.

## Rationale

Exclusions from every source apply at once; a flag that replaced the project's
exclusions would bring its generated files back.

## Acceptance criteria

1. `DEPPHUNTER_EXCLUDE=*.gen.go` and `--exclude testdata` yield the globs
   `*.gen.go` and `testdata`.
2. Globs from the config files come first.
