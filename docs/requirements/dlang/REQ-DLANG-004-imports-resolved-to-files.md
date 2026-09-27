---
id: REQ-DLANG-004
uuid: 645ae498-2c6b-445e-a6ee-20dc5193ebf2
title: Imports resolved to files
scope: dlang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The D plugin **shall** resolve `import a.b.c` to the project file
`a/b/c.d`, `a/b/c.di`, `a/b/c/package.d` or `a/b/c/package.di` under, in
order: the import directories of the file's dub package - `importPaths`
and `sourcePaths`, else `source/` or `src/` where they exist - and of the
repository's packages it depends on (its root package, its sub-packages,
path dependencies, transitively); then the file's own directory and its
ancestors. A file of no dub package also looks in `source/`, `src/` and
`import/` of the repository. A module importing itself is dropped.
`import("file")` **shall** resolve under the package's
`stringImportPaths` (`views/` by default), else beside the file, and be
dropped when missing.

## Rationale

The compiler finds a module by its name under the import paths dub passes;
without dub, a module's name starts at an ancestor directory (Phobos'
`std/` at the repository root, druntime's `core/` under `src/`).

## Acceptance criteria

1. `shop.models` resolves to `source/shop/models/package.d`,
   `widgets.button` (a path dependency) to
   `libs/widgets/source/widgets/button.d`, `commands` from the root package
   to the inline sub-package's `cli/commands.d`, and `shop.cart` from
   `tools/source/tool.d` through its path dependency on `..`.
2. `import("config.json")` in `tools/` resolves to `tools/res/config.json`
   (its `stringImportPaths`); `import("missing.txt")` is dropped.
