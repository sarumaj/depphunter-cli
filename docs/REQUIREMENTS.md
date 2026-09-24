# depphunter - Requirements

`depphunter` is a command-line tool that analyses the project in the current
directory and opens an interactive isometric "archipelago" map of its code base
in the default web browser.

## 1. Goals

- Answer, within a minute on an unfamiliar repository: what is present, how
  large it is, what depends on what, and what it draws in from outside.
- Allow the user to determine interactively **what** and **how much** is shown —
  collapsing, expanding, focusing and filtering — without re-running the tool.
- Support any language ecosystem through a pluggable analysis layer.
- Ship as a single pure-Go, cross-platform binary that operates offline.

### Non-goals, at present

- Precise symbol-level call-graph references, which are deferred to an optional
  LSP layer.
- Editing code, refactoring, or running builds.
- Serving the view to remote users; the server is local-only by design.

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
- Every color encodes exactly one quantity, has a legend, and is never the sole
  channel: the tooltip, the labels and the side panel repeat the information.

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
    { "id": "p:go:github.com/a/b",     "kind": "package",   "name": "github.com/a/b", "version": "v1.2.3", "parent": "e:go" },
    { "id": "p:npm:react",             "kind": "package",   "name": "react", "version": "18.3.1", "requested": "^18.2.0", "parent": "e:npm" },
    { "id": "p:npm:chalk",             "kind": "package",   "name": "chalk", "version": "^5.3.0", "floating": true, "parent": "e:npm" }
  ],
  "edges": [
    { "from": "f:internal/x.go", "to": "p:go:github.com/a/b", "kind": "import", "line": 5 }
  ]
}
```

- Node kinds: `dir`, `file`, `symbol`, `ecosystem`, `package`.
- A package node's `version` is what the project resolves to, `requested` the
  specifier a manifest asked for when a lock file replaced it, and `floating`
  marks a dependency nothing fixes to one version (§ M12).
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
   package, or an external package - using the ecosystem's manifests and
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

- `depphunter [path]` - analyze `path` (default: current directory), serve, open
  browser.
- Precedence: flags > environment (`DEPPHUNTER_*`) > project config
  (`.depphunter.yaml`) > user config
  (`$XDG_CONFIG_HOME/depphunter/config.yaml`) > defaults, resolved with viper
  behind a cobra command (M9).
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
- Select → focus: dim everything except the node and its neighborhood; draw its
  incoming and outgoing edges in distinct colors with counts.
- Side panel: syntax-highlighted source, symbol outline, lists of dependencies
  and dependents (each clickable); aggregate stats for directories and packages.
- Open in editor at the right line: a server-side command template (configured
  or detected), falling back to the `vscode://` URL handler (M3).
- Fuzzy search and filters by language, path glob, ecosystem (M2).
- color-by: language (categorical), size or git history (sequential). Height
  scale: linear / sqrt / log.
- Light and dark themes (auto by OS, overridable).
- Walk mode: the same map in first person, on foot or flying (M7).

### Outputs

- Serve the interactive view (default).
- Watch mode: file-system watcher, incremental re-analysis, updates pushed to
  the browser (M3).
- Graph export: JSON, DOT, GraphML (M3).
- The log goes to stdout, so that what depphunter says can be piped and read
  like any other output; it steps aside to stderr only where stdout is already
  carrying an export with no file to go to. Errors go to stderr whatever the
  log is doing.
- Resolution report: how the dependencies on the map were arrived at, written to
  the log with `--explain` and served as JSON, Markdown or text (M18).
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
- **Accessibility:** color-blind-safe categorical palette, information never
  color-only, keyboard shortcuts for navigation.

## 7. Milestones & acceptance criteria

### M1 - Walking skeleton

- Go plugin: imports via `go/parser`, symbols, multi-module `go.mod` resolution;
  std-lib and external modules become package nodes.
- All other files appear as buildings (language by extension, LOC).
- Local server with token + host check; `/api/graph`, `/api/file`,
  `/api/config`.
- Archipelago view: collapse/expand dirs and files, hover, select/focus with
  edges, side panel with highlighted source and dependency lists, color-by and
  height-scale switches, theme support.
- Config precedence implemented and tested.

*Accepted when* running `depphunter` in this repository opens the map, and
selecting `internal/lang/golang` shows its edges to `go/parser` (std-lib island)
and its dependents.

### M2 - Pluggable languages

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
the islands only it used, without recoloring the remaining languages.

### M3 - Outputs & incrementality

- Plugins split into `Extract` (content only, cached by SHA-256 of the content,
  plugin version and file extension) and a per-run `Resolver` (manifests,
  layout), so cached runs still resolve against the current manifests.
- Watch mode (`fsnotify`) on the analyzed directories only; debounced
  re-analysis; the browser receives updates through **Server-Sent Events** and
  keeps expansion, selection, filters and language colors, highlighting changed
  files.
- JSON / GraphML / DOT export from the CLI (`--export`) and the UI.
- Open in editor via `POST /api/open` (cookie + `X-Depphunter-Request` header;
  files must be in the graph; arguments are never passed through a shell).

> **Decisions (M3):** SSE instead of WebSocket - updates flow one way, SSE needs
> no dependency and reconnects by itself. The project config may not set
> `editor`, since a cloned repository could otherwise choose the command
> depphunter executes.

*Accepted when* a warm run parses no unchanged file, editing a file in `--watch`
mode updates the open map within a second without losing the view state, and the
DOT export renders in Graphviz.

### M4 - Breadth

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

