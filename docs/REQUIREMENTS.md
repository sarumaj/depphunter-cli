# depphunter — Requirements

`depphunter` is a CLI that analyzes the project in the current directory and opens
an interactive, isometric "archipelago" map of its code base in the default web
browser.

## 1. Goals

- Answer, within a minute on an unfamiliar repository: *what is here, how big is
  it, what depends on what, and what does it pull in from outside?*
- Let the user decide interactively **what** and **how much** to see (collapse /
  expand, focus, filter) without re-running the CLI.
- Work for any language ecosystem through a pluggable analysis layer.
- Ship as one pure-Go, cross-platform binary that works offline.

### Non-goals (for now)

- Precise symbol-level (call-graph) references — deferred to an optional LSP layer.
- Editing code, refactoring, or running builds.
- Hosting the view for remote users (the server is local-only by design).

## 2. The metaphor: an archipelago

| Code concept                                  | Map element                                                         |
|-----------------------------------------------|---------------------------------------------------------------------|
| Repository root                               | Mainland                                                            |
| Directory (expanded)                          | Terrace / plateau inside its parent                                 |
| Directory (collapsed)                         | One district block; footprint ∝ file count, height ∝ mean file size |
| File                                          | Building; footprint fixed, height = size (lines of code)            |
| File (expanded)                               | Small plateau with one block per symbol (func, type, …)             |
| External ecosystem (Go modules, npm, PyPI, …) | Separate island                                                     |
| External package / module                     | Building on its ecosystem island; height = number of importers      |
| Import reference                              | Arc ("bridge") between the visible representatives of both ends     |

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
- Edge kinds: `import` (v1). The model must accept `reference` (symbol → symbol)
  later without changes to the UI's aggregation logic.
- An edge target may be a file, a directory (e.g. a Go package), or a package node.

## 4. Language plugin contract

Each ecosystem is a plugin that:

1. Claims files (by extension / name).
2. Extracts, per file: imports (raw specifier + line) and top-level symbols
   (name, kind, line).
3. Resolves each import to a local path (file or directory), a standard-library
   package, or an external package — using the ecosystem's manifests and lockfiles
   (`go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`, `pom.xml`,
   `*.csproj`, …).
4. Declares the ecosystems it can emit (id + display name).

