---
id: REQ-JAVA-001
uuid: 47ff05fa-286f-4edd-90d1-8ea437472179
title: Project classes found by package-path suffix
scope: java
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Java plugin **shall** resolve an import to the project source file whose
path ends with the import's package path followed by the class name and `.java`,
under any source root, using the longest prefix of the import that names such a
file; a wildcard import (`a.b.*`) that names no class **shall** resolve to the
project directory whose path ends with the package path.

## Rationale

Java source roots differ between build tools and layouts (`src/main/java`,
`src`, generated roots); matching the package path as a suffix finds a class
without configuring them.

## Acceptance criteria

1. `import com.example.app.model.User` resolves to
   `src/main/java/com/example/app/model/User.java`.
2. `import com.example.app.model.*` resolves to the directory
   `src/main/java/com/example/app/model`.
3. `import static com.example.app.util.Strings.trim` resolves to
   `src/main/java/com/example/app/util/Strings.java`.
4. `import org.acme.net.Client` resolves to
   `lib/src/main/java/org/acme/net/Client.java` in a second module.
