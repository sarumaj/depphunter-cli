---
id: REQ-AUTH-011
uuid: f86d224b-c880-411a-9f15-5ed0c25b2206
title: A credential goes only to its host
scope: auth
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** send a credential only to the host it was written for,
matching the request's host with its port first and then without it, a Bearer
token taking precedence over a Basic credential for the same host. Over plain
`http://` it **shall** send none, unless the host is loopback or this machine's
own configuration names that host with `http://`.

## Rationale

A registry reached on a port has a credential of its own, while a netrc names a
machine and nothing more; a company password sent to a public registry would be
a leak.

## Acceptance criteria

1. A credential for `harbor.corp:5000` is sent to `harbor.corp:5000` and not to
   another host on port 5000.
2. A credential for `nexus.corp` is sent to `nexus.corp` on any port.
3. A request to `registry.npmjs.org` carries none of a company's credentials.
