---
id: REQ-AUTH-009
uuid: 0b52daea-dd09-455b-b920-3e0473854625
title: Environment references in credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** expand a credential value written as `${NAME}`,
`${env.NAME}` or `%NAME%` to the value of that environment variable.

## Rationale

A password can then live in the environment rather than on disk, the way
pipelines write it.

## Acceptance criteria

1. `${env.NEXUS_PASSWORD}` in a Maven server, `%AZ_PAT%` in a NuGet credential
   and `${NPM_TOKEN}` in an `.npmrc` resolve to the variables' values.
