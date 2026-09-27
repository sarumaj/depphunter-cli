---
id: REQ-RACKET-005
uuid: 10ddb8b4-d9d8-41f2-a8c7-cb67da7f91bd
title: Collections of the repository
scope: racket
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A collection path (`coll/sub/x` is `sub/x.rkt` in collection `coll`,
`coll` alone its `main.rkt`; a `#lang` path's `lang/reader.rkt` first)
**shall** resolve to a file of the repository when one of its collections
has it: the directory of a package's `info.rkt` for a string `collection`
(or the package's directory name when `collection` is not defined or is
`'use-pkg-name`), each subdirectory of a package whose `collection` is
`'multi`, and each subdirectory of a `collects/` directory; with several
candidates the one sharing the longest directory prefix with the importing
file wins. A package is a directory whose `info.rkt` defines `collection`,
`deps`, `build-deps` or `pkg-desc`; the repository root's package is named
by its string `collection`, else by the root directory's name.

## Rationale

Monorepos such as racket/racket and typed-racket keep many packages, and
Racket's own collections in `racket/collects`; their modules require each
other by collection path.

## Acceptance criteria

1. `shop/util` resolves to `util.rkt`, `widgets/button` to
   `libs/widgets-lib/widgets/button.rkt` (a multi-collection package) and
   `compat` to `collects/compat/main.rkt`.
