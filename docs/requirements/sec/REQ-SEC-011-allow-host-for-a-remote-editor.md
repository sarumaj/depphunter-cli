---
id: REQ-SEC-011
title: An editor served from elsewhere is allowed by its origin
scope: sec
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

`--allow-host <origin>` **shall** add the origin to those named by `--embed`
and, unless `--addr` is given on the command line, **shall** set the listen
address to `0.0.0.0:0`. It **shall** be accepted only on the command line,
never from a configuration file or the environment, and each value **shall**
pass the same check as an `--embed` origin. It is repeatable.

## Rationale

An editor served to a browser, such as VS Code at `https://example.com`, frames
the map from that origin and loads it from the browser's machine rather than
this one, so the server must both be framable by it and reachable from the
network. One flag says both. Because it opens the server to the network, only
the command line may turn it on.

## Acceptance criteria

1. `--allow-host https://example.com` yields `frame-ancestors` naming
   `https://example.com` and an address of `0.0.0.0:0`.
2. With `--addr 0.0.0.0:8080` as well, the address stays `0.0.0.0:8080`.
3. `allow_host` in a configuration file or `DEPPHUNTER_ALLOW_HOST` enables
   nothing, and an `addr` from a configuration file is kept.
4. A value an `--embed` origin may not be, such as `https://example.com/`, is
   refused before the server starts.

## Notes

The token is still required: in embed mode it stays in the address
([REQ-SEC-004](REQ-SEC-004-unauthenticated-requests-refused.md)). The `Host`
check is off on a non-loopback address
([REQ-SEC-005](REQ-SEC-005-foreign-host-header-rejected.md)).
