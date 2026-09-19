# depphunter — Requirements

`depphunter` is a CLI that analyzes the project in the current directory and
opens an interactive, isometric "archipelago" map of its code base in the
default web browser.

## 1. Goals

- Answer, within a minute on an unfamiliar repository: *what is here, how big is
  it, what depends on what, and what does it pull in from outside?*
- Let the user decide interactively **what** and **how much** to see (collapse /
  expand, focus, filter) without re-running the CLI.
- Work for any language ecosystem through a pluggable analysis layer.
- Ship as one pure-Go, cross-platform binary that works offline.

### Non-goals (for now)

- Precise symbol-level (call-graph) references — deferred to an optional LSP
  layer.
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

- Positions come from the hierarchy (stable, learnable), never from a force
  simulation.
- Islands ring the mainland, the most imported ecosystem nearest; a ring fills
  its least-full side first, and a new ring starts outside the previous one when
  no side has room (M7).
- Edges are **never all drawn by default**. They appear for the focused node; a
  collapsed node shows the aggregate of its descendants' edges with counts.
- Every colour encodes exactly one thing, has a legend, and is never the only
  channel (tooltip, labels and side panel repeat the information).

## 3. Graph data model

One JSON document produced by analysis and consumed by the UI (and later by
exports).

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
- An edge target may be a file, a directory (e.g. a Go package), or a package
  node.

## 4. Language plugin contract

Each ecosystem is a plugin that:

1. Claims files (by extension / name).
2. Extracts, per file: imports (raw specifier + line) and top-level symbols
   (name, kind, line).
3. Resolves each import to a local path (file or directory), a standard-library
   package, or an external package — using the ecosystem's manifests and
   lockfiles (`go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`,
   `pom.xml`, `*.csproj`, …).
4. Declares the ecosystems it can emit (id + display name).

