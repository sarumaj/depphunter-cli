---
id: REQ-BAZEL-007
uuid: 1592f410-4f04-41ea-84b6-676798472d67
title: WORKSPACE repositories
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Repository rules called in WORKSPACE files, in any `.bzl` file of the
workspace (inside macros too, which declare most of a WORKSPACE project's
repositories; WORKSPACE declarations win) and through `use_repo_rule` in
MODULE.bazel **shall** be imports of the file declaring them: `http_archive`,
`http_file` and `http_jar` a package of the `bazel-repo` island named by its
URL (a mirror's first URL skipped; `lang.RepoName`; a GitHub, GitLab,
Codeberg or sourcehut archive or release by its repository with the ref as
version; `name-1.2.3.tar.gz` by directory and name), pinned by `sha256` or
`integrity` as CMake's URL_HASH pins, else by a commit ref, floating on a
branch archive or no ref; computed URLs name it by the repository name.
`git_repository` and `new_git_repository` are named by the remote (commit
pins, tag neither, branch floats); `go_repository` is the Go module of its
`importpath` at its version or commit; `local_repository` and
`new_local_repository` are the project directory. `maybe(rule, ...)` is the
rule. `workspace(name)` names the main repository.

## Rationale

WORKSPACE projects declare their external code this way.

## Acceptance criteria

1. rules_go's http_archive (mirror first) is `github.com/bazelbuild/rules_go`
   v0.50.1 pinned; zlib without a hash is `zlib.net/zlib` 1.3.1, not
   pinned; a `refs/heads/main` archive floats; `maybe(http_archive, ...)` in
   a macro is read; `go_repository` is `github.com/google/uuid` v1.6.0.
