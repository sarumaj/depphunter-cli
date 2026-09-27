---
id: REQ-KT-001
uuid: ed55f77d-8421-459f-8684-841cabc95b1d
title: Kotlin imports and definitions extracted
scope: kt
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Kotlin plugin **shall** analyze `.kt` and `.kts` files with tree-sitter,
reporting every `import` with its path (`a.b.C`, a wildcard `a.b.*`) and its
alias (`as D`) kept in the displayed spec, and as symbols the file's classes,
interfaces, enum classes, objects, functions, properties and type aliases and
the functions of its classes, objects, enum classes and companion objects
(`Owner.name`).

## Rationale

The Java plugin's granularity - types and their methods - is what the map
draws; Kotlin adds top-level functions and properties, which are what a Kotlin
file often consists of.

## Acceptance criteria

1. `import okhttp3.OkHttpClient as Http` is reported with that spec and the
   module `okhttp3.OkHttpClient`.
2. A class `App` with a function `run` and a companion object function `create`
   yields `App` (class), `App.run` and `App.create` (method).
3. `enum class Mode` is an `enum`, `interface Service` an `interface`,
   `object Registry` an `object`, `typealias Name` a `type`.
