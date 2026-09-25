---
id: REQ-CI-010
uuid: a8bbf970-e2e7-4dbd-b4a7-07fcd33cd668
title: CI jobs become symbols
scope: ci
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The CI plugin **shall** report the jobs of a GitHub workflow and of a GitLab
pipeline as the file's symbols of kind `job`, and for GitLab **shall** treat
every top-level mapping key other than the reserved pipeline keys (`image`,
`services`, `stages`, `types`, `before_script`, `after_script`, `variables`,
`cache`, `include`, `default`, `workflow`) as a job, including hidden
`.template` jobs.

## Rationale

Jobs are the units a pipeline file is made of, the way functions are for a
source file.

## Acceptance criteria

1. A workflow with jobs `test`, `release` and `build` has exactly those three
   symbols.
2. The GitLab test pipeline has exactly the symbols `unit` and
   `.hidden-template`.
