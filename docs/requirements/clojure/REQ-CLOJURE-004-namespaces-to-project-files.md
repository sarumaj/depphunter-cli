---
id: REQ-CLOJURE-004
title: Namespaces to project files
scope: clojure
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A required namespace **shall** resolve to the project file defining it: first
by Clojure's rule (`a.b-c` is `a/b_c` with the importer's platform's
extensions: `.clj` then `.cljc` for Clojure, `.cljs`, `.cljc` then `.clj`
for ClojureScript and its macros, `.cljc`, `.clj`, `.cljs` for `.cljc`,
`.bb` first for babashka) under the source paths of the projects governing
the importer, else to any file whose `ns` form declares it (the
importer's platform first, governed files first). Source paths are
`deps.edn`'s `:paths` (default `src`) and its aliases' `:extra-paths`,
`project.clj`'s `:source-paths` and `:test-paths` (defaults `src`, `test`,
and profiles'), `shadow-cljs.edn`'s `:source-paths`, `bb.edn`'s `:paths`
and `build.boot`'s `:source-paths`/`:resource-paths`. The projects
governing a file are the manifest directories above it, nearest first, and
the projects their `:local/root` dependencies name; a file under none is
governed by every project. A namespace a file requires of itself is no
edge, except a ClojureScript file's macros namespace in a `.clj` beside
it; a namespace under the project's own root namespace that no file
defines, and no table knows, **shall** be dropped.

## Rationale

Clojure loads namespaces from the classpath by this rule; monorepos
(`:local/root`, Leiningen subprojects) and test fixtures make the governing
projects matter.

## Acceptance criteria

1. `shop.db-util` is `src/shop/db_util.clj`, `acme.shared.util` is the
   `:local/root` project's `libs/shared/src/acme/shared/util.clj`, and the
   shadow-cljs build's `shop.common` is the root project's
   `src/shop/common.cljc`.
2. `shop.gone`, under the project's root `shop` and defined nowhere, is
   dropped.
