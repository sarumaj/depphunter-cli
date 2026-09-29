---
id: REQ-SUP-071
title: A Wally registry's package files for package dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a package of the
`wally` island depends on from its registry, a GitHub repository read as files
(`raw.githubusercontent.com/<owner>/<repository>/HEAD/...`): the package file
`<scope>/<name>` holds one JSON manifest per line, one line per version. The
version the target names **shall** be taken, else the newest one its
requirement admits (a bare version is a caret range, as in wally.toml;
pre-releases left out). The manifest's `dependencies` and
`server-dependencies` **shall** be returned as written (`scope/name` with the
requirement after `@`), `dev-dependencies` left out, each carrying the registry
the manifest names (lang.Target.Registry), which alone serves it. A registry
without the package **shall** pass the question to the `fallback_registries` of
its `config.json` (read once), as Wally does.

github.com/UpliftGames/wally-index is the public index. A wally.toml's
`[package] registry` **shall** be recorded as the repository's source (known
when it is the public one). A dependency's registry that is neither the public
one nor vouched for is not asked (REQ-SUP-043).

## Rationale

wally.lock records the graph when it is committed; without it the registry is
the only place a package's dependencies are written.

## Acceptance criteria

1. sleitnick/knit at `1.2.0` takes 1.7.0 (not the 2.0.0 pre-release) and
   yields sleitnick/comm, evaera/promise and the server dependency, without
   the dev dependency; `1.6.0` yields its own.
2. A package the public registry lacks is found in the fallback its
   config.json lists, and its dependencies carry that registry.
3. A dependency whose registry nobody vouches for is not asked.

## Notes

Verified against the live public registry (its `config.json` and package files
such as `sleitnick/knit`, reachable through the sandbox proxy) and with a stub
server. A registry that is not on GitHub is not read (it would have to be
cloned); the Wally API (api.wally.run) serves package archives, not
dependencies. A private GitHub registry is read with whatever this machine
holds for raw.githubusercontent.com (a netrc entry).
