---
id: REQ-CLI-004
title: Short form -o of --output
scope: cli
type: interface
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The command **shall** accept `-o` as the short form of `--output`.

## Rationale

`-o` is the conventional short name of an output file.

## Acceptance criteria

1. `--export dot -o g.dot` writes the export to `g.dot`.
2. `-o` without `--export` is rejected with an error.
