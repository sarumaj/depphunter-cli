---
id: REQ-CLOJURE-006
uuid: b3658345-6591-41b0-9ab1-a126d289ff63
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
(`shop.model.cart.Cart`), to a project `.java` file by path, or to the
declared artifact whose coordinates best match its package (the artifact
spelling two package segments, `commons-io` for `org.apache.commons.io`; the
whole group as a package prefix, `org.quartz-scheduler` counting for
`org.quartz`; the group's last segment as the class's first; the artifact
as one of the first package segments; ties by the artifact's words the
package names), else to a declared library's namespace; any other class
**shall** be dropped.

## Rationale

A class names no artifact; only what the project declares can say where it
comes from.

## Acceptance criteria

1. `java.util.Date` is `jdk` `java.util`, `org.eclipse.jetty.server.Server`
   is `org.eclipse.jetty:jetty-server`, `shop.Native` is
   `java/shop/Native.java`, and `com.example.Nothing` is dropped.
