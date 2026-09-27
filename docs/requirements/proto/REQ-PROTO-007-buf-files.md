---
id: REQ-PROTO-007
title: Buf configuration entries as imports
scope: proto
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read Buf's files with a YAML parser and record as imports:
a `buf.yaml`'s `deps` (v1 and v2), a `buf.lock`'s entries (v1 `remote`,
`owner`, `repository`; v2 `name`), a generation template's remote plugins
(v2 `remote:`, v1 `plugin:` or `remote:` naming a host) and `module` inputs,
all in the `buf` island; and the directories a `buf.work.yaml` lists, a v2
`buf.yaml`'s module paths and a template's `directory` inputs, as edges to
those directories. Local plugins and `protoc_builtin` plugins are not
dependencies of the repository.

## Rationale

Without these a project whose protos import nothing from the registry would not
show the modules and code generators its build downloads.

## Acceptance criteria

1. A v1 `buf.work.yaml` listing `proto` and `third_party` links to both
   directories; a v2 `buf.yaml` links to `api` and `internal/proto`.
2. `plugin: buf.build/protocolbuffers/go:v1.31.0` is the package
   `buf.build/protocolbuffers/go` at `v1.31.0`; `plugin: go-grpc` is not
   recorded.
