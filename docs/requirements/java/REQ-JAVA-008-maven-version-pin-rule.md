---
id: REQ-JAVA-008
uuid: 08ca76c9-3c85-4135-9a12-1b9d29e0d6d3
title: Maven pins a plain version, not a snapshot
scope: java
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The Java plugin **shall** mark a Maven dependency as pinned when its version is
a plain version of any shape, including qualifiers such as `1.2.3.RELEASE`, or a
single-version range `[1.2.3]`, and **shall not** mark it pinned when the
version is empty, a range, `LATEST`, `RELEASE`, an unexpanded property, or ends
in `-SNAPSHOT`.

## Rationale

Maven resolves a plain version to exactly that artifact whatever its shape,
while a `-SNAPSHOT` is republished under the same name and a range or keyword
resolves to whatever is newest.

## Acceptance criteria

1. `6.1.0`, `33.0.0-jre` and `4.13.2` are pinned.
2. `[1.7,2.0)` is not pinned.
3. `1.2.3.RELEASE` is pinned; `1.0-SNAPSHOT`, `LATEST` and `${undefined}` are
   not.

## Notes

Criterion 3 has no automated test; `pinnedMaven` is exercised only through the
test repository's versions. The general floating concept belongs to scope `sup`.
