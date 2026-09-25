---
id: REQ-FND-017
uuid: eeeeebbd-0794-4727-9d2c-6f06ed20cdb8
title: Report paths made repository-relative
scope: fnd
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M13
verification:
  - unit
---

## Statement

The system **shall** make every path a report gives relative to the repository,
and **shall** drop the path, but keep the finding, when it points outside the
repository.

## Rationale

A finding outside the repository is still about a package, and the map has
somewhere to put it.

## Acceptance criteria

1. `/repo/a/b.go` becomes `a/b.go` for root `/repo`.
2. `/elsewhere/x.go` and `../x.go` lose their path; the finding remains.
