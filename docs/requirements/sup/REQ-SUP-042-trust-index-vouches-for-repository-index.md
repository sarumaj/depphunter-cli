---
id: REQ-SUP-042
uuid: 26a3db51-6e35-47d2-b674-4304394f7dfc
title: Vouching for a repository index
scope: sup
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
---

## Statement

An index URL named by `--trust-index` (repeatable) or `trust_indexes:` **shall**
be treated as though this machine's configuration named it: its packages **shall
not** be marked and it **may** be fetched from.

## Rationale

An organization whose repositories carry their own `.npmrc` would otherwise see
every package marked, and a warning that is always on is a warning nobody reads.

## Acceptance criteria

1. A package whose repository-only index is vouched for is not marked.
2. The resolution report lists that index with origin `--trust-index`.
