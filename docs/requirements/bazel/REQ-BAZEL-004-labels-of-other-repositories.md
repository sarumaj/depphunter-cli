---
id: REQ-BAZEL-004
title: Labels of other repositories
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A label of another repository (`@repo//pkg:x`, `@repo`, a canonical
`@@repo+//...` or `@@repo~version//...`) **shall** resolve by what declares
the repository in the file's workspace (the nearest directory above it with
MODULE.bazel, WORKSPACE or REPO.bazel): a `bazel_dep` by its apparent name
(`repo_name`, else its name) or its module name, a hub repository
(REQ-BAZEL-009), a Go repository by Gazelle's name, a WORKSPACE repository
rule (REQ-BAZEL-007), a `local_path_override` or `local_repository`
directory of the project (then the file or package inside it), a repository
a module extension made (`use_repo`): the module whose `.bzl` defines the
extension, or the project's own extension file. A repository Bazel provides
is REQ-BAZEL-010's. Anything else **shall** be an unresolved package of the
bazel island named by the repository. A hub's generated `.bzl`
(`@pypi//:requirements.bzl`) resolves to the module whose extension made the
hub.

## Rationale

The repository name is all a label says about where its code comes from.

## Acceptance criteria

1. `@com_google_protobuf//:protobuf` (repo_name) is the protobuf module;
   `@@rules_go+//go/runfiles` is rules_go; `@abseil-cpp//absl/strings` in a
   vendored copy of another project (grpc's third_party/upb) finds the
   module declared as `repo_name = "com_google_absl"`.
2. `@shop_api//:api` (local_path_override) goes to api/BUILD.bazel,
   `@vendored//:lib` (local_repository) to its BUILD file, `@shop_tools//:bin`
   (use_repo of the project's extension) to tools/extensions.bzl,
   `@unknown_repo//:thing` is unresolved.
