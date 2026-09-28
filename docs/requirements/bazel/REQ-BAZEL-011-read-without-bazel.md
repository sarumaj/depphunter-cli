---
id: REQ-BAZEL-011
title: Bazel read without running Bazel
scope: bazel
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Bazel's files **shall** be read without running Bazel: macros are not
expanded (a target a macro declares is the macro call; label strings a macro
turns into labels, such as grpc's `external_deps = ["absl/base"]`, are
dropped), `select()` branches are all read, computed labels and URLs are not
evaluated, module extensions other than the listed hubs are not run (their
repositories go to the extension's module), and a `.bazelrc` is read only
for its registries (REQ-SUP-057) and its credential helpers' scopes. A
credential helper (`--credential_helper=[<scope>=]<helper>`) is a program
and **shall not** be run: a registry the most specific helper covers
(`<host>`, `*.<domain>` for the domain and the hosts below it, none for
every host) is asked without the helper's credentials, and the report
**shall** say so with a `helper-not-run` note
([REQ-TRC-017](../trc/REQ-TRC-017-resolver-notes.md)). A BUILD target's
package is its directory from the scanned root, not from a nested workspace.
Only a lock file says which version minimal version selection picked;
without one a module is shown at its declared minimum. Buck2 and other
Starlark dialects are not read. OSV has no Bazel ecosystem, so modules and
WORKSPACE downloads are not asked about by name and version (a
`git_override` or `git_repository` at a full commit on a public forge is
asked about by that commit, REQ-FND-026); `--online` follows modules through
registries (REQ-SUP-057), not WORKSPACE repositories.

## Rationale

The map is built from files alone.

## Acceptance criteria

1. A label that names a repository no file of the workspace declares is an
   unresolved `bazel` package, not a guess.
2. `registry.corp.test=...` covers that host, `*.corp.test=...` covers
   `corp.test` and `a.b.corp.test` but not `xcorp.test`, an unscoped helper
   every host, `--credential_helper_timeout` is no helper; a repository's
   helpers are forgotten with it; a registry a helper covers is noted
   whether answered by it or from the cache, one no helper covers is not.
