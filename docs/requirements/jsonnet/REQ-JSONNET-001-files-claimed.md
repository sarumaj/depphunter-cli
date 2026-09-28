---
id: REQ-JSONNET-001
title: Files claimed
scope: jsonnet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Jsonnet plugin **shall** claim Jsonnet files (`.jsonnet`,
`.libsonnet`) and jsonnet-bundler's manifest and lock file
(`jsonnetfile.json`, `jsonnetfile.lock.json`), the two told apart from
other JSON files by their names. Nothing in a `vendor` directory beside a
`jsonnetfile.json` (what `jb install` wrote) **shall** be claimed; another
`vendor` directory **shall** be.

## Rationale

jb installs dependencies into the project; their files are the packages',
not the project's, and are read only to resolve imports (REQ-JSONNET-004).

## Acceptance criteria

1. The fixture's Jsonnet files and manifests are claimed; its
   `vendor/github.com/...` files are not, while a `vendor/` directory with
   no `jsonnetfile.json` beside it is kept.
