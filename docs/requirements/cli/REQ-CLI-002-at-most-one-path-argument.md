---
id: REQ-CLI-002
uuid: 718d6ea5-25ac-44ec-84cb-e8c6f9959c85
title: At most one path argument
scope: cli
type: interface
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The command **shall** accept at most one positional argument and **shall** fail
with an error when given more than one.

## Rationale

One run analyzes one directory; a second path would be silently ignored
otherwise.

## Acceptance criteria

1. `depphunter a b` exits with an error and does not analyze anything.
2. `depphunter` and `depphunter <dir>` are accepted.
