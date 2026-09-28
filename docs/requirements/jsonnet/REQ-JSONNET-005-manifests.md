---
id: REQ-JSONNET-005
title: jsonnet-bundler manifests
scope: jsonnet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Each dependency of a `jsonnetfile.json` and each entry of a
`jsonnetfile.lock.json` (git sources and local directories) **shall** be an
import of its file. A git dependency **shall** be a package of the
`jsonnet-bundler` island named by its repository (lang.RepoName: host,
owner and repository, no scheme or `.git`) and subdirectory; its legacy
name **shall** be its `name` field, else the subdirectory's last element,
else the repository's. A project without a lock **shall** use the lock of
a project listing it as a local source, else an ancestor's.

## Rationale

jb names what it installs by repository and subdirectory; kube-prometheus
keeps its library as a nested project locked by the root.

## Acceptance criteria

1. The fixture's `jsonnetfile.json` and lock list their dependencies as
   imports; the nested `mixin/jsonnetfile.json` has no lock and its
   ksonnet-util is pinned by the root lock with `v1.0` requested.
