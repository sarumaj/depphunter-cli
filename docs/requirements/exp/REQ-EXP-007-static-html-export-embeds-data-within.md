---
id: REQ-EXP-007
uuid: 72a3c072-966d-4d78-8edb-9df7a1870eda
title: Static HTML export embeds data within limits
scope: exp
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The static HTML export **shall** embed the graph, the view settings and the
source text of the graph files, skipping binary files and files over 256 KB and
stopping at 24 MB of source in total, in a JSON script element that embedded
content cannot close.

## Rationale

The side panel of a shared map should show source; the limits keep the file
shareable.

## Acceptance criteria

1. A file of 256 KB + 1 byte and a binary file are not embedded; a small text
   file is.
2. A source containing `</script>` does not close the data element.
