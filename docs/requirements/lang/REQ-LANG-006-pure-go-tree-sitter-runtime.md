---
id: REQ-LANG-006
uuid: 5d017f12-c883-4d53-92de-926178105756
title: Pure-Go tree-sitter runtime
scope: lang
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §4
  - docs/REQUIREMENTS.md M2
verification:
  - inspection
---

## Statement

Plugins that parse with tree-sitter **shall** use the pure-Go runtime
gotreesitter, and the build **shall not** require cgo or WebAssembly for
parsing.

## Rationale

The binary must stay pure Go and cross-compile. A spike showed gotreesitter
meets this without maintaining a C-to-WASM toolchain, with 0 syntax errors on
574 CPython stdlib and 492 TypeScript files.

## Acceptance criteria

1. `CGO_ENABLED=0 go build ./cmd/depphunter` succeeds for every release target.
2. No tree-sitter plugin imports a cgo binding or a WASM runtime.

## Notes

The original plan (tree-sitter compiled to WASM run by `wazero`) was replaced in
M2. Trade-offs recorded: parsing is about 1-1.6 MB/s per core (about 20 times
slower than the C runtime) and the binary grows about 15 MB.
