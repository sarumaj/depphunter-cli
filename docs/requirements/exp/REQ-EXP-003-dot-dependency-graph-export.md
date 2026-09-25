---
id: REQ-EXP-003
uuid: 8e4bb14b-9bc5-43ef-8928-c1630ca96727
title: DOT dependency graph export
scope: exp
type: functional
priority: must
status: implemented
verification:
  - unit
  - manual
---

## Statement

The system **shall** export the import dependencies between files, import-target
directories and external packages as Graphviz DOT (via emicklei/dot), with one
flat cluster per directory and per ecosystem, omitting standard-library
packages, files without edges and symbol reference edges.

## Rationale

A drawing of every node is unreadable; nested clusters were tried and make
Graphviz stack them into very tall layouts. JSON and GraphML keep everything.

## Acceptance criteria

1. The DOT export of the sample graph holds exactly the edges `a.go ->
   x.io/<z>`, `main.go -> pkg/` and `main.go -> x.io/y v1.0.0`.
2. Standard-library packages and edge-less files are absent.
3. The DOT export of this repository renders in Graphviz (`dot -Tsvg`).
