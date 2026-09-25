---
id: REQ-SRV-005
uuid: 1a28bd6f-05ec-407e-83e3-8b9a3cf4efc8
title: Open a file in the configured editor
scope: srv
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M3
verification:
  - integration
---

## Statement

When an editor command template is configured or detected, the server **shall**
open the file named in a `POST /api/open` request (`{"path", "line"}`) at the
given line by starting the command, and answer 204 once the command has started.

## Rationale

Reading the code in the side panel is often not enough; the user wants to
continue in their own editor at the exact place.

## Acceptance criteria

1. With the template `/bin/sh -c "echo {file}:{line} > marker"` a request for
   `a.go` line 3 writes `<root>/a.go:3` to the marker.
2. Without a template the endpoint answers 501 and names `--editor` and
   `DEPPHUNTER_EDITOR`.

## Notes

The cookie, request-header and graph-membership checks of `POST /api/open` and
the rule that the project configuration may not set `editor` belong to scope
`sec`.