Parsing uses tree-sitter through
[gotreesitter](https://github.com/odvcencio/gotreesitter), a pure-Go runtime
that loads the upstream grammars' parse tables (no cgo, no WASM). Only the
grammars a plugin imports are linked in. Plugins reach the runtime through
`internal/lang/treesitter`, so it can be swapped for a WASM/`wazero` build in
one place. A plugin may use another parser when that is more reliable or faster:
Go uses `go/parser`; C# and PowerShell use small statement scanners (see M4).

> **Decision (M2):** the original plan was tree-sitter compiled to WASM and run
> by `wazero`. A spike showed gotreesitter meets the same constraint (pure Go,
> cross-compiles) without maintaining a C→WASM toolchain: 0 syntax errors on 574
> CPython stdlib and 492 TypeScript files. Trade-offs: parsing is ~1–1.6 MB/s
> per core (≈20× slower than the C runtime, so parsing is parallel, bounded per
> file, and skips minified/oversized files), the binary grows ~15 MB, and the
> library is young and moves fast (the version is pinned).

Files not claimed by any plugin still appear on the map (language detected by
extension, lines counted) but carry no edges.

## 5. Functional requirements

### CLI & configuration

- `depphunter [path]` — analyze `path` (default: current directory), serve, open
  browser.
- Precedence: flags > environment (`DEPPHUNTER_*`) > project config
  (`.depphunter.yaml`) > user config
  (`$XDG_CONFIG_HOME/depphunter/config.yaml`) > defaults.
- Respect `.gitignore` (via `git ls-files` when available; built-in ignore list
  otherwise) plus user `exclude` globs.
- UI settings apply live; a **Save** action writes them back to the project
  config (M4).

### Visualization & interaction

- Isometric orthographic camera; pan, zoom, rotate in 90° steps, free orbit
  optional, "fit" and "reset" actions.
- Collapse / expand at every level: directory → file → symbol. Expand/collapse
  all to a depth.
- Hover tooltip with name, path, language, size.
- Select → focus: dim everything except the node and its neighbourhood; draw its
  incoming and outgoing edges in distinct colours with counts.
- Side panel: syntax-highlighted source, symbol outline, lists of dependencies
  and dependents (each clickable); aggregate stats for directories and packages.
- Open in editor at the right line: a server-side command template (configured
  or detected), falling back to the `vscode://` URL handler (M3).
- Fuzzy search and filters by language, path glob, ecosystem (M2).
- Colour-by: language (categorical), size or git history (sequential). Height
  scale: linear / sqrt / log.
- Light and dark themes (auto by OS, overridable).
- Walk mode: the same map in first person, on foot or flying (M7).

### Outputs

- Serve the interactive view (default).
- Watch mode: file-system watcher, incremental re-analysis, updates pushed to
  the browser (M3).
- Graph export: JSON, DOT, GraphML (M3).
- Static self-contained HTML export (M4).
- PNG image of the map as shown (M7).

## 6. Non-functional requirements

- **Distribution:** single static binary for Linux, macOS, Windows and FreeBSD
  (see M7 for the architectures); frontend embedded; no Node.js or network
  access at runtime.
- **Licensing:** BSD 3-Clause; vendored and linked libraries must carry
  permissive licenses compatible with it, and their notices ship with the
  source and the release archives.
- **Security:** bind to `127.0.0.1` by default; random per-run token exchanged
  for a cookie; reject requests with a foreign `Host` header (DNS rebinding);
  serve only files that are part of the analyzed graph.
- **Performance:** ~10k files render and navigate at 60 fps (instanced
  rendering); cold analysis < 5 s for 10k files; warm runs near-instant through
  a content-hash cache (M3).
- **Accessibility:** colour-blind-safe categorical palette, information never
  colour-only, keyboard shortcuts for navigation.

## 7. Milestones & acceptance criteria

### M1 — Walking skeleton

- Go plugin: imports via `go/parser`, symbols, multi-module `go.mod` resolution;
  std-lib and external modules become package nodes.
- All other files appear as buildings (language by extension, LOC).
- Local server with token + host check; `/api/graph`, `/api/file`,
  `/api/config`.
- Archipelago view: collapse/expand dirs and files, hover, select/focus with
  edges, side panel with highlighted source and dependency lists, colour-by and
  height-scale switches, theme support.
- Config precedence implemented and tested.

*Accepted when* running `depphunter` in this repository opens the map, and
selecting `internal/lang/golang` shows its edges to `go/parser` (std-lib island)
and its dependents.

### M2 — Pluggable languages

- Pure-Go tree-sitter runtime; plugins for JS/TS and Python.
- JS/TS: relative paths (incl. `.js` → `.ts`, `index.*`), `tsconfig`/`jsconfig`
  `paths` and `baseUrl` with relative `extends`, workspace packages, `node:` and
  built-in modules, `package.json` ranges pinned by `package-lock.json`;
  `require()` and dynamic `import()`.
- Python: relative imports, project roots (repository, `src/`, directories with
  `pyproject.toml`/`setup.py`/`setup.cfg`), a script's own directory, the
  standard library, distributions from requirements files, `pyproject.toml` (PEP
  621, dependency groups, Poetry) and `Pipfile`, pinned by
  `poetry.lock`/`uv.lock`/`pdm.lock`/`Pipfile.lock`, and well-known
  import→distribution aliases (`yaml` → PyYAML, …).
- Symbol extraction through tags-style queries (definitions; methods named
  `Class.method`).
- Fuzzy search (`/`) over files, directories, symbols and packages.
- Filters by language (also by clicking the legend), island, and path globs
  (`!glob` hides); packages only imported by hidden files are hidden with them.

*Accepted when* a mixed Go/TS/Python repository shows npm, PyPI and Go islands
with versions from lockfiles, and hiding a language removes its buildings and
the islands only it used, without recolouring the remaining languages.

### M3 — Outputs & incrementality

- Plugins split into `Extract` (content only, cached by SHA-256 of the content,
  plugin version and file extension) and a per-run `Resolver` (manifests,
  layout), so cached runs still resolve against the current manifests.
- Watch mode (`fsnotify`) on the analysed directories only; debounced
  re-analysis; the browser receives updates through **Server-Sent Events** and
  keeps expansion, selection, filters and language colours, highlighting changed
  files.
- JSON / GraphML / DOT export from the CLI (`--export`) and the UI.
- Open in editor via `POST /api/open` (cookie + `X-Depphunter-Request` header;
  files must be in the graph; arguments are never passed through a shell).

> **Decisions (M3):** SSE instead of WebSocket — updates flow one way, SSE needs
> no dependency and reconnects by itself. The project config may not set
> `editor`, since a cloned repository could otherwise choose the command
> depphunter executes.

*Accepted when* a warm run parses no unchanged file, editing a file in `--watch`
mode updates the open map within a second without losing the view state, and the
DOT export renders in Graphviz.

### M4 — Breadth

- Rust (tree-sitter): `use` trees expanded to paths; module files via `crate::`,
  `self::`, `super::` and `mod x;`; workspace, path, renamed and
  workspace-inherited dependencies; `Cargo.lock` versions.
- Java (tree-sitter): classes by package-path suffix under any source root; JDK
  split from non-JDK `javax`; Maven (properties, dependencyManagement), Gradle,
  version catalogs; groupId matching by prefix, shared segments, artifact
  aliases and known mismatches.
- C# (scanner): MSBuild root namespace + folder convention; `PackageReference`,
  `Directory.Packages.props`; case-insensitive package ids.
- PowerShell (scanner): `using module`, `Import-Module`, dot-sourcing, `&`,
  `$PSScriptRoot`, `#Requires -Modules`, module manifests; built-in module list.
- Static HTML export (`--export html`, Export menu): one file, ES modules as
  `data:` URLs behind an import map, graph + view settings + source text (256 KB
  per file, 24 MB total).
- **Save settings**: `POST /api/settings` writes the `ui:` section (including
  filters) of the project config via `yaml.Node`, preserving other keys and
  comments.

> **Decisions (M4):** tree-sitter was dropped for two grammars after measuring
> them. The pure-Go C# grammar needed 24 s for Serilog's 216 files (8 s for one
> 59 KB file); the PowerShell grammar turns `Import-Module -Name A, B` into an
> error node that swallows the following lines. Both languages' dependencies and
> declarations are statement-level, so scanners that understand comments,
> strings, here-strings and braces replace them (Serilog: 20 ms). Results on
> real projects: ripgrep 0 unresolved crates; gson 8 unresolved imports
> (generated and test-only code); Serilog 1; Pester 0.

