---
id: REQ-SUP-033
uuid: e2696a49-e39c-445c-9ddb-4ab8706d5e91
title: Index requests carry the host's credential
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** send with each request the credential this machine
holds for that request's host, and no other.

## Rationale

A private feed answers instead of returning 401; a credential sent to another
host would be a leak.

## Acceptance criteria

1. A request to the host a credential was written for carries it; a request to
   another host carries none.

## Notes

The reading and matching of credentials is specified by scope
[`auth`](../auth/).
