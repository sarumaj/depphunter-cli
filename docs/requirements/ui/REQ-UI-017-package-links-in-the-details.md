---
id: REQ-UI-017
title: A package's links in its details
scope: ui
type: functional
priority: should
status: implemented
verification:
  - ui
  - manual
---

## Statement

The details of a package **shall** link to its `page`, labeled with the page's
site ("npmjs.com ↗"), and to its `repository` ("Repository ↗"), each opening
in a new window; and, once `GET /api/locate` (REQ-SRV-018) answers, **shall**
offer "Folder ↗", which shows that folder (REQ-SRV-019). A page framed by
another - the editor's tab - **shall** ask the server to open a link
(`POST /api/browse`) rather than open it itself, and **shall** say on the
status line when nothing could. A package with neither link and no folder
**shall** show none of this, and a static export **shall** offer no folder.

## Rationale

From a package on the map, the next questions are what it is, whose it is and
what it runs, and the answers are on its index's page, in its repository and
in the folder it is installed in.

## Acceptance criteria

1. An npm package with a page and a repository shows "npmjs.com ↗" and
   "Repository ↗", links with `target="_blank"` and `rel="noopener
   noreferrer"`.
2. A folder found for the package adds "Folder ↗"; one found after another
   package was shown is not added to it.
3. In the extension's map tab, choosing a link opens it in the system's
   browser.
