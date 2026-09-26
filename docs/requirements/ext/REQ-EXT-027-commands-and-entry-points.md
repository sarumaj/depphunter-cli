---
id: REQ-EXT-027
uuid: 8346b8f1-53dd-41ea-9e74-1e2dc0ee82e7
title: Commands and entry points
scope: ext
type: interface
priority: must
status: implemented
source:
  - README.md Use
verification:
  - extension
  - inspection
---

## Statement

The extension **shall** provide the commands Open the Map, Open the Map in the
Browser, Restart the Server, Stop the Server, Show the Server Log, Show the
Resolution Report, Export the Graph, Export the Backpack, Refresh the Side Panel
and Open Settings, and
**shall** offer Open the Map in the explorer's context menu for any folder.

## Rationale

The same actions are reachable from the Maps view, the command palette and the
explorer; a subdirectory is a fair thing to map.

## Acceptance criteria

1. Each listed command is registered under `depphunter.*` and titled
   `depphunter: …`.
2. Open the Map on a folder inside a workspace folder maps that folder.
3. From the palette with several workspace folders, the user is asked which one.
