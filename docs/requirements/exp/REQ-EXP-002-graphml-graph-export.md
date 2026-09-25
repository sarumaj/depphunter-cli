---
id: REQ-EXP-002
uuid: eea110bb-016f-4c7e-822b-e23d60f5e69f
title: GraphML graph export
scope: exp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** export the graph as well-formed GraphML with every node and
edge and their attributes (kind, name, path, parent, language, lines, symbol
kind, line, version, requested, floating, transitive, index, index-unknown,
private, standard library, unresolved; edge kind and line), XML-escaped.

## Rationale

GraphML opens in Gephi, yEd and NetworkX for analyses the map does not offer.

## Acceptance criteria

1. A GraphML export parses as XML and holds all nodes and edges.
2. A file node carries its `loc`; a name with `<` and `>` is escaped.
