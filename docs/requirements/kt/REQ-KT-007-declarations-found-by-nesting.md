---
id: REQ-KT-007
title: Declarations found by nesting
scope: kt
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The declaration index of Kotlin and Scala files **shall** take as top-level
the definitions that start outside every bracket (`{`, `(`, `[`) and no further
indented than the first line of their scope, whatever their column, after
setting aside comments (nested ones included), string and character literals,
Kotlin raw strings and string templates (`${...}`), and Scala triple-quoted
strings and interpolated strings (`s`, `f`, `raw` and any other interpolator).
A Scala 2 package block (`package a { ... }`) and a Scala 3 one (`package a:`
followed by indented lines) **shall** declare what they hold in their own
package, nested inside the file's; imports of such a file **shall** be relative
to the package all its packages lie in. Input no compiler accepts **shall** be
read in time linear in its size, without failing.

## Rationale

A column rule missed every definition inside a package block, read code quoted
in a raw string or a comment as a definition, and took a member written at the
first column for a top-level one.

## Acceptance criteria

1. In `package com.acme` followed by `package app { class A }` and
   `package lib { object B { class Inner } }`, `A` is declared in
   `com.acme.app` and `B` in `com.acme.lib`; `import com.acme.lib.B.Inner`
   resolves to the file, and `Inner` is not declared.
2. `package a:` and `package b:` with indented bodies declare their definitions
   in `a` and `b`; members of `object O:` and a closing `end O` are not
   declared.
3. `class NotThis {` inside a raw string, a nested comment or a template, and
   braces inside a character literal, a template or a backquoted name, change
   neither the declarations nor the nesting.
4. A member written at the first column inside a class body and a constructor
   parameter `val x` on its own line are not declared.
5. Unterminated strings, comments and templates, 200 000 unclosed braces,
   stray closing brackets and 5 000 nested templates are read without a panic
   in under two seconds, and so is every prefix of a mixed source and that
   source with any one byte removed.

## Notes

The scanner is shared by the Kotlin, Scala and Java plugins through the Java
resolver; see [REQ-KT-005](REQ-KT-005-declarations-read-from-the-text.md) for
what it cannot see.
