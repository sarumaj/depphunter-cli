---
id: REQ-JAVA-009
uuid: 281b51ec-4de5-447c-9a8d-dbdcc0ba386e
title: Heuristic Maven matching; unmatched unresolved
scope: java
type: limitation
priority: must
status: implemented
verification:
  - unit
---

## Statement

Because Java imports name packages and not artifacts, the Java plugin **shall**
match Maven dependencies heuristically (REQ-JAVA-007), and **shall** report an
import that matches no project class, JDK package or declared artifact as an
unresolved Maven package named `group:artifact`: the table's artifact when the
table knows the package, else a guess from the import's package (the segments
before the first capitalized one, at most three) with its last segment as the
artifact.

## Rationale

No file maps a package to the artifact that ships it; an unmatched import is
better shown as unresolved than attributed to an artifact the build does not
declare. Named like every other Maven package, the unresolved node merges with
the same artifact reached another way.

## Acceptance criteria

1. `import javax.servlet.http.HttpServlet` with no servlet dependency declared
   resolves to `javax.servlet:javax.servlet-api`, unresolved.
2. `import net.sf.saxon.s9api.Processor` with nothing declared resolves to
   `net.sf.saxon:saxon`, unresolved.
