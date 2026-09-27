---
id: REQ-UI-016
title: The panel's actions are pinned in its corner
scope: ui
type: functional
priority: should
status: implemented
verification:
  - unit
  - manual
---

## Statement

The details panel's opening of a file in the editor **shall** be a button in the
panel's top corner, beside maximize and close, that stays in view however far
the panel is scrolled. It **shall** be labelled briefly with the editor ("VS Code
↗") and name the full action in its title, and **shall** be absent for anything
that is not a file and where no editor can be opened. What shares the top of the
panel - the breadcrumbs, and the source heading with its search pinned there
while a file scrolls - **shall** run up to the corner's buttons, however wide
they are, without passing under them.

## Rationale

Reading a long file scrolls the button that opens it out of sight, just when it
is wanted; and a search a fixed distance from buttons of changing width is
either cramped or stranded.

## Acceptance criteria

1. With a file shown, "VS Code ↗" is in the corner and stays there while the
   panel scrolls; pressing it opens the file.
2. A directory, a package or a static export shows no open button.
3. The source's search ends just short of the corner's buttons.