*Accepted when* ripgrep, gson, Serilog and Pester analyze with (almost) no
unresolved dependencies, saved settings survive a reload, and the HTML export
opens from `file://`.

### M5 - Git history overlay

- `internal/history`: one `git log --numstat --no-merges --no-renames --relative
  -- .` pass (newest `history_commits`, default 10,000) into per-file changes
  `[time, author, added, deleted, commit]`; authors keyed by e-mail, shown by
  name; cached per HEAD next to the analysis cache.
- Read in the background after the map is served (`/api/history`: 202 while
  reading, 204 without history); an SSE `history` event reports it, and in watch
  mode the git directory is watched so a commit refreshes it.
- UI color modes Commits, Lines changed, Last change and Authors, computed in
  the browser from the raw changes, so the **Since** slider needs no requests;
  districts use per-file means like size mode; "no commits in range" has its own
  neutral color. Tooltip and side panel show the figures and top authors.
- The HTML export embeds the history.

> **Decisions (M5):** raw changes are sent instead of server-side aggregates so
> any time range can be evaluated instantly; the payload stays small (ripgrep:
> 2,223 commits read in 0.6 s). `--relative` alone still lists commits outside
> the analyzed directory, so the pathspec `-- .` restricts them. Renames are not
> followed (`--follow` works for single files only).

*Accepted when* ripgrep's 2,000+ commits load while the map is already usable,
the four modes and the slider recolor without requests, and the HTML export
keeps the overlay.

### M6 - Symbol references and hardening

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

### M7 - Libraries, sharing and walk mode

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
  ledges (up to half a storey) are computed on the flat layout. Darts (click or
  `Q`; newspapers before M10) select the building they hit, a second dart in a
  tagged building (or `Enter`) shows its details,
  `E` expands or collapses it. The city look (sky, water, facades,
  roads, trees, lamps) is procedural shaders modulating the data colors, never
  replacing them; the isometric view is unchanged.
- License: BSD 3-Clause; vendored web libraries keep theirs (MIT, BSD 3-Clause,
  ISC) and are listed in `web/static/vendor/README.md`.
- Releases: `scripts/dist.sh` cross-compiles archives with checksums for
  Linux (amd64, arm64, armv7, 386, riscv64), macOS (amd64, arm64), Windows
  (amd64, arm64, 386) and FreeBSD (amd64, arm64), each with README, LICENSE and
  the vendored libraries' licenses. CI builds every target on each push, keeps
  the archives as workflow artifacts and runs the tests as 32-bit (386).

> **Decisions (M7):** a library replaces local code only when it is small, pure
> Go, permissively licensed and maintained; its behavior is pinned by the
> existing tests (export, editor, LSP and resolver tests). The walker lives in
> flat layout coordinates and only the renderer bends the world, so picking,
> collisions and the isometric view share one layout.

*Accepted when* the release workflow publishes archives for every listed target,
walking the map selects and expands buildings like clicking does, and the PNG
export matches the view.

### M8 - A walkable city

- The view stays with the map. The isometric camera's target is clamped to the
  map's bounds plus a quarter of its size (at least 6 units), and zooming out
  stops when the map fills 35% of the view. The walker stays within 3 units of
  the outermost shore, flies at most 12 units above the tallest box, and the
  planet radius is capped at three times the map's diagonal.
- Streets are the free space of each terrace top, so they connect by
  construction: side streets in the gaps between children, a ring road along
  the terrace edge. Every obstacle (child footprint, terrace edge) gets a
  sidewalk with a curb; a straight street between two facing obstacles gets a
  dashed center line, wheel tracks and manholes; long streets get zebra
  crossings where the facing obstacles end; free space farther than a street's
  width from any obstacle becomes a park with paths. Terrace sides carry stairs.
  Lamps stand on the sidewalk along each terrace edge.
- Facades pick a style per building - brick with framed windows and sills,
  concrete panels, or a glass curtain wall for tall buildings - with a door on
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

### M9 - Command line on cobra and viper

- The command is a `cobra.Command` (`cmd/depphunter`): POSIX flags via pflag
  (`--flag value`, `--flag=value`, `-o` as the short form of `--output`),
  generated `--help` with a description and examples, `-v`/`--version`, at most
  one path argument. Errors are printed once, without the usage text.
- `internal/config` declares the flags (`RegisterFlags`) and resolves settings
  with viper, keeping the precedence of §5: defaults, user config, project
  config (or `--config`), environment, flags. Files are merged key by key, so a
  project config overrides single `ui:` keys of the user config.
- Every scalar setting but `ui.path_filter` has an environment variable
  (`DEPPHUNTER_` + upper-case key, `ui.` dropped): new are `MAX_FILE_SIZE`,
  `EXPAND_DEPTH`, `HISTORY_COMMITS` and `LSP_TIMEOUT`. Invalid values fail
  with an error.
- Unchanged: `--no-open`/`--no-cache`/`--no-history` turn their settings off;
  `exclude` globs from files, `DEPPHUNTER_EXCLUDE` and `--exclude` add up; the
  project config cannot set `editor`.
- Dependency updates: Renovate (`renovate.json`, recommended preset, non-major
  updates grouped) opens pull requests; the oldest-Go CI job (the version in
  `go.mod`) rejects an update that needs a newer Go.

