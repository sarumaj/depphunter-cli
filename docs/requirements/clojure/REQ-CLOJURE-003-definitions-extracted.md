---
id: REQ-CLOJURE-003
title: Definitions extracted
scope: clojure
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as symbols a file's namespace (kind
`namespace`) and its top-level definitions (also inside a top-level `do`
and in reader-conditional branches, each name once): `def` and `defonce`
(var), `defn` and `defn-` (func), `defmacro` (macro), `defmulti` (func),
`defmethod` (method, named by the multimethod and its dispatch value, `area
:circle`), `defprotocol` and `definterface` (interface, with their methods
as `Protocol.method`), `defrecord` and `deftype` (class), `defstruct` (type)
and `deftest` (test), and a library's macro of the same name for the
function-like forms (`s/defn`, `mu/defn`). Definitions in a `(comment ...)`
block and spec definitions (`s/def`) **shall not** be symbols.

## Rationale

These are what a Clojure namespace defines for others to use.

## Acceptance criteria

1. `src/shop/core.clj` defines exactly the symbols its test lists, and
   `src/shop/common.cljc` defines `now` once though both branches define it.
