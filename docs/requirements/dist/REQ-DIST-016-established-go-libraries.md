---
id: REQ-DIST-016
uuid: 8350cc9b-6fbf-4b59-9b6f-9b4a575a031b
title: Established Go libraries
scope: dist
type: constraint
priority: must
status: implemented
verification:
  - unit
  - inspection
---

## Statement

The system **shall** use established libraries instead of local code for:
opening the browser (`github.com/cli/browser`), writing DOT
(`github.com/emicklei/dot`), splitting editor command templates
(`github.com/kballard/go-shellquote`), the language server transport
(`github.com/sourcegraph/jsonrpc2`), reading `tsconfig`/`jsconfig` with comments
(`github.com/tidwall/jsonc`), and bounded parallel parsing and scanning
(`golang.org/x/sync/errgroup`). A library **shall** replace local code only when
it is small, pure Go, permissively licensed and maintained.

## Rationale

Maintained libraries carry less risk than hand-rolled code; the conditions keep
the binary pure Go and the license compatible. Their behavior is pinned by the
existing export, editor, LSP and resolver tests.

## Acceptance criteria

1. `go.mod` requires each listed library.
2. Each listed purpose is served through its library.
3. The export, editor, LSP and resolver tests pass.
