---
id: REQ-REGO-004
title: Package resolution
scope: rego
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A data path **shall** link to every file of the repository (but the
importer) declaring the longest package the path starts with (at most 64
files); a reference an import of the file already links **shall** not be
repeated. A reference no package declares (external data) and an import of
a namespace holding packages below it **shall** vanish; an import no
package declares **shall** be dropped.

## Rationale

A Rego package may span several files, and `data.a.b.rule` names a rule of
package `a.b`; OPA has no package manager, so undeclared data is loaded at
run time or comes from a bundle.

## Acceptance criteria

1. `import data.lib.kubernetes` links both files of that package;
   `data.external.inventory` is dropped; `data.lib.util.allow` in the test
   links util.rego; `lib.kubernetes.pods` through `import data.lib` links
   both kubernetes files; a package's own files do not link each other.
