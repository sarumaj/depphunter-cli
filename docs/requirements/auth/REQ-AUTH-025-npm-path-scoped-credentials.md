---
id: REQ-AUTH-025
title: npm registry credentials serve their registry path
scope: auth
type: constraint
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

A credential npm keeps under a registry key with a path
(`//host/some/path/:_authToken`, and the `_auth`, `username` and `_password`
fields) **shall** be sent only with requests whose path is under that path,
the longest matching path winning, and a key without a path
(`//host/:_authToken`) **shall** serve the rest of the host. The same
**shall** hold for the Yarn Berry and Bun credentials of a registry with a
path ([REQ-AUTH-024](REQ-AUTH-024-yarn-and-bun-credentials.md)) and for a
credential lent to one
([REQ-AUTH-023](REQ-AUTH-023-repository-feed-credentials-from-the-environment.md)).

## Rationale

GitLab and other hosts serve one npm registry per project or group under a
path of one host, each with its own token. Filing the tokens by host kept only
the last one, sent it to every other project's registry, and sent it with any
Markdown link to the host the link checker followed.

## Acceptance criteria

1. Two path-scoped tokens on one host are each sent with their own
   registry's requests, end to end.
2. A request under neither path, including a link the link checker follows,
   carries neither token; a host-wide key serves it instead when there is
   one.
3. `/npm/` does not cover `/npm-private/`.
