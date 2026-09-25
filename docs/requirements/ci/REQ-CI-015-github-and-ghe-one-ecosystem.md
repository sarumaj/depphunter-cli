---
id: REQ-CI-015
uuid: 80afd2eb-e717-49c8-bdc8-d7c9cf09a611
title: GitHub.com and GHE actions share one ecosystem
scope: ci
type: limitation
priority: must
status: implemented
verification:
  - unit
---

## Statement

The CI plugin **shall** place an `owner/repo@ref` action reference in the single
GitHub Actions ecosystem whether it comes from github.com or from a GitHub
Enterprise instance; the system **shall** accept `--private 'actions:<glob>'` to
keep an instance's own actions off the public index and out of the vulnerability
database.

## Rationale

An action reference does not name its host, so the two cannot be told apart from
the workflow alone.

## Acceptance criteria

1. `--private 'actions:internal-org/*'` is accepted, and packages
   `internal-org/…` in the `actions` ecosystem are treated as private.

## Notes

Private-package handling itself belongs to scope `sup`. No test names the
`actions` ecosystem in a `--private` pattern.