> **Decisions (M9):** the project file is read into a map and merged with
> `MergeConfigMap` after its `editor` key is dropped, rather than overriding the
> editor afterwards, so the environment and flags still win over the user's
> file. Viper's own environment binding replaces the hand-written parser; each
> variable is bound by name to keep the established short names (`THEME`, not
> `UI_THEME`). Mapstructure is at least v2.4.0, for GO-2025-3787 and
> GO-2025-3900. Single-dash
> long flags (`-addr`), which the standard `flag` package accepted, are no
> longer valid.
>
> The dependency refresh after M9 (viper 1.21, `golang.org/x/*` of 2026) needs
> Go 1.26 or newer, so Go 1.22 support ended; `go.mod` states Go 1.27.1. CI's
> oldest-Go job and the lint job read the version from `go.mod`, and
> staticcheck moved to 2026.2.1, which must be built with a Go at least as new
> as the code's.

*Accepted when* every flag, file key and environment variable keeps its effect
and precedence (the config tests), and `--help`, `--version` and exports work
from the command (the command tests).

### M10 - The dependency hunt

- Theme: the walker hunts dependencies. Tracking darts (click or `Q`) fly a
  shallow arc to the aimed point, or ahead under gravity when nothing is aimed
  at, pointing along their path. A hit tags the module: it is selected (its
  dependency trails light up), counted once in the HUD ("modules tagged"), and
  marked by an orange beacon (a beam and a diamond above it) for the session.
  The crosshair is a scope reticle.
- Ramps connect street levels: every nested terrace gets one ramp along the
  side with the most room, in the street beside it, from that street up to the
  terrace's top (at most 2.4 units, starting at a corner). The walker's height
  follows the ramp; its roadway has edge lines and uphill chevrons, its outer
  side a parapet. The top RAMP_LANDING (0.35) is level and closed by a barrier;
  from it a driveway crosses the terrace's sidewalk into its ring road (no lamp
  stands there), and at the foot an apron replaces the street's curb, so both
  ends join the carriageways. Stairs sit at the other end of each terrace side.
- Vegetation is geometry, not texture: bushes (clusters of blobs) along shores
  and in parks, three tree species (broadleaf, conifer, poplar) with per-vertex
  shading darker towards the base, colors varied per plant. Lawns are one green
  with gentle variation and mowing stripes in parks; the dark blotches are gone.
  Parks are sampled where the street shader draws lawn, off the gravel paths
  (at most 60,000 samples per map).
- Walk-mode interaction, first-person-shooter style: entering walk mode
  captures the pointer at the reticle (pointer lock; `Esc` frees it, a click on
  the map captures it again); holding the right button looks through a scope
  (field of view 70° to 22°, eased, with look sensitivity scaled to match). The
  right button no longer expands or collapses: collapsing a directory folds its
  buildings into a district block, which looked like buildings vanishing; that
  stays on `E`. The first pointer movement after locking and implausible jumps
  (250 px or more) are ignored; the shore and the block
  underfoot are never aimed at (no tint, tooltip, selection or collapse); on
  foot, water stops the walker; fog thins with altitude; the key list folds
  away after the first moves; jumping to a directory stands the walker on its
  block instead of beside it.
- The selection outline sits on its box (it floated half a box too high since
  M7); in walk mode it is hidden behind nearer geometry.
- Walk mode hands the walker a tool, drawn in front of the camera as a hand and
  what it holds, and the gesture of using it is animated rather than implied: a
  rod loads and casts, a net sweeps across the view, a camera's shutter kicks
  back, a bubble wand waves, a dart gun recoils. `T` takes out the next one and
  the choice is saved with the rest of the view (`ui.tool`).
- A tool carries what it throws (a bobber trailing its line, a spinning hoop, a
  wobbling bubble, a dart that points along its flight - or nothing at all, for
  the camera, whose photograph arrives the moment it is taken), what the HUD
  calls its tally, and its own aim helper: the crosshair belongs to the dart,
  not to every tool.
- Reading the details of what was just hit holds the view still. The panel takes
  the pointer, and a freed cursor steering the same scene as a reticle fixed in
  the center is two controls fighting over one view: while the panel is open the
  walker does not move, look, aim or fire, and the scene keeps rendering so what
  is being read about stays on screen. Enter or a click on the map takes the
  pointer back and walks on.

> **Decisions (M10):** ramps run along a terrace's side rather than across the
> street: streets are 0.35-0.55 units wide, and climbing a 0.28-unit terrace in
> that distance would be a wall, not a road. Ramps are computed once per layout
> (`rampsFor`, cached per boxes array) and shared by the renderer and the
> walker's collisions.

