---
id: REQ-CFG-010
uuid: e5fe596f-fe0d-42e6-9377-7911ef081d19
title: Project config cannot set the editor
scope: cfg
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M3
  - docs/REQUIREMENTS.md M9
verification:
  - unit
---

## Statement

The system **shall** ignore the `editor` key of the project config file read
from the analyzed directory; the editor command **may** be set by the user
config file, `--config`, `DEPPHUNTER_EDITOR` or `--editor`.

## Rationale

A cloned repository could otherwise choose the command depphunter executes. The
key is dropped before the file is merged, rather than the editor being
overridden afterwards, so the environment and flags still win over the user's
file.

## Acceptance criteria

1. A project file with `editor: sh -c '...'` and a user file with `editor: code
   -g {file}:{line}` yield the user file's editor.
2. `DEPPHUNTER_EDITOR` and `--editor` still set the editor.

## Notes

The same mechanism drops `online` and `trust_indexes` and confines `findings`
paths (scopes `sup` and `fnd`).
