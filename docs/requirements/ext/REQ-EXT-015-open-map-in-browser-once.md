---
id: REQ-EXT-015
uuid: b036bf9c-1250-42c0-b601-e6f037e86534
title: Map opened in the browser once
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md Use
verification:
  - extension
---

## Statement

The command **depphunter: Open the Map in the Browser** **shall** open the map
of a folder in the system's default browser, starting its server if needed,
without changing `depphunter.openIn`.

## Rationale

The setting is where the map opens by default; this is for the one time it is
wanted with more screen, a second monitor or a browser's developer tools.

## Acceptance criteria

1. The command opens `http://127.0.0.1:<port>/?token=…` externally.
2. `depphunter.openIn` is unchanged afterwards, and the next Open the Map uses
   it.
