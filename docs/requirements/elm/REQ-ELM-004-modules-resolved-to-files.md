---
id: REQ-ELM-004
uuid: 65e3164c-3f6a-4e49-b46f-e0278d5e93b8
title: Modules resolved to files
scope: elm
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Elm plugin **shall** resolve `import A.B` to `A/B.elm` under a source
directory of a project listing the importing file: an application's
`source-directories`, a package's `src/`, and the project's `tests/` for a
file under it (elm-test), which is searched first. When several `elm.json`
files list the file, the projects whose own directory holds it come first,
the deepest first, then the others by the longest source directory holding
it; a file no project lists belongs to the nearest `elm.json` above it. A
kernel module (`Elm.Kernel.List`) resolves to its `.js` file there.

## Rationale

Elm finds a module by its path under the source directories; examples and
test applications often list the library's `../src`.

## Acceptance criteria

1. `Shop.Cart` is `src/Shop/Cart.elm`, `Shared.Format` is
   `lib/shared/Shared/Format.elm` (a second source directory), `Helpers`
   from a test is `tests/Helpers.elm`.
2. `UiKit.Button` from `examples/Demo.elm` is
   `packages/ui-kit/src/UiKit/Button.elm` through `../packages/ui-kit/src`,
   and `Elm.Kernel.Kit` is `packages/ui-kit/src/Elm/Kernel/Kit.js`.