Parsing uses tree-sitter through [gotreesitter](https://github.com/odvcencio/gotreesitter),
a pure-Go runtime that loads the upstream grammars' parse tables (no cgo, no WASM). Only
the grammars a plugin imports are linked in. Plugins reach the runtime through
`internal/lang/treesitter`, so it can be swapped for a WASM/`wazero` build in one place.
A plugin may use another parser when that is more reliable or faster: Go uses
`go/parser`; C# and PowerShell use small statement scanners (see M4).

> **Decision (M2):** the original plan was tree-sitter compiled to WASM and run by `wazero`.
> A spike showed gotreesitter meets the same constraint (pure Go, cross-compiles) without
> maintaining a C→WASM toolchain: 0 syntax errors on 574 CPython stdlib and 492 TypeScript
> files. Trade-offs: parsing is ~1–1.6 MB/s per core (≈20× slower than the C runtime, so
> parsing is parallel, bounded per file, and skips minified/oversized files), the binary
> grows ~15 MB, and the library is young and moves fast (the version is pinned).

Files not claimed by any plugin still appear on the map (language detected by
extension, lines counted) but carry no edges.

## 5. Functional requirements

### CLI & configuration

- `depphunter [path]` — analyze `path` (default: current directory), serve, open
  browser.
- Precedence: flags > environment (`DEPPHUNTER_*`) > project config (`.depphunter.yaml`)
  > user config (`$XDG_CONFIG_HOME/depphunter/config.yaml`) > defaults.
- Respect `.gitignore` (via `git ls-files` when available; built-in ignore list
  otherwise) plus user `exclude` globs.
- UI settings apply live; a **Save** action writes them back to the project
  config (M4).

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
- Open in editor at the right line: a server-side command template (configured or
  detected), falling back to the `vscode://` URL handler (M3).
- Fuzzy search and filters by language, path glob, ecosystem (M2).
- Colour-by: language (categorical) or size (sequential). Height scale: linear
  / sqrt / log.
- Light and dark themes (auto by OS, overridable).

### Outputs

- Serve the interactive view (default).
- Watch mode: file-system watcher, incremental re-analysis, updates pushed to the
  browser (M3).
- Graph export: JSON, DOT, GraphML (M3).
- Static self-contained HTML export (M4).

## 6. Non-functional requirements

- **Distribution:** single static binary for Linux, macOS, Windows; frontend embedded;
  no Node.js or network access at runtime.
- **Security:** bind to `127.0.0.1` by default; random per-run token exchanged for
  a cookie; reject requests with a foreign `Host` header (DNS rebinding); serve
  only files that are part of the analyzed graph.
- **Performance:** ~10k files render and navigate at 60 fps (instanced rendering);
  cold analysis < 5 s for 10k files; warm runs near-instant through a content-hash
  cache (M3).
- **Accessibility:** colour-blind-safe categorical palette, information never
  colour-only, keyboard shortcuts for navigation.

## 7. Milestones & acceptance criteria

### M1 — Walking skeleton

- Go plugin: imports via `go/parser`, symbols, multi-module `go.mod` resolution;
  std-lib and external modules become package nodes.
- All other files appear as buildings (language by extension, LOC).
- Local server with token + host check; `/api/graph`, `/api/file`, `/api/config`.
- Archipelago view: collapse/expand dirs and files, hover, select/focus with edges,
  side panel with highlighted source and dependency lists, colour-by and height-scale
  switches, theme support.
- Config precedence implemented and tested.

*Accepted when* running `depphunter` in this repository opens the map, and selecting
`internal/lang/golang` shows its edges to `go/parser` (std-lib island) and its dependents.

### M2 — Pluggable languages

- Pure-Go tree-sitter runtime; plugins for JS/TS and Python.
- JS/TS: relative paths (incl. `.js` → `.ts`, `index.*`), `tsconfig`/`jsconfig` `paths` and
  `baseUrl` with relative `extends`, workspace packages, `node:` and built-in modules,
  `package.json` ranges pinned by `package-lock.json`; `require()` and dynamic `import()`.
- Python: relative imports, project roots (repository, `src/`, directories with
  `pyproject.toml`/`setup.py`/`setup.cfg`), a script's own directory, the standard library,
  distributions from requirements files, `pyproject.toml` (PEP 621, dependency groups,
  Poetry) and `Pipfile`, pinned by `poetry.lock`/`uv.lock`/`pdm.lock`/`Pipfile.lock`, and
  well-known import→distribution aliases (`yaml` → PyYAML, …).
- Symbol extraction through tags-style queries (definitions; methods named `Class.method`).
- Fuzzy search (`/`) over files, directories, symbols and packages.
- Filters by language (also by clicking the legend), island, and path globs (`!glob` hides);
  packages only imported by hidden files are hidden with them.

*Accepted when* a mixed Go/TS/Python repository shows npm, PyPI and Go islands with
versions from lockfiles, and hiding a language removes its buildings and the islands only
it used, without recolouring the remaining languages.

### M3 — Outputs & incrementality

- Plugins split into `Extract` (content only, cached by SHA-256 of the content, plugin
  version and file extension) and a per-run `Resolver` (manifests, layout), so cached
  runs still resolve against the current manifests.
- Watch mode (`fsnotify`) on the analysed directories only; debounced re-analysis; the
  browser receives updates through **Server-Sent Events** and keeps expansion, selection,
  filters and language colours, highlighting changed files.
- JSON / GraphML / DOT export from the CLI (`--export`) and the UI.
- Open in editor via `POST /api/open` (cookie + `X-Depphunter-Request` header; files must
  be in the graph; arguments are never passed through a shell).

> **Decisions (M3):** SSE instead of WebSocket — updates flow one way, SSE needs no
> dependency and reconnects by itself. The project config may not set `editor`, since a
> cloned repository could otherwise choose the command depphunter executes.

*Accepted when* a warm run parses no unchanged file, editing a file in `--watch` mode
updates the open map within a second without losing the view state, and the DOT export
renders in Graphviz.

### M4 — Breadth

- Rust (tree-sitter): `use` trees expanded to paths; module files via `crate::`, `self::`,
  `super::` and `mod x;`; workspace, path, renamed and workspace-inherited dependencies;
  `Cargo.lock` versions.
- Java (tree-sitter): classes by package-path suffix under any source root; JDK split from
  non-JDK `javax`; Maven (properties, dependencyManagement), Gradle, version catalogs;
  groupId matching by prefix, shared segments, artifact aliases and known mismatches.
- C# (scanner): MSBuild root namespace + folder convention; `PackageReference`,
  `Directory.Packages.props`; case-insensitive package ids.
- PowerShell (scanner): `using module`, `Import-Module`, dot-sourcing, `&`, `$PSScriptRoot`,
  `#Requires -Modules`, module manifests; built-in module list.
- Static HTML export (`--export html`, Export menu): one file, ES modules as `data:` URLs
  behind an import map, graph + view settings + source text (256 KB per file, 24 MB total).
- **Save view**: `POST /api/settings` writes the `ui:` section (including filters) of the
  project config via `yaml.Node`, preserving other keys and comments.

> **Decisions (M4):** tree-sitter was dropped for two grammars after measuring them.
> The pure-Go C# grammar needed 24 s for Serilog's 216 files (8 s for one 59 KB file);
> the PowerShell grammar turns `Import-Module -Name A, B` into an error node that
> swallows the following lines. Both languages' dependencies and declarations are
> statement-level, so scanners that understand comments, strings, here-strings and
> braces replace them (Serilog: 20 ms). Results on real projects: ripgrep 0 unresolved
> crates; gson 8 unresolved imports (generated and test-only code); Serilog 1; Pester 0.

*Accepted when* ripgrep, gson, Serilog and Pester analyse with (almost) no unresolved
dependencies, a saved view survives a reload, and the HTML export opens from `file://`.

### Later

- Git history overlay (churn, age, authors; time slider).
- Symbol-level references via LSP.
