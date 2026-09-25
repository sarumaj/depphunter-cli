---
id: REQ-JAVA-009
uuid: 281b51ec-4de5-447c-9a8d-dbdcc0ba386e
title: Heuristic Maven matching; unmatched unresolved
scope: java
type: limitation
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md Known limits
  - docs/REQUIREMENTS.md M4
verification:
  - unit
---

## Statement

Because Java imports name packages and not artifacts, the Java plugin **shall**
match Maven dependencies heuristically (REQ-JAVA-007), and **shall** report an
import that matches no project class, JDK package or declared group as an
unresolved Maven package named by at most its first three segments.

## Rationale

No file maps a package to the artifact that ships it; an unmatched import is
better shown as unresolved than attributed to a guessed artifact.

## Acceptance criteria

1. `import javax.servlet.http.HttpServlet` with no servlet dependency declared
   resolves to Maven package `javax.servlet.http`, unresolved.
