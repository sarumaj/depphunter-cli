---
id: REQ-AUTH-012
uuid: 6183f40f-be7e-4274-924c-78f7c63d6a51
title: URL credentials from this machine only
scope: auth
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** take a credential written into an index URL
(`https://user:password@host/...`) only from this machine's own configuration
and file it under that URL's host; a credential in an index URL the repository
names **shall** be discarded.

## Rationale

A repository that could supply a credential could also choose where it is sent.

## Acceptance criteria

1. A machine-configured index URL with a credential makes requests to its host
   carry that credential.
2. The same URL in a repository file contributes no credential.
