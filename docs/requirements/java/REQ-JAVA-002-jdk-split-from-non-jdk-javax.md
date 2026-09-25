---
id: REQ-JAVA-002
uuid: d34de3ce-316b-4cb4-86f8-b029255a4a89
title: JDK packages split from non-JDK javax
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

The Java plugin **shall** assign imports of JDK packages (`java.`, `jdk.`,
`sun.`, `com.sun.`, `org.w3c.dom`, `org.xml.sax`, `org.ietf.jgss`, `org.omg.`
and the `javax` packages that ship with the JDK) to the Java standard library
ecosystem, identified by their first two segments, and **shall** treat all other
`javax` packages as Maven dependencies.

## Rationale

`javax` is split: `javax.swing` ships with the JDK while `javax.servlet` and
`javax.persistence` are separate artifacts. Treating the whole namespace alike
would hide real dependencies or invent missing ones.

## Acceptance criteria

1. `import java.util.List` resolves to ecosystem `jdk`, package `java.util`.
2. `import javax.swing.JFrame` resolves to ecosystem `jdk`, package
   `javax.swing`.
3. `import javax.servlet.http.HttpServlet` with no matching declared group
   resolves to an unresolved Maven package.
