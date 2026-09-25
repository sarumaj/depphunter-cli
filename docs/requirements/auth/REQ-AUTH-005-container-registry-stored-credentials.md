---
id: REQ-AUTH-005
uuid: b7bc73c6-e46f-4b58-877b-a01b22d9873d
title: Stored container registry credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read the stored `auths` of `~/.docker/config.json` and
`~/.config/containers/auth.json`, as a base64 `auth` or a `username`/`password`
pair, and file each under its registry host, Docker Hub's historical keys being
filed under `registry-1.docker.io`.

## Rationale

Docker, Podman and the tools that borrowed their format keep registry
credentials there.

## Acceptance criteria

1. A stored `auth` for a registry is sent to that registry's host.
2. An entry for `https://index.docker.io/v1/` is sent to `registry-1.docker.io`.
