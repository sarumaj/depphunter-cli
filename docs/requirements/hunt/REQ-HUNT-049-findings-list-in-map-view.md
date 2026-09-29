---
id: REQ-HUNT-049
title: The map view lists every finding it shows
scope: hunt
type: functional
priority: must
status: implemented
source:
  - README.md Findings
verification:
  - ui
  - manual
---

## Statement

In the map view, the UI **shall** offer a list of every finding placed on a
node the map shows under the current filters, opened from the toolbar's
**Findings** button and with `L`. The list **shall** be ordered by severity,
most severe first, and then by the name of the package or file the finding is
against. Each row **shall** show that package (with its version) or file, the
severity, the finding's identifier and title, and where it is (the ecosystem
and the file it was read from, or the line). Pointing at a row or focusing it
**shall** highlight the building it is placed on; picking a row **shall**
select that building and open the finding in the side panel.

## Rationale

From above, a finding is a pin, and reading one means finding its building
first. A repository with dozens of advisories against its lock files is dozens
of buildings to point at; the list is the same findings without the search.

## Acceptance criteria

1. The list holds the findings in the order stated, with a finding about the
   whole repository last among its severity.
2. Hiding a language, an island or a path takes the findings placed on what it
   hides out of the list.
3. Hovering or focusing a row highlights its building; picking it selects the
   building and opens the finding in the side panel.
4. The button and `L` do nothing in walk mode, where the findings are bugs.

## Notes

The rows lead to the node findings.js places each finding on (REQ-FND-020),
which is the node its bug stands at in walk mode.
