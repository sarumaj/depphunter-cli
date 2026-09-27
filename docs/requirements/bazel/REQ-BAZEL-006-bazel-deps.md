---
id: REQ-BAZEL-006
uuid: 06662295-1d7b-4c33-88ac-26ea808559cd
title: Bazel modules named and pinned
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A `bazel_dep(name, version, repo_name, dev_dependency)` in MODULE.bazel (or
a segment it includes) **shall** be an import resolving to the module of the
`bazel` island (the Bazel Central Registry's name) at the version the lock
file selected (REQ-BAZEL-008, the declared one becoming Requested), else at
the declared version. A declared version **shall** pin: minimal version
selection is deterministic and a registry never changes a published version,
though another module may raise it (the lock says to what). No version
floats. Overrides **shall** apply: `single_version_override(version)` pins
that version; `git_override` names the module with the remote as Origin,
pinned by a commit, a tag shown and neither, a branch floating;
`archive_override` has the URL as Origin, the URL's ref as version, pinned
by its `integrity`; `local_path_override` is the directory's MODULE.bazel.

## Rationale

Modules are what a Bzlmod workspace depends on; the version rules follow how
Bazel selects them.

## Acceptance criteria

1. protobuf 27.0 with `single_version_override(version = "27.3")` is 27.3
   requested 27.0; abseil-cpp with a git_override commit is pinned at the
   commit with the remote as Origin; googletest's archive_override is
   v1.15.0 pinned by integrity; shop_api is api/MODULE.bazel;
   `bazel_dep(name = "rules_foo")` floats.
