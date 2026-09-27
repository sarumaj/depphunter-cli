---
id: REQ-KT-002
title: Kotlin standard library island
scope: kt
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Kotlin plugin **shall** assign imports under `kotlin.` to the ecosystem
`kotlin-std` (package: the first two segments, or `kotlin` for a name the root
package defines, such as `kotlin.require`), imports of JDK packages to the Java
plugin's `jdk` ecosystem, and **shall not** treat `kotlinx.` as part of the
standard library.

## Rationale

`kotlin.*` ships with the compiler and is imported implicitly, like the JDK is
for Java; `kotlinx.coroutines` and `kotlinx.serialization` are ordinary Maven
artifacts a build declares and pins.

## Acceptance criteria

1. `import kotlin.collections.List` resolves to `kotlin-std`, package
   `kotlin.collections`.
2. `import kotlin.math.*` resolves to package `kotlin.math`, and
   `import kotlin.require` to package `kotlin`.
3. `import java.util.UUID` resolves to `jdk`, package `java.util`.
4. `import kotlinx.coroutines.launch` resolves to the Maven artifact
   `org.jetbrains.kotlinx:kotlinx-coroutines-core` declared by the build.
