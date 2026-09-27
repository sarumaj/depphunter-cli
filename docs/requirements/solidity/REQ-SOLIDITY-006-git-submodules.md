---
id: REQ-SOLIDITY-006
title: Git submodules
scope: solidity
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The submodules of `.gitmodules` (the scanned ones, else the root's) **shall**
be packages of the `git-submodule` island named by their URL as
`lang.RepoName` names repositories (lower case on GitHub, GitLab,
Bitbucket, Codeberg and sourcehut, which ignore case; a relative URL's
submodule by its path's last element), with the URL as origin only off
those public forges. The commit git's index records for a submodule (`git
ls-files --stage`, a gitlink) **shall** pin it, with its `.gitmodules`
branch as requested; without a recorded commit a branch is its version and
floats, and so does a submodule with neither. The `.gitmodules` of a
submodule that is checked out **shall** be read too: its submodules are
packages paths inside it reach, and the submodule's dependencies (read from
its checkout) for `--resolve-depth`.

## Rationale

`git submodule update` checks out the recorded commit, so it is what the
project builds with; the branch only says what `--remote` would follow.
The recorded commit is in git, not in any file, which is why git is asked.

## Acceptance criteria

1. In a git repository whose index records a gitlink for `lib/forge-std`,
   imports reaching it are pinned to that commit with branch `v1` requested,
   and a submodule without a gitlink floats.
2. The fixture's ds-test remapping reaches the ds-test submodule of the
   checked-out forge-std, which is forge-std's dependency.
