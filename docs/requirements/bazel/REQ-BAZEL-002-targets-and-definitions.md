---
id: REQ-BAZEL-002
uuid: 665c2d6a-cb4b-4844-88f1-c09a0294f0f3
title: Targets and definitions extracted
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A BUILD file's top-level calls that carry a `name` string **shall** be
symbols `//<dir>:<name>` whose kind is the rule or macro called
(`cc_library`, `go_library`, `py_binary`, a project macro); `package`,
`licenses`, `exports_files` and `load` declare none. A `.bzl` file's
top-level functions **shall** be symbols of kind `func`, and its top-level
assignments symbols named by the function that made them (`rule`, `macro`,
`repository_rule`, `module_extension`, `provider`, `aspect`, `tag_class`,
`transition`, `struct`), else `var`. `module(name)` in MODULE.bazel is a
`module` symbol and `workspace(name)` a `workspace` symbol. The language
server's word for `//pkg:name` **shall** be `name`.

## Rationale

Targets are what BUILD files declare and other packages depend on.

## Acceptance criteria

1. The fixture's src/BUILD.bazel has `//src:shop` (cc_library),
   `//src:tool` (py_binary) and `//src:generated` (the project macro
   `shop_macro`); tools/defs.bzl has `shop_rule` (rule), `ShopInfo`
   (provider), `shop_macro` (func) and `VERSION` (var).

## Notes

The package is the BUILD file's directory relative to the scanned root, not
to its workspace: a nested workspace's targets carry the nested path.