- The city look is not walk-mode only: the isometric map draws the same facades,
  roofs, streets, lawns, trees, bushes, lamps and ramps (walk mode adds sky,
  water and the planet's curve). Detail fades by pixel footprint when zoomed
  out. Data colors still read: street and lawn shading is tinted by the ratio
  of a box's color to its kind's usual one (so nesting levels, hover and
  flashes show), and boxes dimmed by a selection or the legend are drawn plain,
  without windows (a per-box fade flag next to the color). The space around the
  islands is sea: a water plane just above the land's base, 40 map sizes
  across (more than zooming out or panning can reveal), with walk mode's ripple
  shader, which fades ripples to their mean where they get smaller than a
  pixel.
- Walk-mode controls follow first-person-shooter habits: the mouse looks, the
  left button fires, the right one scopes, the wheel zooms (30° to 90°, the
  scope 22°), `[` `]` set the planet's curvature, `E` opens or closes what the
  reticle is on. Expanding and collapsing is not offered in walk mode at all
  (it rebuilds the whole city around the walker); `E`, `Q` and `Enter` are
  swallowed so the map's own shortcuts do not fire under a walker. The help
  dialog and the search free the pointer; closing them captures it again where
  the browser allows it. What a dart tagged is reported in the HUD, which a
  walker can read, not in the status corner.
- Curvature is bound by the character typed (`+`, `-`, or `[`, `]`), not by the
  key's place on the board: on a German keyboard the key at `BracketRight`
  types `+`, which made `+` curve the planet while `-` still changed the map's
  depth.
- A relayout (a depth change, a filter, a live update) keeps the walker in
  place: they are put back at the same distance from the same edge of the block
  they stand on, stepping aside if a building now occupies it.
- Every island is reachable on foot: bridges span the water from the mainland
  outwards, one per island, forming a spanning tree over the shores (each
  island joins the nearest shore already reachable). A bridge is an arched deck
  0.8 wide with railings, piers every 1.8 and a marked carriageway; the
  walker's height follows the arch.
- Flying follows the view: `W`/`S` move along the direction looked at (look
  down and press `W` to dive), `A`/`D` strafe level, `Space`/`C` add straight up
  and down. On foot, movement stays level.
- The hunt takes two shots: the first dart tags a building, a second dart into a
  building already tagged shows its details (dependencies, source), which is
  where a shooter's hands already are; `Enter` does the same for the aimed box.
  Either frees the pointer for reading; a click on the map, or closing the
  panel, captures it again. The panel never opens by itself while walking: it covers
  the reticle and cannot be reached with the pointer locked.
- Walk mode reads its buttons from mouse events, not pointer events: pressing a
  second button while one is held fires `pointermove`, not `pointerdown`, so
  firing while scoped never arrived.
- Browsing stays responsive on large repositories: search is debounced (120 ms),
  labels are laid out a few times a second while walking rather than every
  frame (both the map's renderer and the walker's own frames go through that
  throttle), the walk-mode ground buffer is only recolored while walking, and
  darts in flight are dropped when a relayout invalidates their targets.
- Recoloring writes only the boxes whose color or fade changed, and repaints
  just those boxes' vertex ranges of the walk-mode ground, whose buffer holds
  hundreds of thousands of values: pointing at a building must not cost a frame.
  Colors are CSS strings, parsed once per color per pass rather than once per
  box and vertex.
- The walker's ground height comes from a spatial grid of boxes, ramps and
  bridge decks, not from scanning every ramp of the map five times per step.
- A live update keeps the side panel where the reader left it instead of
  scrolling back to the top.
- Dependency lists, breadcrumbs, the legend and the filter shortcuts are
  reachable and operable by keyboard (`Tab`, `Enter`/`Space`), and focusing a
  legend entry isolates its language like hovering does.
- The HTML export from the browser carries the view on screen (`ui` query
  parameter, validated server-side); the CLI's `--export html` keeps using the
  configured view.
- When depphunter stops, the page gives up reconnecting after four attempts and
  says so (clicking retries), instead of an endless stream of failed requests.

*Accepted when* a walker can drive up every nested block by its ramp from one
street onto the other without crossing a curb, a dart tags what the reticle is
on, and the right button only zooms.

### ~~M11 - The map in the terminal~~ (**dropped**)

`--terminal` would have drawn the map in the terminal the command was started
from, for machines reached over SSH: a headless Chromium driven over the
DevTools protocol, its frames painted with the terminal's own graphics (kitty,
iTerm2, sixel, half blocks) and its input forwarded back as events.

> **Decisions (M11):** a text browser was never an option - the map is one WebGL
> canvas with no DOM underneath - so the choice was between shipping a second,
> poorer renderer and carrying a real one's pixels to the terminal. The pixels
> would have won: one map, one set of behaviors, and every terminal gets the same
> map its browser would draw. What sank it is the cost of the other end - a
> dependency on an installed Chromium, for a view that is opt-in and useless
> without it.

### M12 - The supply chain

- Every external package says whether anything fixes it to one version. A lock
  file, an exact specifier, a single-version range, a commit or a digest pins it;
  a range, a wildcard, a snapshot or a moving tag does not, and the package is
  drawn apart, badged in the side panel, and carried as `floating` into the JSON
  and GraphML exports. Where a lock file resolved a range, both are kept: 4.3.1
  `requested` as ^4.2.0.
- What counts as a pin is the ecosystem's own rule, not a shared guess: a version
  in `go.mod` is the one the build selects, while the same string in `Cargo.toml`
  means a caret range and only `Cargo.lock` decides; npm reads "1.2" as 1.2.x;
  Maven pins a plain version whatever its shape (1.2.3.RELEASE) but not a
  `-SNAPSHOT`, which is republished under its own name; NuGet moves on wildcards
  and ranges; PowerShell's `RequiredVersion` names one version where
  `ModuleVersion` is only a minimum.

- Continuous integration is a dependency source of its own: GitHub workflows and
  composite actions (`uses:` per step, reusable workflows, `container:`,
  `services:`, `docker://`, `runs.image`), GitLab pipelines (`include:` as
  `local`, `project`, `template`, `remote` and `component`, and the includes a
  bridge job triggers), and the container images either platform runs, in the
  three islands GitHub Actions, GitLab CI and Container images. A `./path`
  resolves to the file inside this repository, so a local action chains on to
  its own dependencies; jobs become their file's symbols.
- Pinning is stricter for a CI reference than for a package: only an immutable
  one counts, which means a commit or an OCI digest. A tag can be moved and an
  image tag republished, so `@v4` and `:1.25.3` float; a template, a remote
  include and an unversioned reference float although they name no version at
  all, which the target carries as its own flag rather than an invented version.
  Where a commit is documented by the version in a trailing comment - the
  hardening convention - that version is kept as what was requested.

