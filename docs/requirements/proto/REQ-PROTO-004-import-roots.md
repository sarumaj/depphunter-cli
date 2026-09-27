---
id: REQ-PROTO-004
uuid: 7e4ad904-960f-4a0e-aea1-edd071b218b0
title: Import roots and the well-known types
scope: proto
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An import **shall** resolve to the project file at that path under the import
roots Buf's configuration declares for the importer - the nearest
`buf.work.yaml` (its `directories`) or v2 `buf.yaml` (its `modules[].path`, or
its own directory) above the file, else the nearest v1 `buf.yaml`'s directory
(or v1beta1 `build.roots`) - or, where no Buf configuration governs the file,
under the repository root, `proto/`, `protos/`, `api/`, `src/main/proto/`, and
the importer's directory and each of its ancestors, nearest first. Failing
that, a `google/protobuf/` path **shall** resolve to the hidden
`protobuf-std` island, named by its path. After the declared modules of
REQ-PROTO-005, a file under Buf's configuration tries the conventional roots as
well, and then any import resolves to the one project file whose path ends in
the imported path, or the one closest to the importer when that is closer than
all others.

## Rationale

Buf builds a workspace from the roots its configuration names; protoc is given
`-I` flags in build scripts the plugin does not read, and these point at the
repository root, a `proto/` directory, Maven's and Gradle's `src/main/proto` or
a directory above the importer. The well-known types ship with protoc and buf.

## Acceptance criteria

1. In a `buf.work.yaml` workspace listing `proto` and `third_party`,
   `import "common/v1/types.proto"` from `proto/` resolves to
   `third_party/common/v1/types.proto`; in a v2 workspace one module's file
   imports another module's by its module-relative path.
2. Without Buf configuration, `import "com/acme/items.proto"` from
   `src/main/proto/com/acme/orders.proto` and `import "shared.proto"` from a
   file beside `shared.proto` resolve to those files; `common/v1/types.proto`
   resolves to the only project file ending in that path.
3. `google/protobuf/timestamp.proto` is the `protobuf-std` package
   `google/protobuf/timestamp.proto`.
