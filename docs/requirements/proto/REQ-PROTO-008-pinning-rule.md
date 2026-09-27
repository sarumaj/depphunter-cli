---
id: REQ-PROTO-008
title: Buf Schema Registry pinning rule
scope: proto
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A declared Buf Schema Registry module **shall** be pinned by the commit the
`buf.lock` beside its `buf.yaml` records (the ref it was declared with kept as
requested), else by a ref that is a commit (32 hex digits, a dashed UUID or a
git commit); a label, tag or branch ref **shall** be shown as its version but
not pinned; a module with neither lock entry nor ref **shall** float. A remote
plugin **shall** be pinned by an exact version (`v1.31.0`, `v1.31.0-1`) and
float without one.

## Rationale

A BSR label moves when its owner pushes, and a module without a ref resolves to
the latest commit of the default label the next time `buf dep update` runs;
only a commit fixes the content.

## Acceptance criteria

1. `buf.build/envoyproxy/protoc-gen-validate:v1.0.4`, locked, is the lock's
   commit with `v1.0.4` requested; `buf.build/acme/payments` without a lock
   entry floats; `buf.build/bufbuild/protovalidate:0123456789abcdef0123456789abcdef`
   is pinned; `buf.build/acme/other:main` shows `main` and is not pinned.
2. `buf.build/grpc/go` without a version floats.
