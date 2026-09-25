---
id: REQ-EXT-024
uuid: d9a2c71e-448a-4931-bbfa-be487dd3fda2
title: Settings passed as command-line flags
scope: ext
type: interface
priority: must
status: implemented
source:
  - README.md Settings
verification:
  - extension
---

## Statement

The extension **shall** pass each of its analysis, findings, references and
general settings as the flag the README names only when it differs from
depphunter's own default, **shall** append `depphunter.args` after them, and
**shall** pass the folder last.

## Rationale

Flags override the project's `.depphunter.yaml`; passing only what the user
changed lets that file govern everything else.

## Acceptance criteria

1. Settings left at their defaults produce no flag other than `--watch`.
2. Every setting set away from its default produces its flag, with numbers
   truncated to integers and `0` passed as a value.
3. Blank list entries are skipped.
4. `depphunter.args` follow all other flags and the folder is the last argument.
