---
id: REQ-CI-008
uuid: 13331fa3-0cbb-48ef-902e-424042b3a586
title: Three CI ecosystems
scope: ci
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The CI plugin **shall** place its external dependencies in three ecosystems,
drawn as the islands "GitHub Actions" (`actions`), "GitLab CI" (`gitlab-ci`) and
"Container images" (`oci`), the last shared by images from either platform.

## Rationale

Actions, GitLab includes and container images are different kinds of dependency
with different pinning rules and registries.

## Acceptance criteria

1. The plugin declares exactly these three ecosystems.
2. An image used by a GitHub workflow and one used by a GitLab pipeline resolve
   to the same `oci` ecosystem.
