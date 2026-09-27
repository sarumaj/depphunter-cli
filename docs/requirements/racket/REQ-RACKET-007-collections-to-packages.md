---
id: REQ-RACKET-007
title: Collections to packages
scope: racket
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A collection path no file of the repository has **shall** resolve to the
hidden `racket-std` island (named by its collection) when the base
package's collections provide it (a list taken from racket/collects: two
segments deep, `private` modules included); else, by a curated table of
the main distribution's collections (`rackunit` is `rackunit-lib`,
`typed/racket` `typed-racket-lib`, `racket/gui` `gui-lib`, `net/smtp`
`net-lib`, ...), to the declared package the table names or its umbrella
package (`typed-racket` for `typed-racket-lib`), else to the one declared
package named like the collection with a dash (`srfi-lite-lib` for `srfi`),
else to the table's package, unresolved; a collection outside the table
**shall** resolve to the declared package its first segments spell (with or
without `-lib`, `-doc`, `-test`; `typed/racket` spells `typed-racket`),
else to base for a base collection's other module, else be dropped when it
is the repository's own collection, else to an unresolved package named by
its first segment. The declared packages are those of the package the file
belongs to (the nearest package directory above it), or every package's for
a file outside all.

## Rationale

Collections do not name their packages: rackunit is installed by
rackunit-lib, and one collection can span several packages.

## Acceptance criteria

1. `racket/list`, `net/url` and `data/queue` go to `racket-std`;
   `rackunit` to the declared rackunit-lib; `typed/racket/unsafe` to the
   declared typed-racket; `net/smtp`, `plot/pict` and `data/gvector` to
   unresolved net-lib, plot-lib and data-lib; `mystery/thing` to an
   unresolved `mystery`; `racket/gui/base` in widgets-lib to its declared
   gui-lib.
