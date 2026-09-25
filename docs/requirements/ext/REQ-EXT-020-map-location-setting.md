---
id: REQ-EXT-020
uuid: 71a7cdb2-848a-4c80-83d5-9ed8522faaa1
title: Where the map opens
scope: ext
type: functional
priority: must
status: implemented
source:
  - README.md Settings
  - README.md Why the map is in a tab of its own
verification:
  - extension
---

## Statement

The extension **shall** show the map where `depphunter.openIn` says: `webview`
(default) in a dedicated editor tab, `simpleBrowser` in the editor's built-in
browser, `externalBrowser` in the system's default browser; where the built-in
browser is not available it **shall** fall back to the dedicated tab.

## Rationale

The built-in browser withholds the pointer lock walk mode needs, so a tab of the
extension's own is the default; the others remain choices.

## Acceptance criteria

1. With `simpleBrowser`, `simpleBrowser.show` is called with the map address.
2. With `webview`, a webview panel holding the map is opened.
3. With `externalBrowser`, the address is opened externally.
4. Changing only `depphunter.openIn` does not offer a restart.
