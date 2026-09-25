---
id: REQ-CLI-006
uuid: dd97a584-30dd-4afe-9f5e-6e157b19a3d7
title: Help with description and examples
scope: cli
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M9
verification:
  - unit
---

## Statement

The command **shall** print, for `--help` (and `-h`), a generated help text that
contains the usage line `depphunter [path]`, a description of what the command
does and where its settings come from, examples, and every flag with its
description and default.

## Rationale

Generated help stays in step with the flags declared by the configuration
package (REQ-CFG-005).

## Acceptance criteria

1. `depphunter --help` exits with status 0.
2. Its output contains `depphunter [path]`, an `Examples:` section and a flag
   such as `--history-commits`.
