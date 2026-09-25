---
id: REQ-EXT-023
uuid: 86642645-d813-49b7-919c-88ce100484ee
title: Map address taken from the server output
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md How the framing works
  - README.md Remote workspaces
  - README.md VS Code extension
verification:
  - extension
---

## Statement

The extension **shall** take the map address, including its session token, from
the server's `serving at <url>` output line, and **shall** preserve the token
when the address is rewritten for a remote or forwarded host.

## Rationale

In embed mode the token stays in the address, since a cookie would be a
third-party cookie in the frame; the printed address is the only place it
appears. Over SSH, WSL and dev containers the port is forwarded and the rewrite
may drop the query.

## Acceptance criteria

1. The address shown matches `http://127.0.0.1:<port>/?token=<48 hex>`.
2. When the forwarding rewrite drops the query, the address shown still carries
   the token and answers 200.
