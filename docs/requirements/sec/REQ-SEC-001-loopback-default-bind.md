---
id: REQ-SEC-001
uuid: b8723f62-de1c-4ef6-ab40-7f92b6e08c03
title: Loopback bind by default
scope: sec
type: non-functional
priority: must
status: implemented
verification:
  - unit
  - inspection
---

## Statement

The server **shall** listen on `127.0.0.1`, on a free port, unless the listen
address is configured otherwise.

## Rationale

The view is local-only by design; it serves the repository's contents and must
not be reachable from the network by default.

## Acceptance criteria

1. The default of `addr` is `127.0.0.1:0`.
2. A run without `--addr` logs a URL of the form
   `http://127.0.0.1:<port>/?token=...`.

## Notes

`TestServesOnLoopbackByDefault` runs the built binary without `--addr` and
checks the logged URL.