- `--resolve-depth` adds what external packages themselves depend on, level by
  level (`-1` for as far as the answer reaches), from the lock files the
  repository carries: `package-lock.json` v1-v3, `pnpm-lock.yaml` v5-v9,
  classic `yarn.lock`, `Cargo.lock`, and `uv.lock` / `poetry.lock` / `pdm.lock`,
  each of which writes its edges differently. Nothing is fetched. A resolver
  offers this through an optional `lang.Transitive` interface, so an ecosystem
  that cannot answer offline simply does not implement it.
- A package the walk adds is marked `transitive` - no file in the project
  imports it - and package-to-package edges are a kind of their own (`depends`),
  so the count of files importing a package stays a count of files.

- Every external package carries the index it resolves from, read from the
  configuration this machine holds and the configuration the repository carries
  (npm, Yarn, pip, Poetry, uv, NuGet, Maven, Cargo, `GOPROXY`; a container
  reference names its own registry). An index only the repository names is
  marked: nothing here vouches for it, which is what dependency confusion looks
  like, and nothing is fetched from it.
- `--online` allows asking the trusted indexes for what the repository does not
  record - a Go module's own go.mod from the proxy, an npm version document, a
  distribution's requires-dist, a crate's line in the sparse index, a NuGet
  package's nuspec, and the base image an OCI image was built on - which is what
  lets `--resolve-depth` reach ecosystems whose graph is not in the repository.
  An image is followed through its manifest and config blob only, for the base
  name in its annotations or labels; no layers are fetched, and a registry's
  pull-token challenge is answered only where the realm is HTTPS or the
  registry's own host. Maven cannot be asked at all: a POM needs a group and an
  artifact, and a package on the map is a group. The version travels with each
  dependency the index names, since without it the level below cannot be asked
  for at all: a module proxy serves a go.mod for a version, not for a module.
  Lock files are asked first, one level of the walk is asked at once rather than
  one package at a time, answers are cached for a day, and credentials come from
  the user's own npm tokens and netrc and go only to the host they were written
  for.

- The side panel lists dependencies and dependents as trees rather than flat
  lists: a row opens into what that node depends on in turn, without fetching
  anything, since the edges are already in the model. Open branches survive a
  live update, the arrow keys open and close a row beside the pointer, and a
  node already open further up its own branch is shown once more, marked, and
  left closed - lock files contain cycles, and a tree that followed one would
  not end.

*Accepted when* a repository whose dependencies are locked shows no floating
packages, removing its lock file makes every one of them floating, a workflow
pinned to tags shows every action floating until the tags are replaced by
commits, `--resolve-depth 1` adds exactly the packages the lock file names as
the direct dependencies' own, a repository whose .npmrc names an index this
machine does not know has every npm package marked, and a dependency cycle can
be opened down to its repeat and no further.

### M13 - Findings, and the bugs that carry them

- depphunter runs no scanner. Reports the user names with `--findings` (a
  repeatable flag taking globs) are read and placed on the map: govulncheck's
  JSON stream, npm audit (7+ and the older advisory table), Trivy
  (vulnerabilities, misconfigurations and secret hits), osv-scanner,
  golangci-lint and eslint. The format is recognized from the report's own
  shape, not from its file name, since pipelines name them anything.
- With `--online`, the OSV database is asked about every external package the
  map pins to a version, for the ecosystems it covers (Go, npm, PyPI,
  crates.io, Maven, NuGet, GitHub Actions). One batched query covers the whole
  dependency tree and only the advisories it matched are fetched; answers are
  cached for six hours. A floating package is not asked about: it resolves to
  something else on the next install. Nothing leaves the machine without
  `--online`, and `--no-vulns` turns reports and database off together.
- A report is not a fatal input: one that will not parse, or a database that
  will not answer, marks the set partial and leaves the rest of the map
  standing. A path a report gives is made relative to the repository, and one
  that points outside it loses its path rather than its finding - the finding
  is still about a package.
- Severity is one scale (critical, high, medium, low, info) taken from the
  advisory's CVSS v3 vector where there is one, because the word a distribution
  chose often disagrees with it. A linter's "error" is capped at medium: it is
  not the same news as a critical advisory, and the streets would otherwise
  fill with bugs that mean a missing comment.
- A finding is placed where it belongs: a vulnerability on the package it
  affects, a linter's complaint on the file it is about, and both where a
  scanner could say which file reaches the vulnerable code. A directory carries
  the worst of everything below it, as a badge beside its name. The findings are
  a background dataset like history and references (`/api/findings`), computed
  after the map is served and refreshed by `--watch`.
- The side panel lists them worst first, each row opening in place for the
  description, the fixed version and the advisory link.
- In walk mode every finding is a bug patrolling the building it belongs to,
  colored by its severity, and the crosshair names the one it is on. Catching
  one with the current tool displays what it carried, exactly
  as a second hit on a building opens its details; a thrown projectile follows
  a bug that walks on while it flies, and catches any bug it passes through.
  The HUD counts what is left. A finding whose building is not drawn (a
  collapsed directory) puts its bug on the nearest one that is.

*Accepted when* a report from each supported tool is recognized without being
named, a Trivy finding whose CVSS vector disagrees with its severity word takes
the vector's, a repository with no reports and no `--online` asks nothing and
shows nothing, and catching a bug in walk mode opens the same finding the side
panel lists for its building.

### M14 - Three styles, and a tool worth looking at

- The map is dressed by a style (`--style`, `ui.style`): a city, a printed
  circuit board, or a galaxy. All three share every measurement and every piece
  of geometry - the same layout, the same street network, the same ramps and
  bridges - and differ only in which painter answers "what surface is this?"
  (one `uStyle` uniform) and in which props stand on it. A style is a look, not
  a second renderer, and switching costs one prop rebuild.
