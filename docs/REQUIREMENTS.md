# depphunter — Requirements

`depphunter` is a CLI that analyzes the project in the current directory and opens an
interactive, isometric "archipelago" map of its code base in the default web browser.

## 1. Goals

- Answer, within a minute on an unfamiliar repository: *what is here, how big is it,
  what depends on what, and what does it pull in from outside?*
- Let the user decide interactively **what** and **how much** to see (collapse / expand,
  focus, filter) without re-running the CLI.
- Work for any language ecosystem through a pluggable analysis layer.
- Ship as one pure-Go, cross-platform binary that works offline.

### Non-goals (for now)

- Precise symbol-level (call-graph) references — deferred to an optional LSP layer.
- Editing code, refactoring, or running builds.
- Hosting the view for remote users (the server is local-only by design).

## 2. The metaphor: an archipelago

| Code concept | Map element |
|---|---|
| Repository root | Mainland |
| Directory (expanded) | Terrace / plateau inside its parent |
| Directory (collapsed) | One district block; footprint ∝ file count, height ∝ mean file size |
| File | Building; footprint fixed, height = size (lines of code) |
| File (expanded) | Small plateau with one block per symbol (func, type, …) |
| External ecosystem (Go modules, npm, PyPI, …) | Separate island |
| External package / module | Building on its ecosystem island; height = number of importers |
| Import reference | Arc ("bridge") between the visible representatives of both ends |

Rules:

- Positions come from the hierarchy (stable, learnable), never from a force simulation.
- Edges are **never all drawn by default**. They appear for the focused node; a
  collapsed node shows the aggregate of its descendants' edges with counts.
- Every colour encodes exactly one thing, has a legend, and is never the only channel
  (tooltip, labels and side panel repeat the information).

## 3. Graph data model

One JSON document produced by analysis and consumed by the UI (and later by exports).

```jsonc
{
  "root": "depphunter-cli",
  "generatedAt": "2026-09-19T10:00:00Z",
  "nodes": [
    { "id": "d:.",                     "kind": "dir",       "name": "depphunter-cli", "path": "." },
    { "id": "d:internal",              "kind": "dir",       "name": "internal", "path": "internal", "parent": "d:." },
    { "id": "f:internal/x.go",         "kind": "file",      "name": "x.go", "path": "internal/x.go", "parent": "d:internal", "lang": "Go", "loc": 120 },
    { "id": "s:internal/x.go#Foo",     "kind": "symbol",    "name": "Foo", "symbolKind": "func", "line": 10, "parent": "f:internal/x.go" },
    { "id": "e:go",                    "kind": "ecosystem", "name": "Go modules" },
    { "id": "p:go:github.com/a/b",     "kind": "package",   "name": "github.com/a/b", "version": "v1.2.3", "parent": "e:go" }
  ],
  "edges": [
    { "from": "f:internal/x.go", "to": "p:go:github.com/a/b", "kind": "import", "line": 5 }
  ]
}
```

- Node kinds: `dir`, `file`, `symbol`, `ecosystem`, `package`.
- Edge kinds: `import` (v1). The model must accept `reference` (symbol → symbol) later
  without changes to the UI's aggregation logic.
- An edge target may be a file, a directory (e.g. a Go package), or a package node.

## 4. Language plugin contract

Each ecosystem is a plugin that:

1. Claims files (by extension / name).
2. Extracts, per file: imports (raw specifier + line) and top-level symbols (name, kind, line).
3. Resolves each import to a local path (file or directory), a standard-library package,
   or an external package — using the ecosystem's manifests and lockfiles
   (`go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`, `pom.xml`, `*.csproj`, …).
4. Declares the ecosystems it can emit (id + display name).

Parsing is done with tree-sitter grammars compiled to WASM and run via `wazero`
(pure Go, no cgo). A plugin may use a native parser instead when one exists in the Go
standard library (Go itself uses `go/parser`).

Files not claimed by any plugin still appear on the map (language detected by
extension, lines counted) but carry no edges.

## 5. Functional requirements

### CLI & configuration

- `depphunter [path]` — analyze `path` (default: current directory), serve, open browser.
- Precedence: flags > environment (`DEPPHUNTER_*`) > project config (`.depphunter.yaml`)
  > user config (`$XDG_CONFIG_HOME/depphunter/config.yaml`) > defaults.
- Respect `.gitignore` (via `git ls-files` when available; built-in ignore list otherwise)
  plus user `exclude` globs.
- UI settings apply live; a **Save** action writes them back to the project config (M4).

### Visualization & interaction

- Isometric orthographic camera; pan, zoom, rotate in 90° steps, free orbit optional,
  "fit" and "reset" actions.
- Collapse / expand at every level: directory → file → symbol. Expand/collapse all
  to a depth.
- Hover tooltip with name, path, language, size.
- Select → focus: dim everything except the node and its neighbourhood; draw its
  incoming and outgoing edges in distinct colours with counts.
- Side panel: syntax-highlighted source, symbol outline, lists of dependencies and
  dependents (each clickable); aggregate stats for directories and packages.
- Open in editor (`$EDITOR`, `vscode://`, `jetbrains://`) at the right line (M3).
- Fuzzy search and filters by language, path glob, ecosystem (M2).
- Colour-by: language (categorical) or size (sequential). Height scale: linear / sqrt / log.
- Light and dark themes (auto by OS, overridable).

### Outputs

- Serve the interactive view (default).
- Watch mode: file-system watcher, incremental re-analysis, updates pushed over
  WebSocket (M3).
- Graph export: JSON, DOT, GraphML (M3).
- Static self-contained HTML export (M4).

## 6. Non-functional requirements

- **Distribution:** single static binary for Linux, macOS, Windows; frontend embedded;
  no Node.js or network access at runtime.
- **Security:** bind to `127.0.0.1` by default; random per-run token exchanged for a
  cookie; reject requests with a foreign `Host` header (DNS rebinding); serve only files
  that are part of the analyzed graph.
- **Performance:** ~10k files render and navigate at 60 fps (instanced rendering);
  cold analysis < 5 s for 10k files; warm runs near-instant through a content-hash
  cache (M3).
- **Accessibility:** colour-blind-safe categorical palette, information never
  colour-only, keyboard shortcuts for navigation.

## 7. Milestones & acceptance criteria

### M1 — Walking skeleton

- Go plugin: imports via `go/parser`, symbols, multi-module `go.mod` resolution; std-lib
  and external modules become package nodes.
- All other files appear as buildings (language by extension, LOC).
- Local server with token + host check; `/api/graph`, `/api/file`, `/api/config`.
- Archipelago view: collapse/expand dirs and files, hover, select/focus with edges,
  side panel with highlighted source and dependency lists, colour-by and height-scale
  switches, theme support.
- Config precedence implemented and tested.

*Accepted when* running `depphunter` in this repository opens the map, and selecting
`internal/lang/golang` shows its edges to `go/parser` (std-lib island) and its dependents.

### M2 — Pluggable languages

- WASM tree-sitter runtime; plugins for JS/TS (incl. `tsconfig` paths) and Python.
- Symbol extraction through `tags.scm` queries.
- Fuzzy search and filters.

### M3 — Outputs & incrementality

- Content-hash analysis cache; watch mode with WebSocket updates.
- JSON / DOT / GraphML export; open-in-editor.

### M4 — Breadth

- Rust, Java, C# plugins; static HTML export; save UI settings to config.

### Later

- Git history overlay (churn, age, authors; time slider).
- Symbol-level references via LSP.
