---
id: REQ-SRV-008
uuid: 2de0e32c-6409-4626-a169-adcfca74550e
title: Fallback to the vscode URL handler
scope: srv
type: functional
priority: must
status: implemented
verification:
  - manual
---

## Statement

When the server reports that no editor is configured, the UI **shall** open the
file through the `vscode://file/<absolute path>:<line>` URL handler instead of
calling `POST /api/open`.

## Rationale

A machine with VS Code installed can still jump to the code when no command
template could be detected.

## Acceptance criteria

1. With `editor: false` in `/api/config`, choosing to open a file navigates to
   `vscode://file/<root>/<path>:<line>`.