- The colors that carry data do not change with the style. Languages, the
  history overlays, hover, selection and dimming read the same in all three;
  what the style owns is the environment - the ground, the water, the sky - which
  it takes from the stylesheet like everything else, so both themes still choose
  their own values.
- The board: chip packages with rows of pins and a printed part number, a
  heatsink where a building is tall, copper traces down the middle of every
  street with vias along them, a hatched ground pour, solder pads inside a
  silkscreen outline around every part, capacitors and resistors where the trees
  and bushes were, LEDs where the lamps were, and the board's own layered edge.
- The galaxy: crystal spires, faceted and dark at the foot, with strata of light
  across them and windows that read as stars; glowing conduits down every gap;
  dust, rubble and beacons; nebulae and a star field instead of sea and sky.
- The walker's hands and forearms are a model, not geometry assembled in the UI:
  tools/hand.py builds one in Blender - a skeleton of joints with a radius each,
  grown into flesh by the Skin modifier, smoothed, and rigged with a bone per
  phalanx - and exports web/static/hand.glb. The script is the source and is
  committed with it, so the model can be read, reviewed and regenerated rather
  than a binary that cannot be modified. No model is downloaded: a rigged hand
  that can be posed per tool and redistributed under this repository's license is
  not something there is to fetch.
- It is loaded once and cloned for every hand drawn, and it is posed by bone
  name: the fist closes around whatever is held, the knuckle giving least and the
  middle joint most. The names are the contract between the Blender script and
  the UI.
- What the walker holds is the only lit thing on the map. The walk camera carries
  a key, a fill and some ambient light, and because every material in the scene
  proper is unlit they reach nothing else - so a hand is round and a rod blank has
  a highlight, while the city keeps its flat, data-first coloring.
- A tool is held rather than placed beside a hand: each one says which way the
  shaft through the fist points and which way the arm runs back out of the frame,
  and the hand is oriented from those two directions. The tool is a child of the
  hand, so the two move together.
  shoulder with it. The
  tools are built to the same standard: a rod with a cork grip, a tapering blank,
  line guides and a reel with a handle; a net with a bound grip, a ferrule and a
  bag with stiffening rings; a camera with a prism hump, a focus ring, a hood and
  a shutter button, held in both hands; a wand with a bottle cap and a soap film
  that thins and lets go; a launcher with a scope, an air cylinder and a dart
  riding up after each shot.
- A gesture has anticipation, a strike and a follow-through, not a ramp, and the
  hand grips harder through it. Standing still the tool breathes; walking, it
  rises and falls and swings across with the walker's weight.
- While the details panel holds the pointer the crosshair is not recomputed, so
  whatever it was last on has its hover card cleared: a card left on top of what
  is being read is worse than no card.
- Walk mode carries a tracker: a top-down sweep centered on the walker and turning
  with them, with every bug still on the streets as a dot in its severity's
  color, every tagged module as a ring, and anything beyond its range as an
  arrow on the rim. Its range fits whatever is still out there and eases rather
  than jumping, and under it is the distance to the nearest bug and what it
  carries; without it, a bug cannot be found on a map of a thousand files.

*Accepted when* the same repository in all three styles keeps the same layout and
the same language colors, a board's streets carry copper where a city's carry
asphalt, opening a building's details in walk mode leaves no hover card on the
panel, and the tracker points at a bug on the far side of the map.

### M15 - One interface, in the editor as well as in the map

- The extension's own corner of the activity bar holds a **dependency tree** and
  a **backpack** beside the list of maps, both reading the map that was opened
  last. The tree is the graph document itself: a directory opens into what it
  holds, a file into what it imports, an island into its packages and a package
  into what it depends on, with no request per row, since every edge arrived with
  the graph. A branch that reaches something already open on it is shown once
  more, marked, and left closed.
- The selection and the catch are the server's while the map is open
  (`GET /api/session`, `POST /api/selection`, `PUT /api/backpack`), announced on
  the same event stream as everything else. A row picked in the panel selects
  that building on the map; a building picked on the map opens the tree to its
  row. Each change names the client that made it, so a client can tell its own
  change returning from another client's.
- The backpack's lasting store stays the browser's, per repository: it has to
  outlive a server that runs only while the map is open. The page uploads it
  up as it loads, and a finding dropped in the panel is dropped from the map too.
- The catch can be written out as Markdown, CSV or JSON, and the graph in its
  own formats, from the editor as well as from the page; and the map can be
  opened in the browser outside the editor for the one time that is wanted,
  without changing where it opens by default.

*Accepted when* clicking a package in the panel selects the same package on the
map, catching a bug in walk mode makes it appear in the panel's backpack, taking
it out of the panel takes it out of the map's, and a package that depends on
something that depends back on it can be opened down to its repeat and no
further.

### M16 - What the organization owns

- Most of an enterprise repository is internal, and two things depphunter does for
  a public package must not be done for an internal one: naming it to that
  ecosystem's public index (which does not answer, and says the package exists),
  and asking the vulnerability database about it (which hands its name and version
  to a third party). Neither is inferable - a module path on a company host looks
  like any other - so `--private` / `private:` declares them, with GOPRIVATE's
  glob-prefix meaning and an optional ecosystem prefix (`npm:@acme/*`). GOPRIVATE
  and GONOPROXY are read on top, so a Go project that has configured its machine
  needs no configuration here.
- A package so matched carries `private` on the graph, is drawn with a badge, and
  is skipped by both the index client and the OSV query. It is still asked of an
  index this machine's own configuration names: a company registry knows about it
  already. A repository may declare its own packages private - the effect is only
  that depphunter says less, and the repository is who would know.
