---
id: REQ-AUTH-005
title: Stored container registry credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read the stored `auths` of the container registry
credential files (`~/.docker/config.json`, `~/.config/containers/auth.json`
and the others
[REQ-AUTH-020](REQ-AUTH-020-credential-file-locations.md) names), as a base64
`auth` or a `username`/`password`
pair, and file each under its registry host, Docker Hub's historical keys being
filed under `registry-1.docker.io`.

## Rationale

Docker, Podman and the tools that borrowed their format keep registry
credentials there.

## Acceptance criteria

1. A stored `auth` for a registry is sent to that registry's host.
2. An entry for `https://index.docker.io/v1/` is sent to `registry-1.docker.io`.
