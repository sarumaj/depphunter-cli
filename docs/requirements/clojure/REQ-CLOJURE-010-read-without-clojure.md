---
id: REQ-CLOJURE-010
title: Clojure read without running it
scope: clojure
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Manifests and sources **shall** be read without running Clojure, Leiningen,
tools.deps or babashka: computed values (`~` in `project.clj`, `#=`
read-eval), `:deps/prep-lib`, Leiningen profiles in `profiles.clj` and
middleware, and namespaces loaded dynamically (`requiring-resolve`, a
`require` inside a function) are not followed. There is no lock file: a
dependency's own dependencies are known only with `--online`, from its POM
(REQ-SUP-056). Namespaces and classes are attributed to artifacts by
tables and naming heuristics, which can name a sibling artifact of the
right group; classes of transitive dependencies are dropped.

## Rationale

The map is built from files alone.

## Acceptance criteria

1. In metabase, `org.quartz` classes (from quartzite's transitive
   `org.quartz-scheduler/quartz`) are dropped.
