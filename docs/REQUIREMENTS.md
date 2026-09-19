# depphunter - Requirements

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

- Precise symbol-level (call-graph) references - deferred to an optional LSP
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
- Every color encodes exactly one thing, has a legend, and is never the only
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
  dashed centre line, wheel tracks and manholes; long streets get zebra
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
  frame, the walk-mode ground buffer is only recolored while walking, and darts
  in flight are dropped when a relayout invalidates their targets.
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

### Known limits

- Java imports name packages, not artifacts, so Maven dependencies are matched
  by heuristics; unmatched imports are shown as unresolved.
- Servers that index slowly (rust-analyzer, jdtls) may answer before indexing
  finishes and return fewer references within the time budget.
