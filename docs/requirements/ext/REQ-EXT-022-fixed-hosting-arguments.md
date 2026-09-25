---
id: REQ-EXT-022
uuid: f6828320-8008-462b-a02b-0dd92a5eb0dc
title: Fixed hosting arguments
scope: ext
type: interface
priority: must
status: implemented
source:
  - README.md Settings
  - README.md How the framing works
verification:
  - extension
---

## Statement

The extension **shall** start every server with `--no-open`, `--addr
127.0.0.1:0` and `--embed` for each of `vscode-webview:`, `vscode-file:` and
`https://*.vscode-cdn.net`, and **shall not** offer a setting that changes them.

## Rationale

The extension opens the map itself; the operating system picks a free port;
`frame-ancestors` is checked against every frame above the page, so all three
editor origins have to be named or the page is refused.

## Acceptance criteria

1. The command line begins `--no-open --addr 127.0.0.1:0` followed by the three
   `--embed` pairs.
2. The served page's `Content-Security-Policy` names all three origins in
   `frame-ancestors`.
