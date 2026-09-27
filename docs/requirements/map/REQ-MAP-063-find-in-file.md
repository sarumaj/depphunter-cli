---
id: REQ-MAP-063
title: Find in a file's source
scope: map
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

The source shown in the details panel **shall** have a search field beside its
heading that finds the typed text in the file - literally and ignoring case -
highlights every match without changing the text or its coloring, marks the
lines that hold them, says which match is current and how many there are, and
steps through them with `Enter`, `Shift+Enter` and two buttons, scrolling the
current one into view and wrapping at either end. `Escape` in the field
**shall** clear it without closing the panel. The heading and the field
**shall** stay in view while the source scrolls, and what is being looked for
**shall** be found again when a live update redraws the same file.

## Rationale

A file of any length is otherwise read by scrolling, and the browser's own find
also searches everything else on the page.

## Acceptance criteria

1. Every occurrence is found and counted, including several on one line, and
   `a.b(` finds those four characters.
2. `Enter` and `Shift+Enter` step forwards and back, wrapping round.
3. A search with no matches says so and leaves no line marked.
4. The first `Escape` clears the search and the panel stays open.
