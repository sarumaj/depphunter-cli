---
id: REQ-CLOJURE-007
uuid: 864b3504-aa27-4493-aed0-f7563baa982f
title: Namespaces to declared artifacts
scope: clojure
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A namespace no project file defines and not in the standard library
**shall** resolve to the artifact a governing project declares for it: the
artifact of a curated table (`ring.util.*` is `ring:ring-core`,
`honey.sql` `com.github.seancorfield:honeysql`, `reitit.<x>`
`metosin:reitit-<x>`, `taoensso.<x>` `com.taoensso:<x>`, ...) or of Clojure
contrib's rule (`clojure.data.json` is `org.clojure:data.json`) when that
artifact is declared, else the best-named declared artifact: the
namespace's leading segments folding to its name (`next.jdbc`,
`cheshire.core`, `reitit.ring` for `reitit-ring`), also without a
`clj-`/`-clojure` affix (`jmh.core` for `jmh-clojure`); the group's last
segment, then the artifact (`taoensso.timbre`, `integrant.repl`); every
word of the artifact in the namespace (`ring.mock.request` for
`ring-mock`); and, only for a namespace no table knows and outside the
project's root, the group alone (`datomic.api` for
`com.datomic/datomic-pro`). Otherwise it **shall** be an unresolved Maven
package named by the table (or contrib rule), else by the namespace
(`com.a.b` is `com.a:b`, `foo.core` `foo:foo`).

## Rationale

Clojure namespaces name no artifact; the declared dependencies, a table of
libraries that break the naming habits, and those habits connect them.

## Acceptance criteria

1. `taoensso.timbre`, `next.jdbc`, `reitit.ring` and `ring.util.response`
   resolve to their declared artifacts; `clojure.data.json` undeclared is
   unresolved `org.clojure:data.json` and `unknown.lib` is `unknown:unknown`.
2. `babashka.curl` is not matched to a declared `babashka/fs`.
