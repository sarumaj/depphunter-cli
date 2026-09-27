---
id: REQ-BAZEL-003
title: Labels to project files
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`load()` labels and the labels of a target's label attributes (`srcs`,
`hdrs`, `textual_hdrs`, `data`, `deps`, `runtime_deps`,
`implementation_deps`, `exports`, `proto`, `embed`, `plugins`,
`resources`, `main`, `actual`, `tools`, `tests`, `src` and any attribute
ending in `deps`, `_srcs` or `_hdrs`) **shall** resolve, when they name the
main repository (`//pkg:x`, `:x`, a bare file name, `@//`, `@@//`, or the
module's or workspace's own name), to the file the label names in its
package, else to the BUILD file of the package (a rule, or a file a rule
generates), else to a directory. Both branches of a conditional, every
`select()` branch and both sides of a `+` of lists are read; make variables
and flags are not labels. A label into the file's own package is dropped. In
MODULE.bazel and WORKSPACE files, the project files extensions and
repository rules read (`lock_file`, `requirements_lock`, `go_mod`,
`pnpm_lock`, `cargo_lockfile`, `manifests`, `maven_install_json`), the
extensions' `.bzl` files, `include()`d segments and registered toolchains
**shall** be linked the same way; `Label("...")` in a `.bzl` file too.

## Rationale

These are the edges a Bazel build follows inside the repository.

## Acceptance criteria

1. `//lib:helpers` and `@shop_repo//lib:lib` go to lib/BUILD,
   `@//lib:helpers.cc` to lib/helpers.cc, `:util` from its own package is
   dropped, `load(":local.bzl", ...)` goes to src/local.bzl.
2. MODULE.bazel links maven_install.json, requirements_lock.txt, go.mod,
   web/pnpm-lock.yaml, Cargo.lock, Cargo.toml, deps.MODULE.bazel,
   tools/extensions.bzl and tools/BUILD.bazel (the toolchain).
