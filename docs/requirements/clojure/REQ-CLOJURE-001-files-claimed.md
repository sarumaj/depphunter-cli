---
id: REQ-CLOJURE-001
uuid: 6c6d49f9-710d-4cea-9193-831efd8bed7b
title: Files claimed
scope: clojure
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Clojure plugin **shall** claim Clojure, ClojureScript and babashka
sources (`.clj`, `.cljs`, `.cljc`, `.bb`, and a file without a language whose
`#!` line runs `bb`), the manifests `deps.edn`, `bb.edn`,
`shadow-cljs.edn`, Leiningen's `project.clj` and Boot's `build.boot`, and
other `.edn` files, which are data: claimed, but declaring no imports or
symbols. It **shall not** claim files under `.cpcache`, `.shadow-cljs`,
`.lsp` or `node_modules`. A manifest's extraction **shall** be cached apart
from a source's of the same extension (`project.clj` and `core.clj`).

## Rationale

Clojure's manifests are Clojure data or code and share its extensions;
babashka scripts are often extensionless. The skipped directories are the
tools' caches (`.clj-kondo` is read: projects keep hook code there).

## Acceptance criteria

1. `src/a.clj`, `src/a.cljs`, `src/a.cljc`, `x.bb`, `deps.edn`, `project.clj`,
   `build.boot`, `bb.edn`, `shadow-cljs.edn` and `conf/any.edn` are claimed;
   `.shadow-cljs/builds/a.cljs`, `node_modules/x/a.cljs` and
   `.cpcache/x.edn` are not.
2. An extensionless script whose `#!` line runs `bb` is claimed.
3. `project.clj` and `core.clj` have different cache classes.
