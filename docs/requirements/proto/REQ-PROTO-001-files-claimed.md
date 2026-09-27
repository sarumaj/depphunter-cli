---
id: REQ-PROTO-001
title: Protocol Buffers and Buf files claimed
scope: proto
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Protocol Buffers plugin **shall** claim `.proto` files (in any case) and
Buf's configuration files `buf.yaml`, `buf.work.yaml`, `buf.lock` and the
generation templates `buf.gen.yaml` and `buf.gen.*.yaml`. The kind of file is
part of the extraction cache key, as Buf's YAML files share an extension.

## Rationale

A `.proto` file's imports name files relative to import roots that only Buf's
configuration (or a build script) declares, and Buf's files name the Buf
Schema Registry modules and plugins a project depends on.

## Acceptance criteria

1. `api/v1/shop.proto`, `Legacy.PROTO`, `buf.yaml`, `proto/buf.yaml`,
   `buf.work.yaml`, `buf.lock`, `buf.gen.yaml` and `buf.gen.go.yaml` are
   claimed; `buf.yml`, `config.yaml` and `Cargo.lock` are not.
2. `buf.yaml`, `buf.work.yaml` and `buf.gen.yaml` with the same content do not
   share a cache key.
