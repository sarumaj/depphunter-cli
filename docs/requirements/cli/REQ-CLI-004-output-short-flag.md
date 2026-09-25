---
id: REQ-CLI-004
uuid: 8f817aa1-7023-4d45-bf8a-99bc6dd44391
title: Short form -o of --output
scope: cli
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M9
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
