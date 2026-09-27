---
id: REQ-JAVA-007
uuid: 3251b611-d90d-420a-af16-15c8d901c0bd
title: Imports matched to declared artifacts
scope: java
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Java plugin **shall** resolve an import that names no project class to the
declared Maven artifact that best matches it, named `group:artifact`: the
artifact with the longest package prefix of the import among its own prefixes -
those of a table of well-known libraries, and those its name derives (the group
followed by the name's words beyond those repeating the group, the same beside
the group's last segment for a group of three or more segments, and the name's
words as a root package) - and its group, its own prefix winning a tie with the
group; else one sharing at least three leading segments with its group (or all
of a shorter group); else one whose group's last segment or name is one of the
import's first two segments. A tie **shall** go to the artifact whose group
names the import's root package, then to the one more of whose
name words the import spells, then to the family's main artifact (named after
the group alone, else ending in `core`, `api` or `common`), then to a declared
artifact, then to the shorter name, then alphabetically. A table artifact that
no manifest declares **shall** still be the match when another artifact of its
group is declared (it arrives with it), carrying the version all declared
artifacts of the group share, if any. An import the table places in an artifact
that is neither declared nor arrives with a declared one, by a prefix at least
as long as the best match's, **shall** match no declared artifact.

## Rationale

Java imports name packages, not artifacts, and a library's packages need not
start with its groupId (`com.fasterxml.jackson.databind` from
`com.fasterxml.jackson.core:jackson-databind`, `okhttp3` from
`com.squareup.okhttp3:okhttp`, `com.google.common` from
`com.google.guava:guava`). Families such as Jackson, Spring Boot and Netty are
declared by one artifact but imported from several.

## Acceptance criteria

1. `org.springframework.context.ApplicationContext` resolves to
   `org.springframework:spring-context`.
2. `com.fasterxml.jackson.databind.ObjectMapper` resolves to
   `com.fasterxml.jackson.core:jackson-databind`, and
   `com.fasterxml.jackson.annotation.JsonProperty` to
   `com.fasterxml.jackson.core:jackson-annotations` at databind's version.
3. `okhttp3.OkHttpClient` resolves to `com.squareup.okhttp3:okhttp`,
   `com.google.common.collect.Lists` to `com.google.guava:guava` and
   `org.junit.Test` to `junit:junit`.
4. With `ktor-client-core` and `ktor-client-cio` declared,
   `io.ktor.client.HttpClient` resolves to the former and
   `io.ktor.client.engine.cio.CIO` to the latter.
5. With only Spring Boot starters declared,
   `org.springframework.boot.SpringApplication` resolves to
   `org.springframework.boot:spring-boot`.
6. An artifact declared at two different versions carries no version.
7. With `org.liquibase:liquibase-core` and
   `com.github.blagerweij:liquibase-sessionlock` declared, `liquibase.Liquibase`
   resolves to the former.
8. With only `com.google.guava:guava` declared,
   `com.google.common.jimfs.Jimfs` is unresolved `com.google.jimfs:jimfs`.

## Notes

The table lives in `internal/lang/java/known.go`. Artifact names as root
packages are how Scala libraries are imported
([REQ-SCALA-005](../scala/REQ-SCALA-005-artifact-names-as-root-packages.md)).
