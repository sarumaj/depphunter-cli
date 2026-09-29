---
id: REQ-CI-016
title: What another repository's action or reusable workflow runs
scope: ci
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The CI plugin **shall** keep the path inside the repository of an action or
reusable workflow reference (`owner/repo/path@ref`) with its target, and
**shall** read the files another repository's action or reusable workflow is
made of as it reads this repository's, for the index client to answer what
it runs: an action's `action.yml` gives a composite action's steps' `uses:`
and a Docker action's image (`docker://`, or the images the `FROM` of the
Dockerfile it names is built on); a JavaScript action runs no other action; a
reusable workflow gives its jobs' actions, the workflows they call and the
images they run in, a job's `./.github/workflows/<file>` being a workflow of
the same repository at the same reference. A step's `./path` is a directory
of the calling workflow's workspace, not of the action's repository, and
**shall not** be an answer.

## Rationale

An action is code that runs with the repository's secrets, and a composite
action or reusable workflow pulls in further actions nobody reviewed with it.

## Acceptance criteria

1. `octo-org/shared/.github/workflows/release.yml@<commit>` keeps
   `.github/workflows/release.yml` as its path.
2. A composite action answers its steps' actions (a sub-directory's path
   kept, a commit pinned) and a `docker://` step's image, not a `./` step; a
   Docker action answers its image or its Dockerfile's `FROM` images; a
   JavaScript action answers nothing.
3. A reusable workflow answers its steps' actions, its container images and
   its jobs' workflows, a local one as the same repository at the same
   reference.

## Notes

The files are fetched by the index client
([REQ-SUP-077](../sup/REQ-SUP-077-github-actions-files.md)). A repository
used at several paths (`actions/cache/save` and `actions/cache/restore`) is
one package on the map, whose files are read at the first path met.
