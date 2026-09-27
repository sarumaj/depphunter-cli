---
id: REQ-KT-006
uuid: 3b9c1c36-ba71-4263-90a8-c04a53a01eb2
title: Imports of the default package dropped
scope: kt
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The JVM plugins **shall** drop an import that names no project source and whose
first segment begins with an upper-case letter, rather than attribute it to a
Maven group.

## Rationale

Package names begin in lower case; an upper-case root is a class of the default
package, which a Gradle Kotlin script declares and imports itself
(`import TestMode.KSP`). Shown as an unresolved Maven package, it would suggest
a missing dependency.

## Acceptance criteria

1. `import Mode.FAST` in a `build.gradle.kts` that declares `enum class Mode`
   is dropped.
