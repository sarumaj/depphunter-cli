---
id: REQ-LSP-002
uuid: e7a78549-1eeb-4bd7-b888-cb250d7f1223
title: Language server client for installed servers
scope: lsp
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

With `--lsp`, the system **shall** drive, as a JSON-RPC client over standard
input and output (sourcegraph/jsonrpc2), each installed language server for the
files it covers: gopls (Go), typescript-language-server (TypeScript,
JavaScript), pyright, basedpyright or pylsp (Python, first installed),
rust-analyzer (Rust), jdtls (Java) and csharp-ls (C#), skipping a server that is
not installed.

## Rationale

Language servers are the one source of precise symbol-level usage for every
ecosystem, and they are already installed where the ecosystem is used.

## Acceptance criteria

1. With gopls installed, a Go project yields reference edges and `servers` lists
   `gopls`.
2. A server that is not on `PATH` (or in the Go binary directories, for gopls)
   is logged as skipped and the run continues.
