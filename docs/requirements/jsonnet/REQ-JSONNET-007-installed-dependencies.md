---
id: REQ-JSONNET-007
title: Installed dependencies
scope: jsonnet
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

For `--resolve-depth`, an installed package's own `jsonnetfile.json`
(`vendor/<package>/jsonnetfile.json` of a project) **shall** give its
dependencies, versioned by the installing project's lock, and **shall** be
reported as coming from what is installed.

## Rationale

jb has no registry; what it installed is the only record of a
package's own dependencies.

## Acceptance criteria

1. The installed ksonnet-util depends on doc-util (pinned by the root lock)
   and xtd (a branch, floating); k8s-libsonnet, not installed, has no
   answer.
