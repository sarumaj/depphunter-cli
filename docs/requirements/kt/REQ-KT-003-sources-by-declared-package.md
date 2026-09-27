---
id: REQ-KT-003
uuid: 854a7b37-1276-47af-8288-30d9341eb979
title: Project sources found by declared package
scope: kt
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Kotlin plugin **shall** resolve an import to the project Kotlin or Scala
file whose `package` declaration and top-level definitions name the import's
longest prefix, wherever the file sits, and to a Java file as the Java plugin
does; an import naming a package **shall** resolve to the directory holding
all of that package's files, or to its first file when they are spread over
several. The Java plugin **shall** resolve its imports of Kotlin classes and of
a Kotlin file's facade class (`StringsKt` for `strings.kt`) the same way.

## Rationale

Kotlin does not require a file to sit in a directory named after its package,
and a file commonly holds several classes and top-level functions, so the
Java plugin's path matching alone would miss them. Mixed Java and Kotlin
modules import in both directions.

## Acceptance criteria

1. `import com.example.app.model.User` resolves to
   `src/main/kotlin/models/Models.kt`, which declares
   `package com.example.app.model` and `data class User`.
2. `import com.example.app.util.shout` resolves to the file defining the
   top-level function `shout`.
3. `import com.example.app.util.*` resolves to the directory of that package's
   only file.
4. A Java file's `import com.example.app.util.StringsKt` resolves to
   `Strings.kt`, and a Kotlin file's `import org.acme.net.Client` to
   `Client.java`.

## Notes

The index is read from the text of each `.kt` and `.scala` file on every run
(see [REQ-KT-005](REQ-KT-005-declarations-read-from-the-first-column.md)).
