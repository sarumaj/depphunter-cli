---
id: REQ-EXT-037
title: A package's page, repository and folder from the editor
scope: ext
type: functional
priority: should
status: implemented
source:
  - README.md The panel beside the code
verification:
  - extension
---

## Statement

A package's row in the Dependencies view **shall** offer, inline and in its
context menu, **Open the Package's Page** and **Open the Repository** where the
package has a `page` or a `repository` (REQ-MOD-014), opening it in the
system's browser, and **Reveal the Installed Folder**, which asks the server
where the package is installed (REQ-SRV-018). A folder inside the workspace
**shall** be revealed in the Explorer; one outside it **shall** be offered in a
new window or in the system's file manager; none found **shall** be said. The
extension's event stream **shall** ask for links (`?opens=hex,links`) and
**shall** open what a `browse` event hands it in the same way. Only `http` and
`https` addresses **shall** be opened.

## Rationale

The map's frame can open neither a window nor a folder, and the tree is where
the editor's user already is; both lead to the same three places.

## Acceptance criteria

1. A package row's context value is `package`, followed by `page` and
   `repository` when it has them, and the actions follow it.
2. A folder under a workspace folder is revealed with `revealInExplorer`.
3. A folder outside every workspace folder prompts with two choices and opens
   the one chosen; a dismissed prompt opens nothing.
4. `javascript:` and `file:` addresses are not opened.

## Notes

The folder is not offered as a new workspace folder: turning a window of one
folder into a workspace of two restarts the extensions, and every map with them.
