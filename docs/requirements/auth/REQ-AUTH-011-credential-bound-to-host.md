---
id: REQ-AUTH-011
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

## Notes

Loopback is `localhost` or a loopback IP address (127.0.0.0/8, `::1`, and
127.x.y.z as an IPv4-mapped IPv6 address), which Go's HTTP client also sends
past an `HTTP_PROXY`. Other spellings of this machine (`LOCALHOST`, `localhost.`,
`127.1`) are not loopback here, since a request to them goes through the proxy,
and neither is an IPv6 address with a zone. A Buf token
([REQ-AUTH-031](REQ-AUTH-031-buf-tokens.md)) and a Conan login
([REQ-AUTH-035](REQ-AUTH-035-conan-remote-logins.md)), which are sent outside
the credential store, go over plain http only to the same loopback hosts.
