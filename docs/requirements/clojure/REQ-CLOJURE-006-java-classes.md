---
id: REQ-CLOJURE-006
title: Java classes imported
scope: clojure
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An imported class **shall** resolve to the JDK island (`jdk`, as the Java
plugin places it: `java.util.Date` is `java.util`), to the project file of
the namespace whose `defrecord` or `deftype` compiles to it
(`shop.model.cart.Cart`), or to a project `.java` file by path. Otherwise it
**shall** resolve as the Java plugin matches a Java import
([REQ-JAVA-007](../java/REQ-JAVA-007-artifact-matching-heuristics.md)),
over the Maven artifacts the governing manifests declare, by package prefix
only (a table of well-known libraries, the prefixes an artifact's name derives,
its group; Clojure's own root packages derive from no artifact): to the
declared artifact, with its declared coordinate (version, pin, git origin), or
to a table artifact of a declared group that arrives with it
(`com.fasterxml.jackson.annotation` is `jackson-annotations` beside
`jackson-databind`) at the version the group's declared artifacts share.
Failing that, it **shall** resolve to the declared artifact whose coordinates
best match its package (the artifact spelling two package segments,
`commons-io` for `org.apache.commons.io`; the whole group as a package
prefix, `org.quartz-scheduler` counting for `org.quartz`; the group's last
segment as the class's first; the artifact as one of the first package
segments; ties by the artifact's words the package names), else to a
declared library's namespace; any other class **shall** be dropped, not
named unresolved as a Java import is.

## Rationale

A class names no artifact; only what the project declares can say where it
comes from. Sharing the Java plugin's matcher gives a class the node a Java
import of it gets (`com.google.common` is Guava's though the group is
`com.google.guava`). Clojure code routinely imports classes of jars only a
dependency brings (quartzite's `org.quartz`, 149 imports in metabase); an
unresolved node guessed for each would be noise.

## Acceptance criteria

1. `java.util.Date` is `jdk` `java.util`, `org.eclipse.jetty.server.Server`
   is `org.eclipse.jetty:jetty-server`, `shop.Native` is
   `java/shop/Native.java`, and `com.example.Nothing` is dropped.
2. With `com.google.guava/guava` declared,
   `com.google.common.collect.ImmutableList` is `com.google.guava:guava` at
   its declared version, the node a Java import of it and the `deps.edn`
   dependency reach.
3. With `com.fasterxml.jackson.core/jackson-databind` declared at 2.17.0,
   `com.fasterxml.jackson.annotation.JsonProperty` is
   `com.fasterxml.jackson.core:jackson-annotations` at 2.17.0.
4. `org.quartz.JobKey`, declared by nothing, and
   `com.google.common.jimfs.Jimfs`, which the table places in the undeclared
   `com.google.jimfs:jimfs`, are dropped.
