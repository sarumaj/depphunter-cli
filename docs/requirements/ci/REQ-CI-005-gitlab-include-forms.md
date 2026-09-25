---
id: REQ-CI-005
uuid: bd2db582-c374-4d16-a79b-436d31c4bba9
title: GitLab include forms
scope: ci
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The CI plugin **shall** record every entry of a GitLab pipeline's top-level
`include:` as a dependency, in each of the forms `local`, `project` (with `ref`
and `file`), `template`, `remote` and `component`, and in the short forms of a
bare path (local) or a bare `http(s)` URL (remote), given singly or as a list.

## Rationale

Includes are how GitLab pipelines pull in configuration from this repository,
other projects, the instance's templates, arbitrary URLs and the CI/CD catalog.

## Acceptance criteria

1. `local: /.gitlab/ci/build.yml` resolves to `.gitlab/ci/build.yml`.
2. `project: infra/pipelines` with `ref: <commit>` resolves to GitLab CI package
   `infra/pipelines` at that commit, pinned.
3. `template: Security/SAST.gitlab-ci.yml` resolves to package
   `template: Security/SAST.gitlab-ci.yml`.
4. `remote: https://example.com/ci/shared.yml?v=2` resolves to package
   `example.com/ci/shared.yml`.
5. `component: gitlab.com/components/sonar/scan@1.4.0` resolves to package
   `gitlab.com/components/sonar/scan`, version `1.4.0`.