- `--trust-index` / `trust_indexes:` vouches for an index that appears only in the
  repository, which otherwise carries the ⚠ marking and is never fetched from. It
  is read from the user's own config and the command line only: a repository that
  could clear its own warning would leave no guard at all, which is the whole
  point of the marking.
- Credentials are read from the files and variables an enterprise keeps them in
  (internal/auth): the four per-registry forms of an `.npmrc`; netrc; the
  `<servers>` of `~/.m2/settings.xml`, matched to the mirror or repository naming
  them; `<packageSourceCredentials>` matched to its `<packageSources>` entry; the
  stored `auths` of a container registry configuration and the helpers it names;
  a Cargo token, matched to its index through the registry's name; and a
  credential written into an index URL. `${NAME}`, `${env.NAME}` and `%NAME%` are
  expanded; an encrypted password is left alone rather than sent as ciphertext.
  Each credential goes to the host it was written for and to no other, matched
  with the port and then without it, since a registry reached on a port has a
  credential of its own.
- A credential written into an index URL is removed from that URL before the URL
  is recorded. The index a package resolves from reaches the graph, the side
  panel and every export, and the HTML export is a file the documentation
  recommends sharing. Only this machine's own configuration contributes a
  credential from a URL; the repository's is stripped and discarded.
- A container registry that names a credential helper rather than storing a
  credential is asked through that helper, as `docker login` is:
  `docker-credential-<name> get` with the registry on standard input. It is the
  only program executed that was not named on the command line, so the name must
  be a bare name, is resolved on `PATH` alone, and is bounded by a timeout; an
  identity token is declined, since only a registry accepts one.

*Accepted when* a run with `--private 'corp.example/*'` names no corp.example
package to a public proxy and sends none of them to osv.dev, those
packages are still resolved from a registry the machine configures, a project
config naming `trust_indexes` changes nothing, and GOPRIVATE alone is enough to
mark a Go repository's internal modules.

### M17 - Asking again without being answered again

- `/api/graph` carries an `ETag` and honours `If-None-Match`. The tag is the
  snapshot's fingerprint, which is of the nodes and the edges and not of when
  they were read, so a re-analysis that found the same project is the same
  entity. `Cache-Control` is `no-cache` rather than `no-store`: a validator is
  no use to a client that was told not to keep the document.
- Every announcement on the event stream carries an `id`, and the greeting says
  the current sequence and whether the client's `Last-Event-ID` is still it.
  A client that did not resume may have slept through an announcement - nothing
  is announced twice - and asks again; the ETag makes that free when the answer
  is the document it already holds. The first greeting of a connection's life is
  not a reconnection and asks nothing.
- The graph document's TypeScript declaration is generated from the Go one and
  checked by the ordinary test run (`go test ./internal/graph -update` rewrites
  it). A hand copy of a struct is a field added on one side and quietly not read
  on the other, which is not a compile error anywhere.

*Accepted when* loading the map makes exactly one request for the graph, a
reconnection that missed nothing makes none, a reconnection that missed something
is answered 304 where the graph did not change, and adding a field to graph.Node
without regenerating fails the tests by name.

### M18 - Saying how it resolved

- Dependency resolution leaves no mark on the map it produces, and two of its
  questions are answered off screen. Which index a package resolves from decides
  whether naming it discloses anything and whether it may be fetched at all; how
  far the walk past the direct dependencies got decides what the map contains.
  Both are invisible in the result: a package that resolves from a company Nexus
  is drawn exactly like one from registry.npmjs.org, and a tree that stops two
  levels down looks the same whether the dependencies end there or a proxy
  answered 404.
- One report per analysis therefore records what the graph cannot carry
  (`internal/trace`): the indexes the run knew about with where each was learned
  from and whether anything vouches for it; per ecosystem and level, how many
  packages were asked about, how many answered and what was added; and one entry
  per question, naming who answered it - a lock file, an index, a cached or
  already-given answer - or, where none did, why not.
- The reasons nothing answered are kept distinct, because on the map they are
  one and the same absence: no lock file covers it and the run is offline; its
  index is named only by the repository; it is private and its index is the
  public one; a proxy requires a version it was not given; the ecosystem has no
  index that can be queried; or a request was made and returned 404, 401 or
  worse. Where
  requests were made, each URL and the status it returned is kept, since a
  container image takes three round trips to answer and which one failed is the
  question.
- An ecosystem whose walk never started - its graph lives outside the repository
  and `--online` was not given - is recorded as such rather than contributing
  nothing silently. A standard library is not asked about at all.