*Accepted when* ripgrep, gson, Serilog and Pester analyse with (almost) no
unresolved dependencies, saved settings survive a reload, and the HTML export
opens from `file://`.

### M5 — Git history overlay

- `internal/history`: one `git log --numstat --no-merges --no-renames --relative
  -- .` pass (newest `history_commits`, default 10,000) into per-file changes
  `[time, author, added, deleted, commit]`; authors keyed by e-mail, shown by
  name; cached per HEAD next to the analysis cache.
- Read in the background after the map is served (`/api/history`: 202 while
  reading, 204 without history); an SSE `history` event reports it, and in watch
  mode the git directory is watched so a commit refreshes it.
- UI colour modes Commits, Lines changed, Last change and Authors, computed in
  the browser from the raw changes, so the **Since** slider needs no requests;
  districts use per-file means like size mode; "no commits in range" has its own
  neutral colour. Tooltip and side panel show the figures and top authors.
- The HTML export embeds the history.

> **Decisions (M5):** raw changes are sent instead of server-side aggregates so
> any time range can be evaluated instantly; the payload stays small (ripgrep:
> 2,223 commits read in 0.6 s). `--relative` alone still lists commits outside
> the analysed directory, so the pathspec `-- .` restricts them. Renames are not
> followed (`--follow` works for single files only).

*Accepted when* ripgrep's 2,000+ commits load while the map is already usable,
the four modes and the slider recolour without requests, and the HTML export
keeps the overlay.

### M6 — Symbol references and hardening

- `--lsp`: a JSON-RPC client (`internal/lsp`) drives installed language servers
  (gopls, typescript-language-server, pyright/basedpyright/pylsp, rust-analyzer,
  jdtls, csharp-ls). For each definition it asks `textDocument/references`; each
  hit is credited to the innermost enclosing definition, using
  `textDocument/documentSymbol` extents, or to the file for top-level code.
  Edges of kind `reference` are served like the history (`/api/references`),
  cached by the graph's content and the installed servers, and shown through an
  Imports / References switch.
- Git history follows renames (`-M`).
- Lockfiles: `yarn.lock` (classic and Berry, disambiguated by the declared
  range), `pnpm-lock.yaml` (per importer); Python `setup.cfg` and literal
  `setup.py` lists.
- The C# scanner walks interpolation holes of `$"…"` strings.
- CI on Linux, macOS and Windows, pinned to Go 1.22 with `GOTOOLCHAIN=local`;
  lint (gofmt, tidy, vet, staticcheck, markdownlint) and govulncheck; tagged
  releases build stripped binaries with the current Go.

> **Decisions (M6):** references are opt-in because they start external servers
> and take seconds to minutes (gopls: 533 definitions in 7.5 s here). Positions
> are sent in UTF-16 columns, the LSP default. Release builds use the current
> Go: govulncheck found 35 reachable standard-library issues when built with Go
> 1.22.2 and none with the current release, while `go.mod` keeps 1.22 as the
> oldest supported version.

### M7 — Libraries, sharing and walk mode

