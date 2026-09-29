---
id: REQ-SUP-077
title: GitHub actions and reusable workflows read from GitHub
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, an action or reusable workflow of another repository
**shall** be read at the reference the workflow names, through a GitHub REST
API's contents endpoint (`GET <api>/repos/<owner>/<repo>/contents/<path>?ref=
<ref>`, the raw media type): a reusable workflow's own file, else the action's
`action.yml`, else its `action.yaml`, and the Dockerfile a Docker action is
built from; what they run is the answer
([REQ-CI-016](../ci/REQ-CI-016-remote-action-dependencies.md)). A reference
names no host, so the GitHub instances this machine works with **shall** be
asked first, each at its API (`<host>/api/v3`, `api.<name>.ghe.com`): the one
`GITHUB_API_URL` names, the one `GH_HOST` names and every other host of the
GitHub CLI's `hosts.yml`; then github.com's (api.github.com), the public
index, to which an organization's own action (a private pattern) is never
named. Without a credential for api.github.com, a github.com repository's
files **shall** be read from raw.githubusercontent.com instead, which serves
public repositories without the API's limit. A reference without `@ref` is
not asked, and a GitHub API that refuses to answer (403 or 429) **shall** be
noted (`forbidden`).

## Rationale

A composite action or reusable workflow runs further actions, and the
workflow that calls it does not say which: GitHub's files are the only place
that does. An Enterprise Server runs the actions it holds before github.com's.

## Acceptance criteria

1. `GITHUB_API_URL`, `GH_HOST` and `hosts.yml` add their instances' APIs
   before api.github.com; api.github.com itself adds nothing.
2. Without a token, github.com files come from raw.githubusercontent.com (a
   sub-directory's `action.yml`, `action.yaml`, a Dockerfile, a workflow) and
   the API is not asked; a missing file and a missing reference are
   reported.
3. With a token, the contents API is asked with the raw media type; an
   instance named by `GITHUB_API_URL` is asked first with its own token, and an
   organization's own action is not named to github.com.
4. A 403 is noted once.

## Notes

The shapes were checked against GitHub's documentation and
raw.githubusercontent.com (reachable from the sandbox; api.github.com is
not). A repository cannot name a GitHub instance: a workflow's `uses:` has no
host. Whether an action is on an Enterprise Server or on github.com is not
known offline, so its commit is not sent to OSV
([REQ-FND-026](../fnd/REQ-FND-026-commit-pinned-git-dependencies-asked-by-commit.md)).
