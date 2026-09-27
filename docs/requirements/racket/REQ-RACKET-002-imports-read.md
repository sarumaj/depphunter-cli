---
id: REQ-RACKET-002
title: Imports read
scope: racket
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read as imports a module's `#lang` line (each
meta-language such as `at-exp` and the language's module path; `s-exp` and
`reader` with the module path after them), a `#reader` prefix's module path,
and, at module level (in `module`, `module*` and `module+` submodules and
`begin` forms too), the module paths of `require` (through `only-in`,
`except-in`, `prefix-in`, `rename-in`, `combine-in`, `for-syntax`,
`for-template`, `for-label`, `for-meta`, `relative-in`, `multi-in` and
`path-up`), `local-require`, `require/typed` and Typed Racket's other
require forms, `lazy-require`, `include`, `load`, `load-relative`,
Scribble's `include-section`, and a submodule's language. A module path is
a collection path (`racket/list`), a relative path string, `(lib ...)`
(`(lib "list.ss")` is mzlib's), `(file ...)`, `(planet ...)` or
`(submod ...)`. Comments (`;`, `#| |#` nested, `#;` datum comments,
Scribble's `@;`), strings, here strings, regexps, characters, quoted data
and Scribble text **shall** not give imports; a Scribble document's
`@(require ...)` and `@require[...]` forms **shall**.

## Rationale

A module's dependencies are its language and its requires; code quoted
as data or shown in documentation (`@racketblock`) is not a dependency.

## Acceptance criteria

1. `main.rkt` gives exactly the imports of its require forms, none of the
   fake requires in its comments, here string, regexp, string and quoted
   list.
2. `docs/manual.scrbl` gives `#lang scribble/manual`, its `@(require ...)`
   module paths and its `include-section`, not what its prose,
   `@racketblock[...]` or `@;` comments contain.
3. A `#reader(lib "htdp-beginner-reader.ss" "lang")` line gives the reader's
   module; DrRacket's WXME format gives nothing.