- Replace hand-rolled code with established libraries where one exists:
  `cli/browser` (open the browser), `emicklei/dot` (DOT export),
  `kballard/go-shellquote` (editor command templates), `sourcegraph/jsonrpc2`
  (LSP transport), `tidwall/jsonc` (`tsconfig`/`jsconfig` with comments),
  `golang.org/x/sync/errgroup` (bounded parallel parsing and scanning). In the
  UI: `potpack` (terrace packing) and `fzf-for-js` (fuzzy search).
- Map labels (`labels.js`): region names and, for a selection, the names at both
  ends of its arcs, placed greedily by priority without overlaps.
- **Export → PNG image** (`P`): the map as shown, labels included, at the
  screen's resolution; works in the static HTML export too.
- Toolbar menus (Filters, Export) open above the side panel and tooltip.
- Islands ring the mainland (see §2), so large dependency sets no longer produce
  one long row north of it.
- **Walk mode** (`V`): a first-person view of the same layout, bent onto a small
  planet whose radius the wheel or `[` `]` changes. `WASD`/arrows move and turn,
  `Shift` runs, `Space` jumps, `F` toggles flying (`C` sinks); collisions and
  ledges (up to half a storey) are computed on the flat layout. Newspapers
  (click or `Q`) select the building they hit, `Enter` selects the aimed box,
  `E`/right click expands or collapses it. The city look (sky, water, facades,
  roads, trees, lamps) is procedural shaders modulating the data colours, never
  replacing them; the isometric view is unchanged.
- License: BSD 3-Clause; vendored web libraries keep theirs (MIT, BSD 3-Clause,
  ISC) and are listed in `web/static/vendor/README.md`.
- Releases: `scripts/dist.sh` cross-compiles archives with checksums for
  Linux (amd64, arm64, armv7, 386, riscv64), macOS (amd64, arm64), Windows
  (amd64, arm64, 386) and FreeBSD (amd64, arm64), each with README, LICENSE and
  the vendored libraries' licenses. CI builds every target on each push, keeps
  the archives as workflow artifacts and runs the tests as 32-bit (386).

> **Decisions (M7):** a library replaces local code only when it is small, pure
> Go, permissively licensed and maintained; its behaviour is pinned by the
> existing tests (export, editor, LSP and resolver tests). The walker lives in
> flat layout coordinates and only the renderer bends the world, so picking,
> collisions and the isometric view share one layout.

*Accepted when* the release workflow publishes archives for every listed target,
walking the map selects and expands buildings like clicking does, and the PNG
export matches the view.

### M8 — A walkable city

- The view stays with the map. The isometric camera's target is clamped to the
  map's bounds plus a quarter of its size (at least 6 units), and zooming out
  stops when the map fills 35% of the view. The walker stays within 3 units of
  the outermost shore, flies at most 12 units above the tallest box, and the
  planet radius is capped at three times the map's diagonal.
- Streets are the free space of each terrace top, so they connect by
  construction: side streets in the gaps between children, a ring road along
  the terrace edge. Every obstacle (child footprint, terrace edge) gets a
  sidewalk with a curb; a straight street between two facing obstacles gets a
  dashed centre line, wheel tracks and manholes; long streets get zebra
  crossings where the facing obstacles end; free space farther than a street's
  width from any obstacle becomes a park with paths. Terrace sides carry stairs.
  Lamps stand on the sidewalk along each terrace edge.
- Facades pick a style per building — brick with framed windows and sills,
  concrete panels, or a glass curtain wall for tall buildings — with a door on
  the ground floor; roofs are gravel with a plant room and air-conditioning
  units or rows of solar panels; asphalt has grain, patches and cracks.

> **Decisions (M8):** street shading needs the nearest obstacles per fragment.
> A lookup grid (0.5-unit cells, coarser on huge maps, at most 2^20 cells) lists
> the 8 footprints nearest to each cell in a float texture, beside a texture of
> all footprints; the shader measures exact distances to those and to its own
> terrace's edges. Footprints a fragment lies inside (its own terrace, those
> below it) are skipped, so one 2D grid serves every terrace level. The grid is
> built only when walk mode is shown, and rebuilt after a relayout.

*Accepted when* panning, zooming, walking and flying cannot leave the map
behind, and walking between buildings follows connected, marked streets.

### Known limits

- Java imports name packages, not artifacts, so Maven dependencies are matched
  by heuristics; unmatched imports are shown as unresolved.
- Servers that index slowly (rust-analyzer, jdtls) may answer before indexing
  finishes and return fewer references within the time budget.
