---
id: REQ-CLOJURE-002
uuid: 4b15ae40-b797-47c7-833f-36e939ef51ff
title: Requires, imports and loads read
scope: clojure
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read as imports: the libspecs of an `ns` form's
`:require`, `:use`, `:require-macros` and `:use-macros` references and of
top-level `require`, `use` and `require-macros` calls (quoted or not) - a
symbol, a vector `[lib & options]`, a prefix list `[prefix lib1 [lib2 :as
x]]` (or its list form), and a ClojureScript string (`["react" :as
react]`); the classes of `:import` and top-level `import` (`java.util.Date`,
`[java.util Date UUID]`, `(java.io File)`); `(load "path")` and
`(load-file "path")`. Every branch of a reader conditional (`#?`, `#?@`)
**shall** be read; a form after `#_` **shall not**; a libspec with only
`:as-alias` loads nothing and **shall not** be an import.

## Rationale

A `.cljc` file's dependencies are those of every platform it is compiled
for; a discarded form and an alias for keywords load nothing.

## Acceptance criteria

1. In the fixture's `src/shop/core.clj`, a prefix list yields
   `shop.model.cart` and `shop.model.order`, `#_[shop.never]` and
   `[shop.alias-only :as-alias ao]` yield nothing, and `(load "core_extra")`
   is `src/shop/core_extra.clj`.
2. `src/shop/common.cljc` imports `clojure.edn` and `cljs.reader`,
   `clojure.java.shell` and `goog.object` from `#?` and `#?@` branches.
