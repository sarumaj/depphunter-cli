---
id: REQ-SUP-072
title: The Buf Schema Registry's graph for module dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a module of the `buf`
island depends on through the registry's public API (buf.registry.module.v1,
Connect unary calls: `POST` with a JSON body): `GraphService/GetGraph` for the
module at the reference the target names (a commit id or a label; none is the
default label's commit), whose edges from that commit lead to its direct
dependencies' commits, then `ModuleService/GetModules` and
`OwnerService/GetOwners` for their names. Each dependency **shall** be
`<host>/<owner>/<module>`, pinned by its commit id (lower-case hex without
dashes, as buf.lock writes it). A dependency on another registry (federation)
**shall** be left out. A remote plugin (lang.Target.Registry `plugin`) **shall
not** be asked; the report gives it a reason of its own (REQ-TRC-006).

The registry is the module's host: buf.build is the public index, any other
host is that registry, known when buf's credentials on this machine name it
(REQ-AUTH-031) or the user vouches for it (REQ-SUP-043). The only credential
sent is the token buf would send to that host.

## Rationale

`buf.lock` lists a workspace's modules flat; the registry's graph is where a
module's own dependencies are.

## Acceptance criteria

1. acme/payments at `v1.2.0` yields googleapis and protovalidate at their
   commits, not the transitive edge between them and not the other registry's
   module, in three calls carrying the BUF_TOKEN token for the host.
2. A module the registry does not have is "not found" after one call; a remote
   plugin makes no call.
3. A module on buf.corp.example is asked of `https://buf.corp.example`, known
   only with a token for that host or when vouched for.

## Notes

The sandbox proxy blocks buf.build, so the API was taken from its published
definitions (github.com/bufbuild/registry-proto, v1) and verified with a stub
server only.