- The report is rendered once, in Go, and read three ways: `--explain` writes a
  digest to the log (which is where the editor's output channel reads it),
  `GET /api/resolution` serves the whole of it as JSON, and the same endpoint
  renders it as a document (`?format=md`) or as that digest (`?format=text`).
  The editor's **Show the Resolution Report** opens the document, so nothing in
  TypeScript can drift from what the analysis actually did.
- Recording is bounded: the counts are always complete, the per-question detail
  stops at a fixed number of entries, and the written report cuts its long lists
  and its over-long cells short. `--watch` re-analyzes, and each re-analysis
  brings a report of its own rather than adding to yesterday's.

*Accepted when* a package declined for being private, one whose index only the
repository names, one whose index answered 404 and one that nothing was asked
about are four distinguishable entries with four different reasons; a run with
no `--resolve-depth` still reports which index every package resolves from; the
ecosystems that cannot be walked offline are named; and the document the editor
opens and the digest `--explain` writes describe the same analysis.

### M19 - Documentation as a dependency

- A document that links to a file depends on it, and nothing a package manager
  reads will say so. Every link a repository's Markdown carries to a file or a
  directory inside it becomes an edge from the document, drawn like an import;
  the headings become the file's symbols, so a document expands into its
  sections the way a source file expands into its functions. Inline links,
  reference definitions, autolinks and the `href` and `src` of raw HTML all
  count; a link inside fenced code or a code span does not, since it is printed
  rather than followed.
- There is no island for external hosts. A URL is not a dependency the map can
  characterize, and a legend of third-party domain names would add nothing.
- A link that leads nowhere is a finding on the line that carries it, with the
  column, so two breaks on one line are two findings rather than one. Four
  things can be wrong: the file is not there, the fragment names no heading of
  the file it points at, the reference was never defined, or a host says the
  page is gone.
- The first three need nothing but the repository and are exact, so they run on
  every analysis without being asked for. A target that exists but is not on the
  map - ignored, excluded, generated at build time - is not broken; the question
  is the filesystem's, not the graph's.
- The fourth requires a third-party server and occurs only under `--online`. Only
  404 and 410 count: a refusal, a rate limit, a timeout and a server error are
  what a checker is given by hosts that block robots, and reading them as rot
  would report links that work in a browser. What could not be checked is
  counted and said, not drawn, and it does not mark the set incomplete. Answers
  are kept for a day.
- Vendored documentation and fixtures under `testdata` are not checked. The
  first links to the parts of its own repository that vendoring does not copy;
  the second is wrong on purpose, since a link that leads nowhere is what a link
  check is tested against. A finding against either would be a defect that nobody
  is expected to correct.

*Accepted when* a README linking to a moved file reports it and one linking to a
generated file does not, a fragment naming a heading that exists is silent and
one naming a heading that does not is reported, a link inside a fenced block is
neither an edge nor a finding, two broken links on one line are two findings,
and a run over a repository that vendors its dependencies reports nothing from
their READMEs.

### M20 - What the walk costs, and what it is carried out with

- Roads take the city's own way up, always. The dependency roads are routed over
  a grid that carries the ramps and the bridges as the ground they are, and a
  rise steeper than a road can be drawn at is not priced against distance at
  all: the sweep settles everything reachable without climbing before it
  considers anything on the far side of a wall, so a road goes round to the ramp
  however far round it is. A budget would have been wrong at any size, since
  there is always a detour dear enough to buy a wall. Where no ramp connects two
  levels - the shore, which the city above it meets as a wall, and a terrace too
  small to fit one against - the rise is graded along the road until it lies at
  a ramp's gradient. A road never stands a quad on end at a curb.
- The walker has a condition. A fall of more than about a house costs health in
  proportion to the rest of the drop, and a bug within a stride bites no
  oftener than once a second for what its severity is worth, so several bites
  of anything are survivable and a critical one is worth three of a note. Every
  finding in the backpack raises the ceiling and mends by as much. At nothing
  the walk ends and the map returns with the backpack intact; walking in again
  begins at full health.
- Tools are primary or secondary. A primary tool tags a module or catches a
  bug; a secondary one does neither and carries the walker instead - the
  grapple gun's line, the jet backpack's flight, the water skimmers' hold on
  the surface. Which a tool is decides what the crosshair marks, what a shot
  does when it lands and how the slot is drawn, and there is one place that
  decides it.
- Flight is a thing carried rather than a mode. There is no flight key: the jet
  backpack is what flies, putting it away is how the walker comes down, and a
  click on it is a burst of thrust. The water skimmers make open water walkable
  for exactly as long as they are in hand.
- Three tools catch bugs by design - the butterfly net, the bubble wand and the
  fire extinguisher - and two more catch them incidentally, because a hook and
  a nail both take something off a wall. The tracking dart and the nail gun are
  told apart by their physics rather than by their models: the dart is lobbed,
  slow and steers towards the wall ahead of it; the nail is flat, fast and
  scatters.
- Severity is a shape as well as a color, because a color says nothing in a
  crowd or from behind. A critical finding walks as a caterpillar that never
  flies, the middle of the range as the beetle, and a note as a mite. The
  shapes are modelled in `tools/bug.py` and every one of them has a drawn
  stand-in, so a file built before a shape existed still puts it on the street.

*Accepted when* a road between two buildings on different terraces runs up a
ramp and nowhere stands on end, and still does when the ramp is at the far end
of the block and the detour is several times the direct route, a walker who
steps off a tower dies and is returned to the map with the backpack intact, a
secondary tool tags nothing it is pointed at, the jet backpack is the only way
to fly, a nail catches a bug, and a critical finding is visibly a different
animal from a note.

### Known limits

- Java imports name packages, not artifacts, so Maven dependencies are matched
  by heuristics; unmatched imports are shown as unresolved. For the same reason
  an index cannot be asked what a Maven package depends on: a POM is addressed by
  group and artifact, and the map has only the group.
- A GitHub Actions reference is `owner/repo@ref` whether it comes from github.com
  or from a GitHub Enterprise instance, so the two are one ecosystem on the map.
  `--private 'actions:internal-org/*'` is how an instance's own actions are kept
  off the public index and out of the vulnerability database.
- pip's keyring is not consulted, so a PyPI mirror whose credential is held only
  in a keyring answers 401 and is passed over in silence. A credential in the
  index URL or in netrc, which is how such a mirror is otherwise configured, is
  read.
- Servers that index slowly (rust-analyzer, jdtls) may answer before indexing
  finishes and return fewer references within the time budget.
