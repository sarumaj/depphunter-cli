---
id: REQ-CLOJURE-005
title: Clojure's standard library
scope: clojure
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Namespaces `org.clojure/clojure` ships (`clojure.core`, `clojure.string`,
`clojure.set`, `clojure.java.io`, `clojure.edn`, `clojure.walk`,
`clojure.test`, `clojure.pprint` and the rest of Clojure 1.12's), the
`clojure.lang` classes, ClojureScript's `cljs.*` (except `cljs.core.async`),
the Closure Library's `goog.*` (one package `goog`) and, in ClojureScript,
`clojure.spec.*` **shall** be packages of the hidden `clojure-std` island,
named by the namespace. `clojure.spec.alpha` and `clojure.core.specs.alpha`
in Clojure are the Maven artifacts `org.clojure:spec.alpha` and
`org.clojure:core.specs.alpha`, which Clojure itself depends on, so they are
never unresolved. In a babashka context (a `.bb` file, `bb.edn`, an
extensionless script, a file on `bb.edn`'s `:paths` or under a project whose
only manifest is `bb.edn`), a namespace built into bb (`babashka.*`,
`cheshire.core`, `clojure.data.json`, ...) that no manifest declares
**shall** be the `clojure-std` package `babashka`.

## Rationale

The standard library is present wherever Clojure runs; spec is a separate
artifact with versions of its own; babashka bundles libraries.

## Acceptance criteria

1. `clojure.string` is `clojure-std` `clojure.string`, `goog.string` and
   `goog.net.XhrIo` are `goog`, and `clojure.spec.alpha` is
   `org.clojure:spec.alpha` without a version.
2. In `scripts/release.bb`, `babashka.http-client` is `babashka`, while
   `babashka.fs`, declared by `bb.edn`, is `babashka:fs`.
