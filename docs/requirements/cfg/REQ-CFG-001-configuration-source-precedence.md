---
id: REQ-CFG-001
uuid: c1283457-7b45-4120-99aa-691b7ef09ea6
title: Precedence of configuration sources
scope: cfg
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** resolve every setting from, in increasing precedence:
built-in defaults, `--ui-default` seeds (REQ-CFG-016), the user config file, the
project config file, `DEPPHUNTER_*` environment variables, and command-line
flags; a value from a source of higher precedence **shall** override the value
from every source of lower precedence.

## Rationale

The user's standing preferences are refined per repository, then per shell, then
per run. The precedence is resolved with viper.

## Acceptance criteria

1. A setting given in the user file only takes the user file's value.
2. A setting given in the user and project files takes the project file's value.
3. A setting given in a file and in the environment takes the environment's
   value.
4. A setting given in the environment and as a flag takes the flag's value.
5. A flag that is registered but not given does not override a file value.
6. The configuration tests (`go test ./internal/config`) check these cases.

## Notes

Exceptions: list settings that add up across sources (REQ-CFG-009), and keys a
project file may not set (REQ-CFG-010).
