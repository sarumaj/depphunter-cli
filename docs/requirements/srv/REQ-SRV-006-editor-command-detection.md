---
id: REQ-SRV-006
uuid: ec66b25a-35b0-4626-ae92-672c42c5052b
title: Editor command detection
scope: srv
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

When no editor template is configured, the system **shall** detect one:
`$VISUAL` or `$EDITOR` when it names a known GUI editor, otherwise the first
known GUI editor found on `PATH`; terminal editors **shall not** be detected.

## Rationale

Most users never configure an editor; a detected one makes the feature work out
of the box. A terminal editor launched by the server would compete with it for
the terminal.

## Acceptance criteria

1. `VISUAL=/usr/local/bin/zed` yields `/usr/local/bin/zed {file}:{line}`, even
   with `code` on `PATH`.
2. `EDITOR=nvim` with `subl` on `PATH` yields `subl {file}:{line}`.
3. With no known editor in the environment or on `PATH` no template is detected,
   and `POST /api/open` answers 501.
