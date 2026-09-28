---
id: REQ-DLANG-009
title: Islands
scope: dlang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The D plugin **shall** declare two islands: **dub packages** (`dub`, a
private-pattern prefix, asked about by `--online` through the registry of
REQ-SUP-060) and the hidden **D runtime and standard library** (`d-std`,
Std). OSV and Trivy have no D ecosystem, so dub packages are checked for
advisories only by their commit, when one pins them to a git repository on a
public forge (REQ-FND-026).

## Rationale

Every other plugin names its package manager's island and its standard
library the same way.

## Acceptance criteria

1. Every resolved import of the fixture is local, `dub` or `d-std`.
