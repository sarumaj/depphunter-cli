---
id: REQ-JAVA-007
uuid: 3251b611-d90d-420a-af16-15c8d901c0bd
title: Imports matched to declared groupIds
scope: java
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

The Java plugin **shall** match an import that names no project class to a
declared Maven groupId by, in order: the longest groupId that is a package
prefix of the import; else the groupId sharing the most leading segments with it
(at least three, or all segments of a shorter groupId); else a table of known
package-to-group mismatches, applied only when that group is declared; else a
group whose artifactId or last groupId segment equals one of the import's first
two segments.

## Rationale

Java imports name packages, not artifacts, and a library's packages need not
start with its groupId (`com.fasterxml.jackson.databind` comes from
`com.fasterxml.jackson.core`, `okhttp3` from `com.squareup.okhttp3`,
`com.google.common` from `com.google.guava`).

## Acceptance criteria

1. `org.springframework.context.ApplicationContext` matches
   `org.springframework` by prefix.
2. `com.fasterxml.jackson.databind.ObjectMapper` matches
   `com.fasterxml.jackson.core` by shared segments.
3. `okhttp3.OkHttpClient` matches `com.squareup.okhttp3` by artifact alias.
4. `com.google.common.collect.Lists` matches `com.google.guava` and
   `org.junit.Test` matches `junit` through the known-mismatch table.
5. When several artifacts of one group are declared at different versions, the
   group carries no version.
