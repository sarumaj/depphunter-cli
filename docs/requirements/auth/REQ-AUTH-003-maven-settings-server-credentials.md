---
id: REQ-AUTH-003
uuid: a6b0d6a5-e9ff-4374-bef4-00111d31db34
title: Maven server credentials
scope: auth
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M16
verification:
  - unit
---

## Statement

The system **shall** read the `<servers>` of `~/.m2/settings.xml` and file each
server's username and password under the host of the `<mirror>` or profile
`<repository>` whose id it names.

## Rationale

A Maven server entry names a credential by id; the host is written in the mirror
or repository with the same id.

## Acceptance criteria

1. A server whose id matches a mirror is filed under the mirror's host, and one
   matching a profile repository under that repository's host.
2. A server that no mirror or repository names is not kept.
