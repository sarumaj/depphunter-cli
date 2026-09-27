---
id: REQ-SUP-028
uuid: e44c9d80-57e8-49aa-852d-27e174d3d899
title: Maven index cannot be asked
scope: sup
type: limitation
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall not** ask a Maven repository what a package named
without an artifact depends on, and **shall** record the question as
unanswerable for this ecosystem. A package named `group:artifact` is asked
(REQ-SUP-056).

## Rationale

A POM is addressed by group and artifact. Every plugin names Maven packages
`group:artifact`; what can still lack an artifact is a Bazel hub target that no
artifact list or lock file names (`maven:<escaped_name>`, REQ-BAZEL-011), and
guessing an artifact for it is worse than saying nothing.

## Acceptance criteria

1. With `--online`, a Maven package named without an artifact produces no
   request and the report says the ecosystem's index cannot be asked.

## Notes

Until the Java, Kotlin and Scala plugins named Maven packages by artifact
(REQ-JAVA-012), this applied to all of their packages.
