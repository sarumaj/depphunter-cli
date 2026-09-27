---
id: REQ-PROTO-006
title: Well-known third-party protos
scope: proto
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** attribute the import paths of widely used third-party
protos to the Buf Schema Registry modules publishing them by a curated table
of path prefixes (the longest matching prefix wins): `google/` (other than
`google/protobuf/`) to `buf.build/googleapis/googleapis`, `validate/` to
`buf.build/envoyproxy/protoc-gen-validate`, `buf/validate/` to
`buf.build/bufbuild/protovalidate`, `grpc/` to `buf.build/grpc/grpc`,
`gogoproto/` to `buf.build/gogo/protobuf` or `buf.build/cosmos/gogo-proto`,
`protoc-gen-openapiv2/options/` to `buf.build/grpc-ecosystem/grpc-gateway`,
and Envoy, xDS/UDPA, OpenTelemetry, Prometheus and Cosmos SDK protos to their
modules. Where a Buf configuration declares one of a path's candidates, that
one is used (REQ-PROTO-005); an import the project does not have, that no
declared module provides and that the table knows **shall** be the table's
first module, unresolved; any other **shall** be unresolved and named by the
path's first directory.

## Rationale

Projects built with protoc copy or download these protos without declaring
them anywhere the plugin reads; the table still names the module a Buf user
would declare, and the unresolved mark says nothing declares it.

## Acceptance criteria

1. Without Buf configuration, `import "gogoproto/gogo.proto"` is the
   unresolved `buf.build/gogo/protobuf` and `import "mystery/v1/thing.proto"`
   the unresolved `mystery`.
2. `import "validate/validate.proto"` with
   `buf.build/envoyproxy/protoc-gen-validate` declared resolves to it;
   `protoc-gen-openapiv2/options/annotations.proto` with
   `buf.build/grpc-ecosystem/grpc-gateway` declared resolves to it.
