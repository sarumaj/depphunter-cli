---
id: REQ-CMAKE-007
title: Fetched content
scope: cmake
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`FetchContent_Declare()`, `ExternalProject_Add()` and CPM.cmake's
`CPMAddPackage()`, `CPMFindPackage()` and `CPMDeclarePackage()` (the
shorthand `gh:`/`gl:`/`bb:owner/repo[@version][#tag]` or a URL, and the
keyword form) **shall** be packages of the `cmake-fetch` island named by
their URL (`lang.RepoName`: no scheme, user or `.git`, host in lower case);
a GitHub or GitLab archive or release asset **shall** be named by its
repository with the ref as its version. A git `GIT_TAG` that is a full
commit pins; a branch name (main, master, develop, trunk, HEAD, `origin/*`
and the like) or no tag floats; any other tag is shown and neither pins nor
floats, since a tag can be moved. A download pins with a `URL_HASH` or
`URL_MD5` (or an archive of a commit) and floats without a version. As in
CPM, a `VERSION` without a `GIT_TAG` fetches the tag `v<VERSION>`. A local
path is the project file or the directory's `CMakeLists.txt`, as is a
`SOURCE_DIR` without another source. `FetchContent_MakeAvailable()`,
`FetchContent_Populate(name)` and `CPMGetPackage()` **shall** resolve to the
content declared under that name (the same file first, then a directory
above). A declaration inside a function body whose source uses the
function's parameters **shall not** be recorded.

## Rationale

Fetched content is a build's dependency manager when there is none; its
pinning follows the rule of Terraform's and GitHub Actions' git references
(REQ-CI-011).

## Acceptance criteria

1. `GIT_TAG f8d7d77c...` pins `github.com/google/googletest`, `GIT_TAG
   v3.5.2` shows v3.5.2 unpinned and not floating, `GIT_TAG origin/main` and
   no tag float.
2. A release asset with `URL_HASH SHA256=...` is `github.com/nlohmann/json`
   v3.11.3, pinned.
3. `CPMAddPackage("gh:TartanLlama/expected@1.1.0")` is
   `github.com/TartanLlama/expected` v1.1.0; `FetchContent_MakeAvailable(json)`
   in `cmake/Deps.cmake` resolves to the declaration's package.
