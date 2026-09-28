---
id: REQ-JSONNET-010
title: Read without jsonnet or jb
scope: jsonnet
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Jsonnet projects **shall** be read without evaluating Jsonnet or running
jb: an import path computed at run time does not exist in Jsonnet, but
Tanka's per-environment library paths, `-J` flags given to a tool and
`JSONNET_PATH` entries outside the repository are not known, a legacy
name is matched only through the manifests or `vendor/`, and nothing is
asked of any server (no `--online`).

## Rationale

jsonnet-bundler has no registry, and the library path is a property of
the command line.

## Acceptance criteria

1. A missing `missing.libsonnet` is dropped and `mystery/x.libsonnet` is an
   unresolved package.
