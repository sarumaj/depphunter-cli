---
id: REQ-CI-015
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

Private-package handling itself belongs to scope `sup`. With `--online`, an
action is asked of the GitHub instances this machine works with before
github.com, and the map names the one that had it
([REQ-SUP-077](../sup/REQ-SUP-077-github-actions-files.md)); an organization's
own action (`actions:<glob>`) is not named to github.com there.
