# depphunter

[![CI](https://github.com/sarumaj/depphunter-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/sarumaj/depphunter-cli/actions/workflows/ci.yml)

Browse any code base as an interactive isometric archipelago in your browser.

- **Mainland** = your repository. Directories are terraces, files are buildings
  (height = lines of code, colour = language).
- **Islands** = external ecosystems (Go modules, the standard library, …) with
  one building per dependency.
- **Click** anything to see what it depends on and what uses it;
  **double-click** to expand or collapse directories and files (files expand
  into their symbols).
- **Walk** (`V`) the map in first person on a tiny planet: `WASD` to move, the
  mouse to look, `Space` to jump, `F` to fly. Throw newspapers (click or `Q`) at
  buildings to select them, `E` to expand what you aim at, and the mouse wheel to
  change the planet's curvature.

Everything runs locally: one binary, no network access, no Node.js.

## Install

Download an archive for your platform from the
[releases](https://github.com/sarumaj/depphunter-cli/releases) (checksums in
`checksums.txt`), or build it with Go 1.22 or newer:

| OS      | Architectures                         | Archive   |
|---------|---------------------------------------|-----------|
| Linux   | amd64, arm64, armv7, 386, riscv64     | `.tar.gz` |
| macOS   | amd64 (Intel), arm64 (Apple silicon)  | `.tar.gz` |
| Windows | amd64, arm64, 386                     | `.zip`    |
| FreeBSD | amd64, arm64                          | `.tar.gz` |

```sh
go install github.com/sarumaj/depphunter-cli/cmd/depphunter@latest
```

## Usage

```sh
depphunter            # analyse the current directory and open the browser
depphunter ~/src/app  # analyse another directory
depphunter --no-open --addr 127.0.0.1:8080
depphunter --watch    # keep the map in sync while you edit
depphunter --export dot -o deps.dot   # write the graph and exit
depphunter --export html -o map.html  # a self-contained map to share
```

| Flag              | Default                   |                                                         |
|-------------------|---------------------------|---------------------------------------------------------|
| `--addr`          | `127.0.0.1:0`             | listen address; port 0 picks a free port                |
| `--no-open`       |                           | print the URL instead of opening the browser            |
| `--exclude`       |                           | glob of paths to skip (repeatable)                      |
| `--max-file-size` | `2097152`                 | larger files are listed but not read                    |
| `--config`        | `<path>/.depphunter.yaml` | config file to use                                      |
| `--theme`         | `auto`                    | `auto`, `light`, `dark`                                 |
| `--color-by`      | `language`                | `language`, `size`, `commits`, `churn`, `age`, `authors` |
| `--height-scale`  | `sqrt`                    | `linear`, `sqrt`, `log`                                 |
| `--show-std`      | `false`                   | show standard-library islands                           |
| `--expand-depth`  | `0`                       | initially expanded depth; `0` = auto, `-1` = all        |
| `--watch`         | `false`                   | re-analyse on file changes, update the browser live     |
| `--no-cache`      |                           | neither read nor write the analysis cache               |
| `--no-history` | | do not read git history |
| `--history-commits` | `10000` | read at most this many commits |
| `--lsp` | | find symbol references with installed language servers |
| `--lsp-timeout` | `5m` | time budget for language servers |
| `--version` | | print the version and exit |
| `--editor`        | auto-detected             | editor command template, e.g. `"code -g {file}:{line}"` |
| `--export`        |                           | write `json`, `graphml`, `dot` or `html` and exit       |
| `-o`              | stdout                    | output file for `--export`                              |

Settings are resolved from, in increasing precedence: built-in defaults, the
user config (`$XDG_CONFIG_HOME/depphunter/config.yaml`, or the OS equivalent),
the project config `.depphunter.yaml`, `DEPPHUNTER_*` environment variables
(`ADDR`, `OPEN`, `EXCLUDE`, `THEME`, `COLOR_BY`, `HEIGHT_SCALE`, `SHOW_STD`,
`WATCH`, `CACHE`, `EDITOR`, `HISTORY`, `LSP`), and flags. The project config
cannot set `editor`: it arrives with the repository, and the editor is a command
depphunter runs.

```yaml
# .depphunter.yaml
exclude: [testdata, "*.pb.go"]
history_commits: 5000
ui:
  color_by: language
  height_scale: sqrt
  show_std: false
  expand_depth: 0
  hide_languages: [Markdown]      # filters, as the Filters panel sets them
  hide_islands: [npm]
  path_filter: "!**/testdata/**"
```

The browser's **Save settings** button writes the current colour, height, theme,
depth and filters into the `ui:` section of `.depphunter.yaml` (or the
`--config` file), keeping the file's other keys and comments.

## Watch mode and cache

Parsing results are cached per file content under the user cache directory
(`~/.cache/depphunter` on Linux), so a second run only parses files that changed
— the CPython standard library goes from 1.2 s to 20 ms. With `--watch`,
depphunter watches the directories it analysed, re-analyses after changes settle
(300 ms), and pushes the new map to the browser, which keeps your expansion,
selection and filters and briefly highlights the files that changed.

## Exports

`--export` (or the **Export** menu in the browser) writes:

| Format    | Contents                                                                                                                                                                       |
|-----------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `json`    | the full graph document the UI uses (nodes, symbols, edges)                                                                                                                    |
| `graphml` | the full graph with all attributes, for Gephi, yEd or NetworkX                                                                                                                 |
| `dot`     | the dependency graph for Graphviz: files, package directories and external packages, clustered per directory; standard-library packages and files without imports are left out |
| `html`    | the interactive map as one file that opens without depphunter or a network: UI, graph, current view settings and source text (files up to 256 KB, 24 MB in total)              |

**Export → PNG image** (or `P`) saves the map as shown, labels included, at your
screen's resolution; it also works in an exported HTML page.

The HTML export contains your source code and, when the git history was read,
commit authors' names; share it like you would share the repository.

Dependency graphs are shallow, so Graphviz draws them long and thin; for big
ones, `unflatten -l 3 -c 5 deps.dot | dot -Tsvg -o deps.svg` spreads them out.

## Git history

In a git work tree, depphunter reads the history of the analysed files (the
newest 10,000 non-merge commits by default; `--history-commits` changes the
limit, `--no-history` turns it off) in the background once the map is shown, and
caches it per commit. The **Colour** menu then offers:

| Mode | Colour shows |
|---|---|
| Commits | commits per file (per-file mean for collapsed directories) |
| Lines changed | lines added plus deleted |
| Last change | how recently a file changed, recent is strong |
| Authors | distinct authors |

Files without commits in range get a separate neutral colour. The **Since**
slider in the legend limits commits, lines changed and authors to a time range;
tooltips and the side panel show the same figures, the panel also the top
authors. Renamed files keep the history of their old names. With `--watch`, a
new commit updates the overlay.

## Symbol references

Imports show which files depend on which; with `--lsp`, depphunter also asks
language servers which symbols use which. It uses the servers it finds on `PATH`
(and `go install` locations for gopls):

| Language | Server |
|---|---|
| Go | `gopls` |
| JavaScript / TypeScript | `typescript-language-server` |
| Python | `pyright-langserver`, `basedpyright-langserver` or `pylsp` |
| Rust | `rust-analyzer` |
| Java | `jdtls` |
| C# | `csharp-ls` |

The servers run in the background after the map is shown (gopls needs about 7 s
for this repository), within `--lsp-timeout`; results are cached until the code
changes. The legend's **Imports / References** switch then changes what
selection arcs and the side panel show: for a function, what it uses and what
uses it. JSON and GraphML exports include the reference edges.

## Opening files in your editor

The side panel's **Open in editor** button (or `O`) opens the selected file at
the selected symbol's line. depphunter uses `--editor` / `DEPPHUNTER_EDITOR` /
the user config, or detects a GUI editor from `$VISUAL`, `$EDITOR` or `PATH` (VS
Code, Cursor, Zed, Sublime Text, JetBrains IDEs, …). Without one, the button
hands the file to VS Code's `vscode://` URL handler.

## Keyboard & mouse

|                           |                                          |
|---------------------------|------------------------------------------|
| Drag / right-drag / wheel | pan / orbit / zoom                       |
| Click / double-click      | select / expand–collapse                 |
| `Enter`, `Backspace`      | expand–collapse selection, select parent |
| `Q` `E`                   | rotate 90°                               |
| `F`                       | fit to screen                            |
| `+` `−`                   | expand / collapse one level everywhere   |
| `/`                       | search files, symbols and packages       |
| `O`                       | open the selected file in your editor    |
| `P`                       | save the map as a PNG image              |
| Legend click              | hide / show a language                   |
| `Esc`                     | clear selection                          |
| `V`                       | walk mode                                |

In walk mode:

|                        |                                                    |
|------------------------|----------------------------------------------------|
| Click the map          | look with the mouse (`Esc` frees it), or drag      |
| `W` `A` `S` `D`/arrows | move / turn; `Shift` runs                          |
| `Space`                | jump (flying: rise)                                |
| `F`                    | fly on / off (`C` sinks)                           |
| Click / `Q`            | throw a newspaper: the building it hits is selected |
| `Enter`                | select what the crosshair is on                    |
| `E` / right click      | expand / collapse what the crosshair is on         |
| Wheel / `[` `]`        | planet size (curvature)                            |
| `V` / `Esc`            | back to the map                                    |

## Security

The server binds to loopback by default and prints a URL containing a random
token, which the browser exchanges for a cookie. Requests without it, requests
with a foreign `Host` header (DNS rebinding), and requests for files that are
not part of the analysed project are rejected.

## Languages

| Ecosystem               | Imports resolved through                                                                                                                                                          | Islands                              |
|-------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------|
| Go                      | every `go.mod` (multi-module, local `replace`)                                                                                                                                    | Go modules, Go standard library      |
| JavaScript / TypeScript | relative paths, `tsconfig`/`jsconfig` `paths`, workspaces, `package.json` + `package-lock.json` / `yarn.lock` / `pnpm-lock.yaml`                                                                                   | npm, Node.js built-ins               |
| Python                  | relative imports, `src/` layouts, requirements files, `setup.cfg`, literal `setup.py` lists, `pyproject.toml`, `Pipfile`, `poetry.lock`/`uv.lock`/`pdm.lock`/`Pipfile.lock`                                              | PyPI, Python standard library        |
| Rust                    | the module tree (`crate::`, `self::`, `super::`, `mod x;`), workspace and path crates, `Cargo.toml` (renamed and workspace dependencies) + `Cargo.lock`                           | crates.io, Rust standard library     |
| Java                    | source files by package path (any source root), `pom.xml` (properties, dependency management), Gradle scripts and version catalogs                                                | Maven, Java standard library         |
| C#                      | namespaces to project folders (`RootNamespace` + folder), `PackageReference`, `Directory.Packages.props`                                                                          | NuGet, .NET base library             |
| PowerShell              | `using module`, `Import-Module`, dot-sourced and `&`-invoked scripts (`$PSScriptRoot`), `#Requires -Modules`, module manifests (`RequiredModules`, `RootModule`, `NestedModules`) | PowerShell Gallery, built-in modules |

Java imports name packages, not artifacts, so they are matched to Maven groupIds
by prefix, shared leading segments, artifact names and a short table of
well-known mismatches (Guava, JUnit 4, Lombok, …); what cannot be matched is
shown as unresolved.

Files in other languages appear on the map without dependency edges. Parsing
uses a pure-Go tree-sitter runtime (C# and PowerShell use small built-in
scanners instead), so the binary still cross-compiles without a C toolchain.

## How it works

```text
 scan ──► extract (per file, parallel, cached) ──► resolve ──► graph ──► server ──► browser
  │         tree-sitter / go/parser / scanners      manifests,    JSON     HTTP+SSE    three.js
  └ git ls-files or built-in ignores                lockfiles                        map / walk
        background, after the map is shown:  git history · LSP references ──► SSE ──► overlay
```

1. **Scan** (`internal/scan`) lists the project's files through `git ls-files`
   (so `.gitignore` applies) or a built-in ignore list, applies `exclude`
   globs, and measures language (by extension) and lines of code.
2. **Extract** (`internal/lang/*`): every language plugin claims files and
   extracts imports and top-level symbols from the content alone. Files are
   parsed in parallel; results are cached by the SHA-256 of the content, the
   plugin version and the extension (`internal/cache`), so a second run only
   parses what changed.
3. **Resolve**: a per-run resolver of each plugin reads the manifests and
   lockfiles (`go.mod`, `package.json`, `pyproject.toml`, `Cargo.toml`,
   `pom.xml`, `*.csproj`, …) and turns every import into a local file or
   directory, a standard-library package or an external package with version.
4. **Graph** (`internal/analyze`, `internal/graph`): directories, files,
   symbols, ecosystems and packages become nodes, imports become edges; the
   document is what the UI, the exports and the cache exchange.
5. **Serve** (`internal/server`): a loopback HTTP server with a per-run token
   serves the embedded UI (`web/static`) and the graph (`/api/graph`), file
   source (`/api/file`), exports, "open in editor" and "save settings".
   Server-Sent Events (`/api/events`) push new graphs in `--watch` mode
   (`internal/watch`) and announce the git history (`internal/history`) and
   LSP references (`internal/lsp`), which are read in the background once the
   map is up.
6. **Render** (browser, plain ES modules, no build step): `model.js` builds a
   navigable tree with aggregates, `layout.js` computes the archipelago from
   the hierarchy and expansion state (never a force simulation, so the same
   repository always gives the same map), `scene.js` draws every box in one
   instanced three.js mesh and edges as arcs, `labels.js` places labels,
   `filter.js` and `history.js` compute filters, search and history colours
   locally, and `walk.js`/`city.js` add the first-person view.

The HTML export (`--export html`) inlines the same modules as `data:` URLs with
the graph, settings, history and source text, so the page needs neither
depphunter nor a network.

### Built with

Go libraries (all pure Go, so every target cross-compiles with
`CGO_ENABLED=0`):

| Library | Used for |
|---|---|
| [odvcencio/gotreesitter](https://github.com/odvcencio/gotreesitter) | tree-sitter runtime and grammars for JS/TS, Python, Rust and Java |
| [golang.org/x/mod](https://pkg.go.dev/golang.org/x/mod) | parsing `go.mod` |
| [BurntSushi/toml](https://github.com/BurntSushi/toml) | `pyproject.toml`, `Cargo.toml`, Gradle version catalogs, TOML lockfiles |
| [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) | config files (comment-preserving save), `pnpm-lock.yaml` |
| [tidwall/jsonc](https://github.com/tidwall/jsonc) | `tsconfig.json` / `jsconfig.json` with comments |
| [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) | `--watch` |
| [sourcegraph/jsonrpc2](https://github.com/sourcegraph/jsonrpc2) | talking to language servers (`--lsp`) |
| [golang.org/x/sync](https://pkg.go.dev/golang.org/x/sync) | bounded parallel scanning, parsing and LSP requests |
| [emicklei/dot](https://github.com/emicklei/dot) | DOT export |
| [kballard/go-shellquote](https://github.com/kballard/go-shellquote) | splitting editor command templates without a shell |
| [cli/browser](https://github.com/cli/browser) | opening the default browser |

Go itself provides `go/parser` for Go sources, `net/http` for the server and
`embed` for the UI. The browser UI vendors, in
[web/static/vendor](web/static/vendor/README.md):
[three.js](https://threejs.org) (WebGL rendering, orbit controls),
[highlight.js](https://highlightjs.org) (source highlighting),
[potpack](https://github.com/mapbox/potpack) (packing terraces) and
[fzf-for-js](https://github.com/ajitid/fzf-for-js) (fuzzy search). External
tools are optional: `git` for file listing and history, language servers for
`--lsp`.

## Development

```sh
go test -race ./...
go run honnef.co/go/tools/cmd/staticcheck@2024.1.1 ./...
```

CI (`.github/workflows/ci.yml`) builds and tests on Linux, macOS and Windows, on
the current Go and on Go 1.22 with `GOTOOLCHAIN=local`, so a `go.mod` that
starts to need a newer Go fails the build. It also checks formatting, `go mod
tidy`, vet, staticcheck, govulncheck, JavaScript syntax and Markdown. Pushing a
`v*` tag runs `.github/workflows/release.yml`, which tests and then publishes
stripped binaries for every platform in the [Install](#install) table.

Both workflows build the archives with `scripts/dist.sh`, which also works
locally (it needs `zip`):

```sh
scripts/dist.sh v1.2.3                     # every target into dist/
TARGETS="linux/amd64 darwin/arm64" scripts/dist.sh
```

CI builds all targets on every push, keeps the archives for 14 days as workflow
artifacts, and runs the tests as 32-bit (`GOARCH=386`).

The milestones and design decisions are in
[docs/REQUIREMENTS.md](docs/REQUIREMENTS.md).

## License

[BSD 3-Clause](LICENSE) © 2026 Dawid Ciepiela. The embedded three.js (MIT),
highlight.js (BSD 3-Clause), potpack (ISC) and fzf-for-js (BSD 3-Clause) keep
their own licenses; see
[web/static/vendor](web/static/vendor/README.md).
