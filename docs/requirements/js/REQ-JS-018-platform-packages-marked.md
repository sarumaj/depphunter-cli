---
id: REQ-JS-018
title: Platform packages marked
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A package that a lock file says installs on some platforms only **shall** stay
in the graph, with its `depends` edges, and its package node **shall** carry
`platform`, the platforms it installs on written as Yarn Berry writes them:
`os=linux & cpu=x64`. The plugin **shall** read that from a Berry `yarn.lock`
entry's `conditions`, and from the `os`, `cpu` and `libc` lists that
`package-lock.json` (and `npm-shrinkwrap.json`) v2 and v3, `pnpm-lock.yaml`
and `bun.lock` copy from the package's manifest, a list of several values being
`(os=darwin | os=linux)` and a negated value `!os=win32`. The side panel
**shall** show it as "installs on".

## Rationale

A package such as esbuild ships its binary as one optional dependency per
platform, and every lock file lists all of them whatever platform wrote it.
Dropping them would lose the binary this platform does install; drawing them
unmarked makes each read as a dependency of every install.

## Acceptance criteria

1. In each lock tree fixture that installs it (npm v3, Berry, pnpm v5, v6 and
   v9), `fsevents`, declared `os: [darwin]` or `conditions: os=darwin`, is a
   dependency of `b` whose node carries `platform: os=darwin`; no other
   package carries one.
2. A Berry `yarn.lock` whose esbuild entry depends on `@esbuild/linux-x64`
   (`conditions: os=linux & cpu=x64`) and `@esbuild/win32-x64` keeps both
   edges, each with its conditions.
3. In `bun.lock`, `{ "os": "darwin", "cpu": "arm64" }` gives
   `os=darwin & cpu=arm64` and `{ "os": ["linux"], "cpu": ["x64"], "libc":
   ["glibc"] }` gives `os=linux & cpu=x64 & libc=glibc`.
4. The lists `["darwin", "linux"]` and `["!win32"]` give
   `(os=darwin | os=linux)` and `!os=win32`.
5. A package node with a `platform` shows it in the side panel as "installs
   on".

## Notes

The platform is the package's, not the edge's: one node stands for every copy
of a name, and a package's platforms do not change from one dependent to the
next. Nothing is filtered by the platform depphunter runs on: the map is of the
repository, not of one machine's install.
