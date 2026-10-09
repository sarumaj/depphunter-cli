---
id: REQ-EXT-038
title: Allowing an editor served from elsewhere
scope: ext
type: interface
priority: should
status: implemented
source:
  - README.md Settings
verification:
  - extension
---

## Statement

The extension **shall** offer `depphunter.allowHost`, a list of origins settable
for the machine only, and when it holds any **shall** pass each as
`--allow-host <origin>` after the fixed `--embed` pairs and **shall** omit
`--addr 127.0.0.1:0`. Blank entries **shall** be ignored.

## Rationale

The browser build of the editor frames the map from the page it is served from,
which the extension cannot know, and loads it from another machine than the
server's. A workspace's settings come with the repository, so they may not open
the server to the network.

## Acceptance criteria

1. `allowHost: ["https://example.com"]` yields `--allow-host
   https://example.com`, the three fixed `--embed` pairs and no `--addr`.
2. Left empty, the command line is that of
   [REQ-EXT-022](REQ-EXT-022-fixed-hosting-arguments.md).
