---
id: REQ-CI-001
uuid: 228fb20e-145d-41a9-a8b6-ab13459e52f9
title: CI configuration files are analyzed
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

The CI plugin **shall** analyze as dependency sources the non-binary `.yml` and
`.yaml` files under `.github/workflows/`, every `action.yml` and `action.yaml`,
`.gitlab-ci.yml`, `.gitlab-ci.yaml`, any file ending in `.gitlab-ci.yml`, and
any YAML file under `.gitlab/`, and **shall** read each as a GitHub workflow, a
GitHub action or a GitLab pipeline according to its location.

## Rationale

Continuous integration pulls in third-party code that runs with the repository's
secrets, and no package manifest records it. The same YAML means different
things depending on which platform reads it, so the kind of file is part of its
extraction cache key.

## Acceptance criteria

1. `.github/workflows/nightly.yaml`, `tools/my-action/action.yaml`,
   `ci/build.gitlab-ci.yml` and `.gitlab/ci/test.yml` are claimed.
2. `docker-compose.yml`, `.github/dependabot.yml` and
   `.github/workflows/notes.txt` are not claimed.
3. Two files with identical content but of different kinds do not share an
   extraction cache entry.
4. A file that does not parse as YAML yields no dependencies and no error.
