# depphunter

[![CI](https://github.com/sarumaj/depphunter-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/sarumaj/depphunter-cli/actions/workflows/ci.yml)
[![Open VSX Release Date](https://img.shields.io/open-vsx/release-date/sarumaj/depphunter)](https://open-vsx.org/extension/sarumaj/depphunter)
[![Open VSX Downloads](https://img.shields.io/open-vsx/dt/sarumaj/depphunter)](https://open-vsx.org/extension/sarumaj/depphunter)

> **Note:** This codebase was developed with the assistance of AI tools
> (Claude, by Anthropic). All code is reviewed and tested before being merged.

|                   Initial view                    |                      Dependency trace                      |
|:-------------------------------------------------:|:----------------------------------------------------------:|
| ![Initial view](docs/screenshots/screenshot1.png) |   ![Dependency trace](docs/screenshots/screenshot2.png)    |
|                   **Walk mode**                   |                       **Night mode**                       |
|  ![Walk mode](docs/screenshots/screenshot3.png)   |      ![Night mode](docs/screenshots/screenshot4.png)       |
|                  **Flight mode**                  |                 **Vulnerability hunting**                  |
| ![Flight mode](docs/screenshots/screenshot5.png)  | ![Vulnerability hunting](docs/screenshots/screenshot6.png) |

depphunter renders a code base as an interactive isometric archipelago in a
browser.

- **The mainland** is the repository. Directories are terraces and files are
  buildings, whose height is the file's line count and whose color is its
  language. The map is drawn as a city: facades with windows, streets with
  sidewalks and crossings, ramps between levels, parks, and a surrounding sea.
  Selecting a node dims the rest, and dimmed buildings lose their detail so that
  the selection remains legible.
- **The islands** are external ecosystems - Go modules, a standard library, npm
  - with one building per dependency.
- **Selecting** a node shows what it depends on and what depends on it, as arcs
  over the map with an arrow at the far end of each. Double-clicking expands or
  collapses a directory or a file; a file expands into its symbols.
- **Walk mode** (`V`) presents the same map in first person on a small planet.
  `WASD` moves, the mouse looks and `Space` jumps. The walker holds a tool,
  drawn in the hands at the end of an arm; using it on a building selects the
  module, draws its dependency trails and marks it with a beacon for the rest
  of the session.

  Ten tools occupy slots `1` to `9` and `0`; `E` cycles through the primary
  tools, `Q` through the secondary ones, and holding `R` opens a wheel of all of
  them. The seven **primary** tools are what the hunt is done with: a fishing
  rod (the default), a butterfly net, a camera, a bubble wand, a fire
  extinguisher, a tracking dart and a nail gun. Each has its own animation, its
  own aim helper and its own valid targets - the dart acts on buildings, the
  net, the bubbles and the extinguisher on bugs (the extinguisher also puts out
  fires), the rod, the nail gun and the camera on either. What a tool throws
  travels according to its own flight model, and the two launchers are opposites
  rather than variants: the tracking dart is the longest and most deliberate
  shot, lobbed high and steering in the air towards the wall ahead of it, one
  shot per click through the scope; the nail gun reaches only across a street,
  fires flat and fast for as long as the button is held down, and scatters. The
  fire extinguisher is held down in the same way. A bubble decelerates and
  rises; foam spreads and drops. The net throws nothing and must be brought
  within reach; the camera shows its lens view live on its back, and every use
  of it keeps the frame, except a use on a module that has already been tagged,
  with no bug in front of it, which reads that module the way every other tool
  does there. A photograph is of the city and of nothing else: neither the
  camera nor the hand holding it is in it — what the walker holds is drawn over
  the world in a pass of its own, and that pass is left out — and neither is
  anything the interface has put on the map, so nothing is lit by a selection,
  nothing is dimmed by one and no dependency arcs cross the rooftops. A
  screenshot (`P`) is the screen rather than what the camera was pointed at, so
  it keeps all of them.

  What it keeps goes into the **photographs** (`G`), a contact sheet of the
  session's pictures captioned with whatever was in the frame. Each can be
  saved to a PNG file, let go of, or — from the street — put back up on the
  camera: the camera comes out if it is not already in hand and is brought all
  the way up to the walker's face, square on, until the picture on its back
  screen — which stands in for the live view while it is there — covers nearly
  the whole of it. After a few seconds it goes back down and gives the hand
  back to whatever was in it. A click puts it away sooner. The pictures are a session
  and nothing more, so anything worth keeping is saved to a file.

  The three **secondary** tools touch nothing on the map and carry the walker
  instead. The grapple gun, fired with `F` or `C`, hooks the building the walker
  is looking at and draws them up the facade and onto the roof, from where a
  shot over the edge is the way down. Its claw closes on a parapet, not on a
  flat wall: a hook that strikes more than two storeys below a roof's edge
  glances off, tumbles down and is reeled back in, and pulls nobody anywhere;
  the jet backpack flies; the water skimmers make the bay walkable, passing
  under the bridges rather than over them and stepping back up onto a shore that
  stands half a unit above the water. The latter two run on a tank, which
  empties only while the tool is doing its work and fills again whenever it is
  not, so neither is a way of getting everywhere; a gauge beside the health bar
  says what is left, running dry in the air is a fall, and a tool that has run
  out stays stopped until its tank has filled back to a quarter, when it works
  again by itself. One tool of each
  kind is carried at a time, one to a hand — the primary in the right, the
  secondary in the left — so the map can be flown over and its bugs netted
  without putting either down. A click uses the right hand, and `F`, `C` or the
  middle button the left. The HUD lays the slots out the way the walker is: what
  the left hand carries on the left and what the right hand hunts with on the
  right, each group behind a small hand of its own, because a row read at a
  glance in the middle of something else should not have to be parsed. The rod
  climbs too, from the hunting hand, which leaves the other free for the jet or
  the skimmers: a cast that comes down on a roof winds the walker up onto it,
  more slowly than the grapple and on a shorter line. A fish hook is not made
  for brick, though, so a cast at a wall, however high, always skips off the way
  a glancing grapple does. The right button holds
  the scope, and the mouse wheel zooms the view.

  Running and jumping are paid for out of a second gauge, the walker's **wind**.
  A sprint drains it in a few seconds and each jump takes a little more; it
  fills again while walking or standing, and a walker who has run it out has to
  get a quarter of it back before they can run or jump again, so the way across
  a district is a series of dashes rather than one long one. Flying costs nothing:
  that is the jet's tank, not the walker's chest.

  The walker has a **health bar**. A fall is what it would be in life, measured
  against the walker, who is half a unit tall: nothing up to about three metres,
  which a jump off a terrace wall stays under, then a share of the bar that grows
  with the height, and the end of the walk from about seventeen metres — five or
  six floors — however full the backpack. Being reeled down a line counts as
  falling. A bug's bite costs more the worse the finding is, and deep water with
  nothing to float on takes all of it in a couple of seconds, the walker going
  under as it does: the view sinks and bobs, the water closes over it from the
  bottom of the screen with bubbles rising through it, and all of it drains away
  again if a shore is reached in time — so stowing the
  skimmers out over the bay is the end of that walk, and so is walking into it
  without them unless a shore is reached first. The bay is not a wall around the
  map: it is ground half a unit below the shore, so it can be stepped down into
  the way a curb can, waded about in, and — because the shore is further up than
  a step — climbed back out of, whether by something in it or something on a
  pair of floats. A bridge deck is where that stops. Walking off an edge into a
  drop is not a thing anybody means to do, so the railings have to be gone over
  rather than through: off a deck, or off anything else standing well above the
  surface, the water has to be jumped into. Every bug
  in the backpack raises the bar and mends by as much. At nothing the screen goes
  red, walk mode ends and the map returns — nothing caught is lost — and walking
  in again starts at full health. Away from bites and fire, the bar slowly
  fills again by itself.

  Each finding a scanner reported is represented by one bug, up to 140 of the
  most severe, except a vulnerability proven reachable, which burns as a fire
  instead (see [Findings](#findings)). A bug is shaped by its severity as well
  as colored by it: a critical finding is a caterpillar that crawls and never
  flies, a high or medium one a beetle, and a low or informational one a mite.
  Bugs are placed at several heights on a building's facade and around its roof
  as well as in the streets, each oriented to the surface it holds on to; some
  hold on to nothing and instead fly a circuit around the building, rising,
  falling and banking at the corners. Catching one opens what was reported about
  it. A module the walker has tagged carries a beacon in the color of the worst
  finding in it, and its ring on the tracker matches, so the hunt's own trophies
  say which of them were worth having. A tracker in the corner of the screen
  sweeps the surrounding map and tightens as the walker approaches a bug, so
  that the last part of the approach can be made on the sweep rather than by
  guesswork.

  A first visit is given a short **introduction** explaining what the shapes
  stand for, that the map can be walked into, and what the bugs are, and a first
  walk is given one of its own covering what moves, what is in each hand, what
  using it does, the bugs, and fire and staying alive; the help (`?`) documents
  every key and every tool, and can show either introduction again.

  Terraces are laid out as city blocks: the space between buildings forms a
  connected street network with sidewalks, lane markings and crossings; ramps
  and stairs connect levels; unoccupied lots become parks; and a bridge crosses
  the water to every island.

The tool runs entirely locally. It is a single binary, requires no Node.js, and
makes no network request unless `--online` is given (see
[Package indexes](#package-indexes)).

## Two ways to run it

depphunter is a command-line tool that serves the map over HTTP to a browser.
The [VS Code extension](#vs-code-extension) is a wrapper around that same tool:
it starts a server for the open folder and displays the map in a tab beside the
code. The page served is identical in both cases.

|                  | [Command-line tool](#command-line-tool)             | [VS Code extension](#vs-code-extension)                   |
|------------------|-----------------------------------------------------|-----------------------------------------------------------|
| Installation     | a release archive, or `go install`                  | the Marketplace or Open VSX (the binary is bundled)       |
| Invocation       | `depphunter [path]`                                 | `depphunter: Open the Map`, or a folder's context menu    |
| The map opens in | the default browser                                 | a tab of the editor's own, beside the code                |
| Configuration    | flags, `DEPPHUNTER_*` variables, `.depphunter.yaml` | `depphunter.*` settings, and the same `.depphunter.yaml`  |
| Additionally     | exports to JSON, GraphML, DOT and HTML              | maintains one server per folder for the editor's lifetime |

## Contents

- [Command-line tool](#command-line-tool): [Install](#install) ·
  [Usage](#usage) · [Configuration](#configuration) ·
  [Watch mode and cache](#watch-mode-and-cache) · [Exports](#exports) ·
  [HTTP API](#http-api) ·
  [Opening files in an editor](#opening-files-in-an-editor)
- [VS Code extension](#vs-code-extension):
  [Install](#install-the-extension) · [Use](#use) ·
  [The panel beside the code](#the-panel-beside-the-code) ·
  [Settings](#settings) · [Remote workspaces](#remote-workspaces) ·
  [Why the map is in a tab of its own](#why-the-map-is-in-a-tab-of-its-own) ·
  [How the framing works](#how-the-framing-works)
- [The map](#the-map): [Keyboard & mouse](#keyboard--mouse) ·
  [Styles](#styles) · [Findings](#findings) · [Git history](#git-history) ·
  [Versions and pinning](#versions-and-pinning) ·
  [Dependencies of dependencies](#dependencies-of-dependencies) ·
  [Package indexes](#package-indexes) ·
  [Private dependencies](#private-and-internal-dependencies) ·
  [The resolution report](#the-resolution-report) ·
  [CI pipelines](#ci-pipelines) ·
  [Infrastructure as code](#infrastructure-as-code) ·
  [Nix](#nix) · [Gleam](#gleam) · [Elm](#elm) ·
  [Interface definitions](#interface-definitions) ·
  [Shell scripts](#shell-scripts) ·
  [Documentation](#documentation) ·
  [Symbol references](#symbol-references) · [Languages](#languages)
- [Security](#security) · [Contributing](#contributing) · [License](#license)

## Command-line tool

### Install

Download an archive for the target platform from the
[releases](https://github.com/sarumaj/depphunter-cli/releases) (checksums in
`checksums.txt`), or build it with Go 1.27.1 or newer:

| OS      | Architectures                        | Archive   |
|---------|--------------------------------------|-----------|
| Linux   | amd64, arm64, armv7, 386, riscv64    | `.tar.gz` |
| macOS   | amd64 (Intel), arm64 (Apple silicon) | `.tar.gz` |
| Windows | amd64, arm64, 386                    | `.zip`    |
| FreeBSD | amd64, arm64                         | `.tar.gz` |

```sh
go install github.com/sarumaj/depphunter-cli/cmd/depphunter@latest
```

### Usage

```sh
depphunter            # analyze the current directory and open a browser
depphunter ~/src/app  # analyze another directory
depphunter --no-open --addr 127.0.0.1:8080
depphunter --watch    # re-analyze on change and update the open map
depphunter --findings trivy.json      # place a scanner's report on the map
depphunter --export dot -o deps.dot   # write the graph and exit
depphunter --export html -o map.html  # write a self-contained map
```

| Flag                | Default                   |                                                                                               |
|---------------------|---------------------------|-----------------------------------------------------------------------------------------------|
| `--addr`            | `127.0.0.1:0`             | listen address; port `0` selects a free port                                                  |
| `--no-open`         |                           | print the URL rather than opening a browser                                                   |
| `--exclude`         |                           | glob of paths to omit; repeatable                                                             |
| `--max-file-size`   | `2097152`                 | files above this size are listed but not read                                                 |
| `--config`          | `<path>/.depphunter.yaml` | configuration file to read                                                                    |
| `--theme`           | `auto`                    | `auto`, `light` or `dark`                                                                     |
| `--color-by`        | `language`                | `language`, `size`, `commits`, `churn`, `age` or `authors`                                    |
| `--height-scale`    | `sqrt`                    | `linear`, `sqrt` or `log`                                                                     |
| `--style`           | `city`                    | presentation of the map: `city`, `circuit` or `galaxy`                                        |
| `--show-std`        | `false`                   | include standard-library islands                                                              |
| `--expand-depth`    | `0`                       | directory depth expanded initially; `0` picks one, `-1` expands all                           |
| `--ui-default`      |                           | seed a view setting for a repository that has saved none: `key=value`, repeatable (see below) |
| `--watch`           | `false`                   | re-analyze on file change and update the open map                                             |
| `--no-cache`        |                           | neither read nor write the analysis cache                                                     |
| `--no-history`      |                           | do not read git history                                                                       |
| `--history-commits` | `10000`                   | read at most this many commits                                                                |
| `--resolve-depth`   | `0`                       | levels of transitive dependencies to resolve from lock files; `-1` for all                    |
| `--private`         | *(GOPRIVATE etc.)*        | glob naming packages the organization owns; never sent to a public index or to OSV            |
| `--trust-index`     | *(none)*                  | index URL to treat as configured on this machine, so a repository naming it is unmarked       |
| `--python`          | *(VIRTUAL_ENV, `.venv`)*  | Python interpreter whose installed packages resolve imports no package index has              |
| `--online`          | `false`                   | query package indexes for what the project's own files do not record                          |
| `--explain`         | `false`                   | write the resolution report once the analysis is complete                                     |
| `--lsp`             |                           | resolve symbol references using the installed language servers                                |
| `--lsp-timeout`     | `5m`                      | time budget for the language servers                                                          |
| `--findings`        |                           | scanner report to place on the map; repeatable, globs permitted                               |
| `--no-vulns`        |                           | place no scanner reports and do not query the OSV database                                    |
| `--no-links`        |                           | do not follow the links in the repository's Markdown                                          |
| `-v`, `--version`   |                           | print the version and exit                                                                    |
| `-h`, `--help`      |                           | list the flags with their defaults                                                            |
| `--editor`          | auto-detected             | editor command template, e.g. `"code -g {file}:{line}"`                                       |
| `--embed`           |                           | origin permitted to frame the map, e.g. `vscode-webview:`; repeatable                         |
| `--export`          |                           | write `json`, `graphml`, `dot` or `html` and exit                                             |
| `-o`, `--output`    | stdout                    | output file for `--export`                                                                    |

Long flags take two hyphens (`--addr`, not `-addr`), and a flag's value may be
separated from it by either a space or `=`.

Diagnostic output - what was analyzed, the address being served, and the
[resolution report](#the-resolution-report) - is written to **stdout**, so that
it can be piped or redirected like any other output. The sole exception is
`--export` without `-o`, where stdout carries the exported document; the log is
then written to stderr instead, so that a redirected export contains nothing but
the export. Errors are written to stderr in all cases.

### Configuration

Settings are resolved in the following order of increasing precedence: built-in
defaults; the user configuration
(`$XDG_CONFIG_HOME/depphunter/config.yaml`, or the platform equivalent); the
project configuration `.depphunter.yaml`; the `DEPPHUNTER_*` environment
variables (`ADDR`, `OPEN`, `EXCLUDE`, `MAX_FILE_SIZE`, `THEME`, `COLOR_BY`,
`HEIGHT_SCALE`, `STYLE`, `SHOW_STD`, `EXPAND_DEPTH`, `TOOL`, `WATCH`, `CACHE`,
`EDITOR`, `PYTHON`, `HISTORY`, `HISTORY_COMMITS`, `RESOLVE_DEPTH`, `ONLINE`,
`EXPLAIN`, `VULNS`, `LINKS`, `LSP`, `LSP_TIMEOUT`, and the comma-separated
`FINDINGS`, `PRIVATE` and `TRUST_INDEXES`); and the command-line flags. For
`exclude`, `findings`, `private` and `trust_indexes`, the lists from every
source — the user file, the project file, the environment and the flags — are
combined rather than replacing one another. `--private` also takes in the Go toolchain's
`GOPRIVATE`, `GONOPROXY`, `GONOSUMDB` and `GONOSUMCHECK` patterns.

The project configuration arrives with the repository, so it may not set
`editor` (a command depphunter executes), `online`, `python` or
`trust_indexes`, and its `findings` paths must stay inside the repository. A
file passed with `--config` is trusted like the user configuration, and must
exist.

`--ui-default` accepts the keys `theme`, `color_by`, `height_scale`, `style`,
`show_std`, `expand_depth`, `tool` and `path_filter`.

```yaml
# .depphunter.yaml
exclude: [testdata, "*.pb.go"]
history_commits: 5000
ui:
  color_by: language
  height_scale: sqrt
  show_std: false
  expand_depth: 0
  tool: rod                       # walk mode: rod, net, camera, bubbles,
                                  # extinguisher, dart, nailer, grapple,
                                  # jetpack, skimmers
  hide_languages: [Markdown]      # filters, as the Filters panel sets them
  hide_islands: [npm]
  path_filter: "!**/testdata/**"
```

The browser's **Save settings** button writes the current color, height, style,
theme, depth, walk-mode tool, standard-library islands and filters into the
`ui:` section of `.depphunter.yaml` (or the `--config` file), keeping the file's
other keys and comments.

That section is where a repository's view lives, and it is what the map reads on
the way in. `--theme` and the rest override it, as a flag does; `--ui-default`
sits underneath it instead, replacing only what depphunter would otherwise have
started at. So `--ui-default theme=dark` decides how a repository that has never
been saved opens, and stops deciding the moment somebody presses Save. That is
what the editor extension sends its appearance settings as, so the two never
disagree about which one won.

### Watch mode and cache

Parse results are cached by file content under the user cache directory
(`~/.cache/depphunter` on Linux), so that a subsequent run parses only the files
that have changed; for the CPython standard library this reduces analysis from
1.2 s to 20 ms. Under `--watch`, depphunter monitors the directories it
analyzed, re-analyzes once changes have settled for 300 ms, and pushes the
resulting map to the browser, which preserves the current expansion, selection
and filters and briefly highlights the files that changed.

### Exports

`--export` (or the **Export** menu in the browser) writes:

| Format    | Contents                                                                                                                                                                                                                                                                   |
|-----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `json`    | the complete graph document used by the interface: nodes, symbols and edges                                                                                                                                                                                                |
| `graphml` | the complete graph with all attributes, for Gephi, yEd or NetworkX                                                                                                                                                                                                         |
| `dot`     | the dependency graph for Graphviz: files, package directories and external packages, clustered by directory. Standard-library packages and files without imports are omitted                                                                                               |
| `html`    | the interactive map as a single file, requiring neither depphunter nor a network: the interface, the graph, the source text (files up to 256 KB, 24 MB in total) and the current view — colors, height, theme, depth and filters. `--export html` uses the configured view |

**Export → PNG image**, or `P`, writes the map as currently displayed, including
labels, at the screen's resolution. It is available in an exported HTML page as
well.

The HTML export contains the repository's source code and, where git history was
read, the names of commit authors. It should be distributed under the same
conditions as the repository itself.

Dependency graphs are shallow, which leads Graphviz to lay them out long and
narrow. For large graphs, `unflatten -l 3 -c 5 deps.dot | dot -Tsvg -o deps.svg`
produces a more even result.

### HTTP API

The server that hosts the map exposes a small HTTP API, which is also what the
VS Code extension's side panel reads. Every request requires the session token,
supplied in the cookie that the initial address is exchanged for or, when the
server runs with `--embed`, in an `X-Depphunter-Token` header (or a `?token=`
parameter, which the event stream uses). Every request that modifies state additionally
requires the header `X-Depphunter-Request: 1`, which a cross-site page cannot
set.

| Endpoint                                               | What it is                                                                                                                   |
|--------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------|
| `GET /api/graph`                                       | the graph document: nodes, symbols and edges                                                                                 |
| `GET /api/config`                                      | the view the map opens with, and the server's capabilities                                                                   |
| `GET /api/file?path=`                                  | the source of a file present on the map, and of no other file; `415` for a binary file, naming its type; `&as=raw` its bytes |
| `GET /api/history`, `/api/references`, `/api/findings` | computed in the background: `202` while in progress, `204` if there is no result                                             |
| `GET /api/export?format=&ui=`                          | `json`, `graphml`, `dot` or `html`; `ui` is the view (JSON) an `html` export opens with                                      |
| `GET /api/resolution?format=`                          | how the dependencies were resolved: `json` (default), `md` or `text`                                                         |
| `GET /api/events`                                      | Server-Sent Events: `graph`, `history`, `references`, `findings`, `selection`, `backpack`                                    |
| `GET /api/session`                                     | the state shared between clients: the selected node and the backpack                                                         |
| `POST /api/selection`                                  | `{"id": "f:src/main.go", "origin": "…"}`; the map follows                                                                    |
| `GET /api/backpack?format=`                            | the collected findings as `json`, `csv` or `md`                                                                              |
| `PUT /api/backpack`                                    | `{"items": [...], "origin": "…"}`; replaces the contents                                                                     |
| `POST /api/open`                                       | `{"path": "…", "line": 12}`; opens the file in the configured editor                                                         |
| `POST /api/settings`                                   | writes the `ui:` section of the configuration file                                                                           |

`origin` identifies the client that made the change and is echoed in the
resulting announcement, so that a client can distinguish its own change from
another's. The selection and the backpack belong to the session; the backpack's
durable store is the browser's, held per repository, and the page uploads it as
it loads.

**`/api/graph` carries an `ETag`**, so that a client already holding the document
may send `If-None-Match` and receive `304`. The tag is a fingerprint of the nodes
and edges rather than of the time at which they were read, so a re-analysis that
finds the same project yields the same entity — which is the question a client
reconnecting to a restarted server is in fact asking. The document reaches twenty
megabytes for a repository of a hundred thousand nodes, and in the majority of
requests nothing about it has changed.

**Every announcement carries an identifier**, and the stream opens with a
greeting stating its current position:

```json
event: hello
data: {"version":3,"etag":"\"9e4f…\"","seq":11,"resumed":true}
```

`resumed` is the stream's answer to the `Last-Event-ID` returned by the client:
nothing has been announced since the event it last received, so nothing has been
missed. Without it, a reconnection is indistinguishable from a first connection
and any announcement made while the connection was down is lost — and since a
browser retries an `EventSource` on its own, this occurs without the user's
knowledge.

### Opening files in an editor

The **Open in editor** button in the side panel's corner — labelled with the
editor, "VS Code ↗", and pinned beside maximize and close however far the panel
is scrolled — or `O`, opens the selected file at the line of the selected
symbol. The command is taken from `--editor`,
`DEPPHUNTER_EDITOR`, the user configuration or a `--config` file; failing those,
depphunter detects a graphical editor from `$VISUAL`, `$EDITOR` or `PATH` (VS
Code, Cursor, Zed, Sublime Text, the JetBrains IDEs and others). If none is
found, the button delegates to VS Code's `vscode://` URL handler. Under the [VS
Code extension](#vs-code-extension), the command is set to the editor the
extension is running in.

## VS Code extension

The extension displays the map in a tab beside the code. It starts `depphunter`
for the open folder, waits for the address it reports, and shows that address in
an editor tab of its own ([why](#why-the-map-is-in-a-tab-of-its-own)). The map
behaves exactly as it does in a browser tab, live updates included. It requires
VS Code 1.74 or later. Its source is under `extension/`.

### Install the extension

Install **depphunter** (`sarumaj.depphunter`) from the Visual Studio Marketplace
or [Open VSX](https://open-vsx.org/extension/sarumaj/depphunter); the registry
serves the build for your platform. Every
[release](https://github.com/sarumaj/depphunter-cli/releases) also provides one
`.vsix` per platform, each containing the binary for that platform. Select the
matching build — `linux-x64`, `darwin-arm64`, `win32-x64` and so on — and install
it with *Extensions: Install from VSIX…* in the command palette, or from a
terminal:

```sh
code --install-extension depphunter_1.2.3_vscode_darwin-arm64.vsix
```

**No further installation is required.** The extension and the server it starts
are produced by the same release and, for a release tagged `vX.Y.Z`, carry the
same version, so the two cannot diverge. A `_universal` build is also provided
for platforms not listed above; it contains no binary and falls back to
`depphunter` on `PATH`.

To run a different build — one under development, or a newer release on a machine
whose extension has not been updated — set `depphunter.path` to it. An explicit
setting always takes precedence over the bundled binary.

The binary the extension runs is also put first on `PATH` in the editor's
integrated terminals, so `depphunter` typed there — `depphunter --export html`,
say — is the same version as the map. Only terminals the editor opens are
changed, not the shell profile; ones already open are offered a relaunch.
Nothing is added where the binary is found on `PATH` anyway, and
`depphunter.addToPath` turns it off.

### Use

Select the depphunter icon in the activity bar. The **Maps** view lists the
window's folders, and any subfolder mapped from the explorer while its server
runs; selecting one maps it. The entry's buttons open the map and, while a
server runs, restart and stop it; the view's title bar opens the map in the
browser, shows the log and opens the settings. The same action is available in
the command palette as `depphunter: Open the Map` and in the explorer's context
menu for any folder.

| Command                                   | What it does                                                       |
|-------------------------------------------|--------------------------------------------------------------------|
| `depphunter: Open the Map`                | Maps the folder, or displays a map already produced                |
| `depphunter: Open the Map in the Browser` | Opens the same map outside the editor, for this instance           |
| `depphunter: Restart the Server`          | Restarts it, which is how changed settings take effect             |
| `depphunter: Stop the Server`             | Stops it; the next invocation analyzes afresh                      |
| `depphunter: Show the Server Log`         | The server's output, verbatim                                      |
| `depphunter: Show the Resolution Report`  | Which index each package resolved from, and how the walk proceeded |
| `depphunter: Export the Graph`            | JSON, GraphML, DOT or a self-contained HTML map                    |
| `depphunter: Export the Backpack`         | The collected findings as Markdown, CSV or JSON                    |
| `depphunter: Refresh the Side Panel`      | Reads the graph and the backpack from the server again             |
| `depphunter: Open Settings`               | The extension's settings, documented below                         |

One server is maintained per folder and kept until the window closes or the
server is stopped explicitly: analyzing a large repository takes time, and under
`--watch` it need happen only once. A status bar item is shown while a server is
running; selecting it opens the map. If a server exits unexpectedly, a warning
offers to show its log or restart it.

#### The panel beside the code

The same panel holds two further views below **Maps**, both showing the map
opened most recently:

- **Dependencies** presents the graph as a tree. A directory expands into its
  contents, a file into its imports, an island into its packages, and a package
  into its own dependencies, as far as `--resolve-depth` reached. Expanding a
  row requires no further request: every edge is already present in the graph
  the panel fetched once. A branch leading back to a node already expanded
  above it is shown once more, marked `↻`, and left collapsed, since
  dependency graphs contain cycles. A package that is not pinned, or that
  resolves from an index this machine does not configure, is marked in the
  list itself rather than only in its tooltip.

- **Backpack** holds the findings collected while walking the map, ordered by
  severity, with those absent from the most recent scan marked as resolved at
  the end. Removing an entry here removes it from the map's backpack as well.

The Dependencies view's title bar holds the resolution report, the graph export
and a refresh, and a file's row has a button that opens the file; the Backpack's
title bar exports it.

The two views and the map form a single interface: selecting a row selects the
corresponding building on the map, and selecting a building on the map expands
the tree to its row. The server is what makes this so — it holds the selection
and the collected findings while the map is open and announces changes to either
([API](#http-api)) — which is also why a second browser tab stays in step.

### Settings

The settings, in the groups the Settings editor presents them in. Each names the
flag it passes, and passes it only when set to something other than depphunter's
own behavior — an enum left at `default`, a number left empty, a switch left at
depphunter's default — so that a folder's `.depphunter.yaml` continues to govern
everything not set here.

| Setting                                                                        | Default   | Flag                | What it does                                                                                                   |
|--------------------------------------------------------------------------------|-----------|---------------------|----------------------------------------------------------------------------------------------------------------|
| **General**                                                                    |           |                     |                                                                                                                |
| `depphunter.path`                                                              | *(empty)* |                     | The binary to run. Empty selects the bundled binary, falling back to `depphunter` on `PATH`.                   |
| `depphunter.addToPath`                                                         | `true`    |                     | Put that binary first on `PATH` in the editor's terminals.                                                     |
| `depphunter.openIn`                                                            | `webview` |                     | `webview` (a dedicated tab), `simpleBrowser` (the built-in browser) or `externalBrowser` (the system default). |
| `depphunter.watch`                                                             | `true`    | `--watch`           | Re-analyze on file change and update the map.                                                                  |
| `depphunter.config`                                                            | `""`      | `--config`          | A configuration file to read instead of the folder's `.depphunter.yaml`, relative to the folder.               |
| `depphunter.editorCommand`                                                     | `""`      | `--editor`          | The command **Open in editor** invokes. Empty selects the current editor.                                      |
| `depphunter.args`                                                              | `[]`      |                     | Further arguments, one per entry, appended last. [Usage](#usage) lists them.                                   |
| **Analysis**                                                                   |           |                     |                                                                                                                |
| `depphunter.exclude`                                                           | `[]`      | `--exclude`         | Globs of paths to omit.                                                                                        |
| `depphunter.maxFileSize`                                                       | *(empty)* | `--max-file-size`   | Files larger than this many bytes are not read.                                                                |
| `depphunter.resolveDepth`                                                      | *(empty)* | `--resolve-depth`   | Levels of transitive dependencies to resolve from lock files; `-1` for all.                                    |
| `depphunter.online`                                                            | `false`   | `--online`          | Query package indexes and the OSV database over the network.                                                   |
| `depphunter.cache`                                                             | `true`    | `--no-cache`        | Read and write the analysis cache.                                                                             |
| `depphunter.history`                                                           | `true`    | `--no-history`      | Read git history for the history overlays.                                                                     |
| `depphunter.historyCommits`                                                    | *(empty)* | `--history-commits` | Read at most this many commits.                                                                                |
| `depphunter.private`                                                           | `[]`      | `--private`         | Globs naming the packages the organization owns. Never sent to a public index or to OSV.                       |
| `depphunter.trustIndexes`                                                      | `[]`      | `--trust-index`     | Index URLs to treat as configured on this machine, so a repository that names one is not marked.               |
| `depphunter.explain`                                                           | `false`   | `--explain`         | Write the [resolution report](#the-resolution-report) to the output channel whenever the map is built.         |
| **Appearance** (seeds: a repository that has saved a view of its own keeps it) |           |                     |                                                                                                                |
| `depphunter.style`                                                             | `default` | `--ui-default`      | `city`, `circuit` or `galaxy`.                                                                                 |
| `depphunter.theme`                                                             | `default` | `--ui-default`      | `auto`, `light` or `dark`.                                                                                     |
| `depphunter.colorBy`                                                           | `default` | `--ui-default`      | `language`, `size`, `commits`, `churn`, `age` or `authors`.                                                    |
| `depphunter.heightScale`                                                       | `default` | `--ui-default`      | `linear`, `sqrt` or `log`.                                                                                     |
| `depphunter.expandDepth`                                                       | *(empty)* | `--ui-default`      | Directory levels expanded initially; `0` picks for you, `-1` expands all.                                      |
| `depphunter.showStd`                                                           | `false`   | `--ui-default`      | Include standard-library islands.                                                                              |
| **Findings**                                                                   |           |                     |                                                                                                                |
| `depphunter.findings`                                                          | `[]`      | `--findings`        | Scanner reports to place on the map, relative to the folder. Globs permitted.                                  |
| `depphunter.vulns`                                                             | `true`    | `--no-vulns`        | Place reports on the map and, under `online`, query the OSV database.                                          |
| `depphunter.links`                                                             | `true`    | `--no-links`        | Follow the folder's Markdown links and report those that lead nowhere.                                         |
| **References**                                                                 |           |                     |                                                                                                                |
| `depphunter.lsp`                                                               | `false`   | `--lsp`             | Resolve symbol references using the installed language servers.                                                |
| `depphunter.lspTimeout`                                                        | `""`      | `--lsp-timeout`     | Time budget for the language servers, e.g. `90s`.                                                              |

What is left out is left out on purpose: `--addr`, `--no-open` and `--embed`
are how the extension hosts the map and are not for changing, and `--export`
(with `--output`) writes a file and exits rather than serving. `--python` has
no setting of its own; pass it through `depphunter.args`.

They are ordinary settings, so they can be set per workspace in
`.vscode/settings.json`:

```json
{
  "depphunter.style": "circuit",
  "depphunter.findings": ["reports/trivy.json", "reports/*.sarif"],
  "depphunter.exclude": ["vendor"],
  "depphunter.lsp": true,
  "depphunter.resolveDepth": 1
}
```

A server reads its settings at start-up, so a change takes effect on the next
`depphunter: Restart the Server`; the extension offers to perform the restart.
Anything the settings do not cover belongs in `depphunter.args` or in the
project's `.depphunter.yaml`, which the server reads as usual.

**Open in editor** opens the file at the line currently in view, in this editor:
the extension locates this editor's own command-line launcher (on Windows, its
executable) and passes it to the server, rather than leaving the server to find
whatever is on the `PATH` the extension host inherited.
`depphunter.editorCommand` overrides this where the file should be opened
elsewhere.

### Remote workspaces

Over SSH, WSL and dev containers the port is forwarded to `localhost` on the
local machine and the extension works unchanged. In Codespaces the forwarded
address is a public hostname, which the server rejects as a DNS-rebinding
attempt, since it answers only to `localhost`, `127.0.0.1` and `::1`; there, run
`depphunter` from a terminal instead. On vscode.dev without a remote the
extension does not load at all, since it needs a Node extension host.

### Why the map is in a tab of its own

The editor's built-in browser is the natural place for a local page, but it
places that page in a sandbox of its own, and the pointer lock is among the
capabilities that sandbox withholds. Walk mode therefore cannot capture the
mouse there, and since a sandbox can only be narrowed further down the frame
chain, the page has no means of recovering it.

The map is opened in a dedicated editor tab instead: one webview containing one
iframe, and that iframe carries no sandbox attribute. Adding none removes
nothing, so the pointer lock the editor granted the webview is preserved. What
is given up is an address bar and a back button, on a single page that requires
neither. `depphunter.openIn` may still select `simpleBrowser`, where walk mode
reports the limitation and turns the view by dragging instead.

### How the framing works

In either case the page is inside a webview and is therefore framed by origins
belonging to the editor. depphunter refuses to be framed by default, so the
extension starts it with `--embed`, naming every frame above the page — all
three, because `frame-ancestors` is evaluated against the entire chain and not
only the frame immediately containing the page:

```sh
--embed vscode-webview: --embed vscode-file: --embed https://*.vscode-cdn.net
```

- `vscode-webview:` is the webview, which is given an origin of its own for
  every session (`vscode-webview://<uuid>`), so there is no exact name to give.
- `vscode-file:` is the editor's window, served from `vscode-file://vscode-app`
  in the desktop editor.
- `https://*.vscode-cdn.net` is the webview in the browser build.

If any one of them is omitted, the browser refuses the page before it loads,
leaving an empty tab with the reason recorded only in the webview's own
developer tools. No other origin may frame the page; any that attempts to is
refused in the same way. The remaining effects of this mode are described under
[Inside an editor](#inside-an-editor).

In this mode the session token remains in the address rather than being
exchanged for a cookie, because a cookie set by the map would be a third-party
cookie within the frame and would never be returned. This is why the extension
reads the address from the server's own output rather than constructing it from
the port: that address is the only place the token appears. The extension always
starts the server in this mode, so even a map it opens in an external browser
keeps the token in the address.

## The map

The remainder of this document describes the map itself, which behaves
identically whether it was started by the command-line tool or by the extension.
Where a section names a flag, the extension passes it through its own setting if
one exists (see [Settings](#settings); `depphunter.style` and the other
appearance settings only seed the default) and through `depphunter.args`
otherwise.

### Keyboard & mouse

Panning is bounded at the point where the centre of the view lies a quarter of
the map's extent (plus a small margin) beyond its edge, and zooming out at the
point where the map occupies roughly a third of the view. In walk mode the
walker may travel 3 units out over the water and 12 units above the tallest
building.

|                           |                                                                                                                   |
|---------------------------|-------------------------------------------------------------------------------------------------------------------|
| Drag / right-drag / wheel | pan, orbit, zoom                                                                                                  |
| Click / double-click      | select, expand or collapse; double-click on open ground enters walk mode                                          |
| Middle-drag               | zoom                                                                                                              |
| `Enter`, `Backspace`      | expand or collapse the selection, select parent                                                                   |
| `→` `←` in the panel      | expand or collapse a dependency row                                                                               |
| `Enter` while reading     | close the details and resume                                                                                      |
| `E` `Q` in walk mode      | the next tool for the right hand, the left hand                                                                   |
| `Q` `E`                   | rotate by 90°                                                                                                     |
| `Home`                    | fit the map to the view                                                                                           |
| `R`                       | reset the view                                                                                                    |
| `+` `-`                   | expand or collapse one level throughout                                                                           |
| `/`                       | search files, symbols and packages                                                                                |
| `O`                       | open the selected file in the editor                                                                              |
| `P`                       | write the map to a PNG image                                                                                      |
| Legend click              | show or hide a language                                                                                           |
| Pin click                 | read the findings recorded on a building                                                                          |
| `⤢` in the details        | maximize the details over the map, or put them back beside it; remembered for the next file                       |
| Find in the details       | search a file's source: every match highlighted, `Enter`/`Shift+Enter` or `↓`/`↑` step through them, `Esc` clears |
| `+` beside a finding      | add it to the backpack                                                                                            |
| `B`                       | open the backpack                                                                                                 |
| `G`                       | open the photographs the camera has taken; from the street each can be put up on the camera and looked at there   |
| `X`                       | open the export menu                                                                                              |
| `K`                       | save settings to the config file                                                                                  |
| The figure                | the walker's last position in walk mode                                                                           |
| `Esc`                     | close the photographs or the backpack, or clear the selection                                                     |
| `V`                       | enter walk mode                                                                                                   |
| `?`                       | show all the controls                                                                                             |

In walk mode:

|                         |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
|-------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Mouse                   | look. The pointer is captured at the reticle; `Esc` releases it and a click on the map captures it again. Where it cannot be captured at all — a frame that withholds the pointer lock — walk mode reports this once, and a click then uses the tool rather than requesting the lock again                                                                                                                                                                                                                                                                                                                 |
| `R`                     | the tool wheel: every tool at once, the primary ones down its right side and the secondary ones down its left, with an empty left hand at the bottom. Hold `R`, point with the mouse and release; or tap `R` to leave it up and take what is under the cursor with `R`, `Enter` or a click. `Esc`, the right button, or releasing with the cursor still in the middle changes nothing. While it is up the walker is held where they stand and the city behind it is blurred: nothing moves them, spends a tank or bites them, so changing hands costs no time. Pointing at a tool already in hand keeps it |
| `1` `2` `3`             | select a secondary tool, or press the same key again to put it down. Only one is carried at a time                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `4` … `0`               | select a primary tool                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `E`                     | the next primary tool, cycling. The right hand is never empty                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `Q`                     | the next secondary tool, and after the last of them an empty left hand — which is how the walker comes down out of the air or steps off the water deliberately. Four presses return to the starting state                                                                                                                                                                                                                                                                                                                                                                                                  |
| `W` `A` `S` `D`/arrows  | move and turn; `Shift` runs, which spends the walker's wind                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `Space`                 | jump, which costs a little wind; while flying, ascend                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| Jet backpack in hand    | flight. While flying, `W` and `S` move along the view direction — looking down and pressing `W` descends — and `C` descends vertically. `F` (or middle click) opens the throttle for a burst. The view banks into turns and sideways moves, and levels out again on landing                                                                                                                                                                                                                                                                                                                                |
| Water skimmers in hand  | the surface of the water is walkable, passing under the bridges rather than over them; stowing them over deep water drowns the walker                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| Click                   | use the right hand; held down, the nail gun and the extinguisher keep firing. A module within reach is selected, and a bug that is caught is displayed and retained                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `F`, `C` / middle click | use the left hand. A secondary tool selects and catches nothing: the grapple hooks the building being looked at, the jet gives a burst of thrust. While flying, `C` descends instead                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `B`                     | the backpack, from the street as well as from the map                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `X` / `K`               | open the export menu; save the current view to the config file                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `H`                     | stow or draw both hands. A stowed tool remains functional and throws from the walker's eye                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Hold right button       | look through the scope                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `Enter`                 | show the details of whatever the reticle is on, as a second use of the tool would. The street softens around the building, which stays sharp with a little of what is around it. This releases the pointer; a click on the map resumes                                                                                                                                                                                                                                                                                                                                                                     |
| Wheel                   | zoom                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `+` `-` (or `[` `]`)    | planet radius, and therefore curvature                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `V` / `M` / `Esc`       | return to the map (`Esc` first releases a captured pointer). Re-entering walk mode restores the previous position                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |

The map draws the walker at their last position, as a figure facing the
direction they faced, and re-entering walk mode restores that position — unless
a node was selected on the map in the interim, in which case the walker is
placed at that node instead.

On foot the bay can be stepped down into and waded in, but deep water drowns a
walker without the water skimmers, and every island is reachable by bridge; the
walker may travel at most 3 units out over the water. The ground beneath the
walker is never a target, so aiming at the street selects nothing. While a bug
is being caught, the street softens around it for the second the catch takes,
so it is the one thing in focus. Expanding and
collapsing are reserved to the map view, since either rebuilds the entire city
and is disorienting from street level. The list of controls collapses once the
walker begins to move; `?` displays all of them.

### Styles

The same map, presented in three ways (`--style`, or the **Style** menu):

| Style     | What it is                                                                                                                                                                                                                                                                                                                                                                                                                     |
|-----------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `city`    | Buildings with facades and roofs, streets with crossings and parks, wooded shores, and bridges between the islands                                                                                                                                                                                                                                                                                                             |
| `circuit` | A printed circuit board: chip packages with rows of pins, heatsinks in place of tall buildings, copper traces along every street with vias set into them, solder pads and silkscreen around each part, capacitors in place of trees and lit LEDs in place of lamps. Beyond the edge of the board lies the backplane it is plugged into, which is live: charge travels along its tracks, indicating that it cannot be walked on |
| `galaxy`  | Platforms suspended in darkness: crystal spires with strata of light and star-like windows, joined by luminous conduits. In place of sea and sky there is the band of the galaxy with its dust lanes, two nebulae behind it and three layers of stars in front. The void the platforms hang in drifts in layers at three different speeds, so the map view redraws continuously under this style (and under `circuit`)         |

Only the environment differs. The colors that carry data — the language
palette, the history overlays, hover and selection — are identical in all three
(packages take a tint matching the style),
so a style alters the presentation and never the reading of the code. Nothing
else differs either: the same layout, the same streets, the same walk.

### Findings

depphunter runs no scanner; it reads the reports an existing pipeline has
already produced. Point `--findings` at the JSON emitted by CI — the flag is
repeatable and accepts globs — and each report is placed on the map:

| Tool                                  | written by                                         |
|---------------------------------------|----------------------------------------------------|
| `govulncheck -format json`            | the advisory, and the call site that reaches it    |
| `npm audit --json`                    | npm 7 and later, and the npm 6 advisory table      |
| `trivy … --format json`               | vulnerabilities, misconfigurations and secret hits |
| `osv-scanner --format json`           | lock-file scans                                    |
| `golangci-lint run --out-format json` | one finding per issue                              |
| `eslint -f json`                      | one finding per message                            |

The format is determined from the report's structure rather than from its file
name, so the names a pipeline assigns are immaterial:

```sh
govulncheck -format json ./... > reports/govulncheck.json
trivy fs --format json -o reports/trivy.json .
depphunter --findings 'reports/*.json'
```

Each finding is placed on the node it concerns: a vulnerability on the package
it affects, a linter's diagnostic on the file it refers to. A directory carries
the most severe finding beneath it. Severities are normalized onto one scale —
critical, high, medium, low, info — derived, for Trivy and OSV entries, from the
advisory's CVSS v3 vector where one is present (npm audit's own rating is used as
given), because the severity a distribution assigns frequently
disagrees with it; Trivy classifies CVE-2020-8203 as `MEDIUM` against a vector
scoring 9.8. A linter's "error" is deliberately not treated as a critical
advisory: golangci-lint and eslint findings are capped at medium, since
otherwise the streets would fill with bugs representing missing comments;
Trivy's misconfigurations and secret hits keep Trivy's rating. An advisory
govulncheck finds no call into is capped at low.

Under `--online`, depphunter additionally queries [OSV](https://osv.dev) for
every external package the map pins to a version — batched queries of 500
packages each, followed by the advisories it matched — covering Go, npm, PyPI,
crates.io, Maven, NuGet, GitHub Actions, Conan (as OSV's ConanCenter; vcpkg has
no OSV ecosystem), Composer (as Packagist), RubyGems, Swift packages (as
SwiftURL, by URL), pub, Hex (Elixir, Erlang and Gleam packages alike), CRAN,
Bioconductor, Hackage, opam and Julia; OSV
has no ecosystem for Terraform modules and providers, Buf Schema Registry
modules, CocoaPods, Carthage, LuaRocks, Wally, CPAN, Zig, Bazel modules and
repositories (the Maven, PyPI, Go, npm and crates.io packages Bazel's module
extensions install are asked about as such), Nix flake inputs, nixpkgs
packages or Elm packages. Floating packages
are not queried, since they resolve to a different version on the next
installation. Answers are cached for six hours.
`--no-vulns` disables all of this.

Viewed from above, every building carrying findings bears a **pin**, colored by
the most severe of them and growing taller with their number, so that the red
pins identify where to look next from across the map. Hovering over a pin gives
the count; selecting it displays the findings. The side panel lists them under
**Findings**, ordered by severity, each expanding in place to show the
description, the fixed version and the advisory link. A collapsed directory
lists the findings beneath it as well, so that a district marked red for
something several levels down can be reached from its pin.

In walk mode the findings appear in the streets: each is a **bug** patrolling
the building it belongs to, colored by severity and shaped by it — a
caterpillar for a critical finding, a beetle for the middle of the range, a
mite for a note. Catching one with the current primary tool — the net, the
bubble wand and the fire extinguisher are meant for it — displays what it
carries. The HUD reports how many remain, and a bug left uncaught bites: the
worse the finding, the more it costs.

A vulnerability govulncheck proved reachable is not a bug but a **fire**: the
package and the file that calls into it burn, on the map as well as in the
street, and the fire spreads to the files that import them until it is put out
with the extinguisher.

The **backpack** (`B`) is shared between the two views. Adding a finding with
the `+` beside it in the panel is equivalent to catching its bug in the street;
in both cases the bug stops moving. Its contents survive a re-layout, a depth
change and a reload. An entry is kept until it is cleared; once the scanners
stop reporting it, it is struck through rather than removed, so that a resolved
finding remains visible as such.

A repository is larger than it appears from within it, so the corner of the walk
HUD carries a **tracker**: a sweep centred on the walker and rotating with them,
with one dot per bug in its severity's color, one ring per module already
tagged with the tool, and an arrow at the rim for each bug or fire beyond its
range. The range
adapts to what remains, and beneath it are the distance to the nearest bug and
what that bug carries.

### Git history

In a git work tree, depphunter reads the history of the analyzed files — by
default the most recent 10,000 non-merge commits; `--history-commits` changes
the limit and `--no-history` disables it — in the background once the map is
displayed, and caches the result per commit. The **color** menu then offers:

| Mode          | color shows                                                                 |
|---------------|-----------------------------------------------------------------------------|
| Commits       | commits per file; the per-file mean for collapsed directories               |
| Lines changed | lines added plus lines deleted; the per-file mean for collapsed directories |
| Last change   | recency of the last change, with recent changes strongest                   |
| Authors       | the number of distinct authors                                              |

Files with no commits in range are given a separate neutral color. The
**Since** slider in the legend restricts commits, lines changed and authors to a
time range; tooltips and the side panel report the same figures, and the panel
additionally lists the principal authors. Renamed files retain the history of
their former names. Under `--watch`, a new commit updates the overlay.

### Versions and pinning

Every external package carries the version the project resolves it to — or,
where nothing resolves it, the range it is declared with — and whether anything
fixes it at that version. Lock files, exact specifiers
(`==1.2.3`, `RequiredVersion`), single-version ranges (`[1.2.3]`), commits and
digests pin a dependency; ranges, wildcards, snapshots and mutable tags do not.
A dependency that nothing pins is drawn in amber (violet under `galaxy`),
labelled **⚠ floating** in the side panel, and marked in its tooltip. Where a
lock file resolved a range, the panel reports both: `4.3.1`, requested as
`^4.2.0`.

| Ecosystem           | pinned by                                                                                                    | floats on                                                                                                         |
|---------------------|--------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| Go modules          | the version in `go.mod`, which the build selects; a script's `go install x@v1.2.3`                           | — (a `require` always names a version); a script's `@latest` or `@v1.2`                                           |
| npm                 | `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, an exact `1.2.3`                                         | any range, including `1.2`, which denotes 1.2.x                                                                   |
| crates.io           | `Cargo.lock`, a script's `cargo install x@1.2.3`                                                             | the manifest alone, where `"1.2.3"` denotes `^1.2.3`                                                              |
| PyPI                | `poetry.lock`, `uv.lock`, `pdm.lock`, `Pipfile.lock`, `==1.2.3`                                              | `>=`, `~=`, `^` and other ranges                                                                                  |
| Maven               | a plain version, `[1.2.3]`                                                                                   | ranges, `LATEST`, `RELEASE`, `-SNAPSHOT`, dynamic `1.+` and `latest.*`, unexpanded `${…}`                         |
| NuGet               | an exact version, `[1.2.3]`                                                                                  | wildcards (`2.*`) and ranges                                                                                      |
| PowerShell Gallery  | `RequiredVersion`                                                                                            | `ModuleVersion`, which is a minimum                                                                               |
| GitHub Actions      | a full commit SHA                                                                                            | tags, branches, or no ref                                                                                         |
| GitLab CI includes  | a commit                                                                                                     | tags, branches, templates, remote includes, or no ref                                                             |
| Container images    | an `@sha256:` digest                                                                                         | tags                                                                                                              |
| vcpkg               | an `overrides` entry                                                                                         | `version>=`, which is a minimum; no version and no baseline                                                       |
| Conan               | an exact reference (`zlib/1.2.13`), `conan.lock`                                                             | a version range (`[>=1.0 <2]`, `[~1.2]`)                                                                          |
| Composer            | `composer.lock`, `installed.json`, a bare `1.2.3` or `1.2`, `dev-main#<sha>`                                 | `^`, `~`, `*`, `1.2.*`, alternatives, ranges and branches                                                         |
| RubyGems            | `Gemfile.lock` (a git gem by its revision), a bare `1.2.3`, `= 1.2.3`                                        | `~>`, `>=`, `<`, `!=`, several requirements, no version                                                           |
| Swift packages      | `Package.resolved`, `exact:`, a bare `"1.2.3"`, `revision:`                                                  | `from:`, `.upToNextMajor`, `.upToNextMinor`, ranges, `branch:`                                                    |
| pub                 | `pubspec.lock` (a git package by its commit), a bare `1.2.3`, a git commit                                   | `^`, ranges, `any`, no constraint, a git branch or tag                                                            |
| Hex                 | `mix.lock`, `rebar.lock`, Gleam's `manifest.toml`, a bare `1.2.3`, `== 1.2.3`, a git `ref` commit            | `~>`, `>=`, `or` and `and` requirements, a git branch or tag                                                      |
| CRAN, Bioconductor  | `renv.lock`, `packrat.lock` (a GitHub package by its commit), `(== 1.2.3)`                                   | `(>= 1.2)`, no version, a `Remotes` branch or tag                                                                 |
| Hackage             | cabal's `plan.json`, `cabal.project.freeze`, `stack.yaml.lock`, `extra-deps`, `==1.2.3`, a repository commit | `^>=` and other ranges, no version, a repository tag or branch                                                    |
| Terraform modules   | a registry `version` of `1.2.3` or `= 1.2.3`, a git `ref` commit                                             | `~>` and other ranges, no version, a git tag or branch, no ref, an archive                                        |
| Terraform providers | `.terraform.lock.hcl` (the root module's, for the modules it calls), a single exact constraint               | `~>`, `>=` and other constraints, no constraint                                                                   |
| Buf Schema Registry | `buf.lock`, a commit ref (`:0123…`), a plugin's exact version                                                | a label, tag or branch ref, no ref, a plugin without a version                                                    |
| CMake FetchContent  | a `GIT_TAG` commit, a `URL_HASH`, an archive of a commit                                                     | a branch `GIT_TAG` (`main`, `origin/…`), no `GIT_TAG`, a download without a hash                                  |
| CocoaPods           | `Podfile.lock` (a git pod by its checkout commit), a bare `'1.2.3'`, `'= 1.2.3'`, a `:commit`                | `~>`, `>=` and other ranges, no version, a `:branch`, a git pod without a reference                               |
| Carthage            | `Cartfile.resolved`, `== 1.2.3`, a quoted commit                                                             | `~>`, `>=`, no requirement                                                                                        |
| LuaRocks            | `luarocks.lock`, `== 1.2.3` or a bare `1.2.3` in a rockspec (LuaRocks reads it as `==`)                      | `~>`, `>=` and other constraints, no version                                                                      |
| Wally               | `wally.lock`, `=1.2.3`                                                                                       | a bare `1.2.3` (a caret range in Wally), `^1`, other ranges                                                       |
| CPAN                | `cpanfile.snapshot` (Carton), `== 1.2` in a `cpanfile` or META prerequisites                                 | a bare `1.2` (a minimum in CPAN::Meta), `>= 1, < 2` and other ranges, `0` or no version                           |
| opam                | `*.opam.locked`, `dune.lock/`, `{= "1.2"}` or `(= 1.2)`, a `pin-depends` commit                              | `>= 5.6 & < 6` and other ranges, no constraint, a `pin-depends` branch or tag                                     |
| Julia               | `Manifest.toml` (and `Manifest-v1.11.toml`), `=1.2.3` in `[compat]`, a `[sources]` commit `rev`              | a bare `1.2` (a caret range in Pkg), `~1.2`, `>= 1`, `1.2 - 1.5`, no `[compat]` entry                             |
| Zig                 | a `.hash` in `build.zig.zon` (Zig verifies the download), a commit in the URL                                | a branch archive (`refs/heads/`), a URL without a ref or hash; a tag is shown, neither                            |
| Clojure (Maven)     | an exact `:mvn/version` or Leiningen version (Maven's rule), a full `:git/sha`                               | `RELEASE`, `LATEST`, ranges, snapshots; a `:git/tag` alone is shown, neither                                      |
| Bazel modules       | `MODULE.bazel.lock`, a `bazel_dep` version, `single_version_override`, an override's commit or `integrity`   | no version; a `git_override` branch; a `git_override` tag is shown, neither                                       |
| Bazel repositories  | an `http_archive` `sha256` or `integrity`, a `git_repository` commit, an archive of a commit                 | a branch (archive or `branch =`), no ref and no hash; a tag is shown, neither                                     |
| Nix                 | `flake.lock`, a commit (`rev=`, `/<commit>`) or `narHash` in the reference, niv and npins pins               | a branch (`nixos-24.05`, `refs/heads/`), a channel, `<nixpkgs>`, a registry name, no ref; a tag is shown, neither |
| Elm                 | an application's `elm.json` (exact versions of direct, indirect and test dependencies)                       | a package's `elm.json` ranges (`1.0.0 <= v < 2.0.0`)                                                              |

A package a shell script installs (`pip install`, `npm install -g`, `go
install`, `cargo install`, `gem install`) follows its ecosystem's row; one
installed without a version, or at `latest`, floats.

The JSON and GraphML exports carry `requested` and `floating` per package.

### Dependencies of dependencies

`--resolve-depth` extends the graph beyond what the code imports directly to
what those packages themselves require: `1` adds one level, `2` adds two, and
`-1` continues as far as the available information reaches. That information
comes from the lock files the repository already carries; nothing is fetched,
and the analysis remains offline.

| Lock file                                 | gives                                              |
|-------------------------------------------|----------------------------------------------------|
| `package-lock.json` (v1-v3)               | every installed package and what it requires       |
| `pnpm-lock.yaml` (v5-v9)                  | `packages:` and, since v9, `snapshots:`            |
| `yarn.lock` (classic)                     | each entry's resolved version and `dependencies`   |
| `Cargo.lock`                              | `dependencies` per crate                           |
| `uv.lock`, `poetry.lock`, `pdm.lock`      | each distribution's own requirements               |
| an installed Python environment           | each distribution's `Requires-Dist`                |
| `conan.lock` (Conan 1, `graph_lock`)      | the `requires` of each node                        |
| `composer.lock`, `installed.json`         | each package's `require`, without the platform     |
| `Gemfile.lock`                            | the dependencies listed under each spec            |
| `.build/checkouts/*/Package.swift`        | the checked-out package's `.package` lines         |
| `mix.lock`                                | each Hex package's requirements                    |
| `manifest.toml` (Gleam)                   | each package's `requirements`                      |
| `renv.lock`, `packrat/packrat.lock`       | each R package's requirements                      |
| `dist-newstyle/cache/plan.json`           | what each package of cabal's build plan depends on |
| `Podfile.lock`                            | the pods each pod's specs depend on                |
| `wally.lock`                              | each Wally package's dependencies                  |
| `cpanfile.snapshot` (Carton)              | each distribution's `requirements`                 |
| `dune.lock/` (dune package management)    | each package's `depends`                           |
| `Manifest.toml` (Julia, formats 1 and 2)  | each package's `deps`                              |
| `zig-pkg/<hash>/` or Zig's global cache   | a fetched package's own `build.zig.zon`            |
| `MODULE.bazel.lock` (before Bazel 7.2)    | the resolved module graph (`moduleDepGraph`)       |
| `maven_install.json` (rules_jvm_external) | each artifact's `dependencies`                     |
| `flake.lock` (versions 5 to 7)            | each input's own `inputs`, `follows` resolved      |
| `elm.json` of packages in `ELM_HOME`      | an installed Elm package's `dependencies`          |

Packages added in this way are marked **transitive**, meaning that no file in
the repository imports them. Edges between packages are of kind `depends`, as
distinct from the `import` edges that originate at a file, so that the count of
files importing a package remains exactly that. Two versions of one package
remain a single building, so an edge between packages is an edge between names.

A Python package that no lock file gives edges for (`Pipfile.lock` records none)
falls back to the installed environment (see [Languages](#languages)).
Ecosystems that keep the dependency graph outside the repository — Go modules,
NuGet, container images, a Composer, Bundler, Mix or R project that commits no
lock, pub, whose `pubspec.lock` is a flat list, rebar3, whose `rebar.lock`
records only a depth, and a Haskell project without cabal's build plan on disk
(`cabal.project.freeze` and `stack.yaml.lock` list versions only), Terraform
registry modules, pods no `Podfile.lock` records, rocks (`luarocks.lock` is a
flat list), CPAN distributions no `cpanfile.snapshot` records, opam
packages no `dune.lock/` records (an `*.opam.locked` is a flat list) and Julia
packages no `Manifest.toml` records, Gleam packages no `manifest.toml`
records, Elm packages the compiler has not installed in `ELM_HOME` (an
application's `elm.json` lists indirect packages flat), Maven artifacts of
Java, Kotlin, Scala
and Clojure builds (Maven, Gradle without its lock files, sbt, tools.deps and
Leiningen), and Bazel modules (a lock file since Bazel 7.2 records versions
only) — require `--online`, described below; the PowerShell Gallery, vcpkg,
Conan 2
(whose lock is a flat list), Bioconductor packages no lock records, Swift
packages that SwiftPM has not checked out under `.build`
(`Package.resolved` is flat as well) and Terraform modules fetched from git or
an archive are not resolved beyond the first level at present, and neither are
Buf Schema Registry modules: `buf.lock` is a flat list, and the registry's API
is not a package index depphunter asks. Content a CMake build fetches is not
resolved beyond the first level either, nor are Carthage dependencies
(`Cartfile.resolved` is flat), Wally packages no `wally.lock` records and Zig
packages Zig has not fetched into `zig-pkg/` or its global cache (there is no
Zig registry for `--online` to ask), nor Bazel's WORKSPACE repositories, nor
niv and npins sources (their `sources.json` is flat). A Terraform provider
depends on nothing.

The side panel presents these as a **tree**: every row under *Depends on* and
*Used by* expands into that node's own dependencies, and so on recursively.
Nothing is fetched, since the edges are already present in the map, so a row
expands immediately; `▸`/`▾` or the arrow keys expand and collapse it, and the
expansion state is preserved when a `--watch` update redraws the panel. A
package that depends on something which in turn depends on it is shown once
more, marked `↻`, and left collapsed, since lock files do contain cycles and a
tree following one would not terminate.

### Package indexes

Every external package records the index it comes from. depphunter reads both
the index configuration present on this machine and the configuration the
repository carries — `.npmrc`, including `@scope:registry`; `.yarnrc.yml`;
`pip.conf` and a requirements file's `--index-url`; Poetry and uv sources in
`pyproject.toml`; `NuGet.config`; a POM's `<repositories>` and the `maven`
repositories of Gradle build and settings scripts (not those of
`pluginManagement` or `buildscript`), other than Maven Central; the mirrors in
`~/.m2/settings.xml`; `.cargo/config.toml`; the `composer` repositories of
`composer.json` and of Composer's own `config.json`; a `Gemfile`'s `source`
lines (a `source ... do` block serves only its gems), `Gemfile.lock`'s
remotes, `~/.gemrc` and Bundler's rubygems.org mirror; a `pubspec.yaml`'s
`hosted:` servers, the servers `pubspec.lock` resolved from and
`PUB_HOSTED_URL`; `HEX_API_URL`; the repositories of `renv.lock` other than
CRAN and Posit Package Manager (each serving the packages recorded from it),
`options(repos = ...)` in `.Rprofile` and `~/.Rprofile`, and
`RENV_CONFIG_REPOS_OVERRIDE`; the `repository` stanzas of `cabal.project` and
of cabal's own configuration (`~/.cabal/config`, `~/.config/cabal/config`,
`CABAL_CONFIG`, `CABAL_DIR`) other than Hackage itself; a `Podfile`'s `source`
lines and the spec repositories of `Podfile.lock` (each serving the pods
installed from it) other than CocoaPods' own; the `rocks_servers` of a
project's `.luarocks/config-5.x.lua`, of `~/.luarocks/config-5.x.lua` and of
`LUAROCKS_CONFIG` other than luarocks.org; the Maven repositories of a
`deps.edn` or `bb.edn` (`:mvn/repos`), a `project.clj` or `build.boot`
(`:repositories`) and a `shadow-cljs.edn` other than Maven Central and
Clojars; the `--registry` lines of a `.bazelrc` and `~/.bazelrc` other than
the Bazel Central Registry; and `GOPROXY` — and the
side panel names the index each package resolves from. A container image
requires no configuration, since `ghcr.io/org/app` names its registry directly,
and neither does a Terraform module: `app.terraform.io/acme/vpc/aws` names its
registry, which is trusted when Terraform's CLI configuration names the host
(see [Authenticated registries](#authenticated-registries)).

The two sources are not treated alike. An index named by **this machine's** own
configuration is trusted. One that appears only in the repository is recorded
and marked **⚠ index**, because a repository directing a package manager at an
index that nothing here configures is the form a dependency-confusion attack
takes. No request is ever made to such an index, unless it is vouched for with
[`--trust-index`](#vouching-for-an-internal-index).

`--online` permits depphunter to query the trusted indexes for dependencies the
repository does not record, which is how `--resolve-depth` reaches the
ecosystems whose graph is held outside the repository:

| Ecosystem              | asked for                                                                                                                                                                                   | answer                                                                                               |
|------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| Go                     | `<proxy>/<module>/@v/<version>.mod`                                                                                                                                                         | its direct (non-`// indirect`) `require` entries                                                     |
| npm                    | `<registry>/<package>/<version>`, or `/latest` when unpinned                                                                                                                                | its `dependencies`                                                                                   |
| PyPI                   | `<host>/pypi/<name>/<version>/json`, or `/pypi/<name>/json` when unpinned                                                                                                                   | `requires_dist`, excluding extras                                                                    |
| crates.io              | `<index>/<se>/<rd>/<name>`, sparse index                                                                                                                                                    | its normal `deps`, excluding optional ones                                                           |
| NuGet                  | `<feed>/<id>/<version>/<id>.nuspec`                                                                                                                                                         | `<dependencies>`, both flat and by group                                                             |
| OCI                    | the manifest, then its config blob                                                                                                                                                          | the **base image** it was built on                                                                   |
| Composer               | `<repository>/p2/<vendor>/<name>.json` (`metadata-url` elsewhere)                                                                                                                           | the version's `require`, excluding the platform                                                      |
| RubyGems               | `<server>/info/<name>`, the compact index Bundler reads                                                                                                                                     | the version's runtime dependencies                                                                   |
| pub                    | `<server>/api/packages/<name>`, the package API pub reads                                                                                                                                   | the version's (or latest's) `dependencies`                                                           |
| Hex                    | `<api>/packages/<name>`, then `/releases/<version>` (or latest stable)                                                                                                                      | its requirements, excluding optional ones                                                            |
| CRAN                   | crandb's `/<name>/<version>` (or current); `src/contrib/PACKAGES` elsewhere                                                                                                                 | `Depends`, `Imports` and `LinkingTo`, without R's base packages                                      |
| Hackage                | `<server>/package/<name>/preferred`, then `/package/<name>-<version>/<name>.cabal` (or newest)                                                                                              | its libraries' `build-depends`, without GHC's own packages                                           |
| Terraform modules      | `<modules.v1>/<namespace>/<name>/<provider>/versions` (service discovery off the public registry)                                                                                           | the providers and registry modules of the version asked for, or the newest its constraint allows     |
| CocoaPods              | `<cdn>/Specs/<a>/<b>/<c>/<pod>/<version>/<pod>.podspec.json` (newest: the shard's version list)                                                                                             | its and its default subspecs' `dependencies`                                                         |
| LuaRocks               | `<server>/<rock>-<version>.rockspec`, versions from `<server>/manifest-5.1.zip` (read once)                                                                                                 | its run-time `dependencies`, without `lua`                                                           |
| CPAN                   | MetaCPAN's `<api>/v1/release/<distribution>` (the latest release), then `/v1/module/<module>` per dependency (once)                                                                         | its run-time requirements as distributions, without perl's own modules                               |
| opam                   | `<repository>/packages/<name>/<name>.<version>/opam`, opam-repository's files (a pinned version only)                                                                                       | its `depends`, without the compiler and what only tests or documentation need                        |
| Julia                  | `<registry>/<L>/<Name>/Versions.toml`, then `Deps.toml` and `Compat.toml` (General, or a depot registry on GitHub)                                                                          | the dependencies of the pinned or newest admitted release, with their compat ranges, without `julia` |
| Maven (group:artifact) | `<repository>/<group path>/<artifact>/<version>/<artifact>-<version>.pom` and its parents, the version from `maven-metadata.xml` when unpinned; Clojars after Central for a Clojure project | its compile and runtime dependencies, excluding optional ones                                        |
| Bazel modules          | `<registry>/modules/<name>/<version>/MODULE.bazel`, the newest version not yanked from `metadata.json` when unversioned (the Bazel Central Registry, or a `.bazelrc` `--registry`)          | its `bazel_dep`s, excluding dev dependencies                                                         |
| Elm                    | `<site>/packages/<author>/<name>/<version>/elm.json`, the newest release a range admits from `releases.json` when unversioned (package.elm-lang.org)                                        | its `dependencies` as ranges, excluding test dependencies                                            |

A container image has no dependency list. What it has is the image it was built
on, which is the source of its unpatched vulnerabilities, and that is what is
followed: the manifest — one platform's, where the manifest is a multi-platform
index — and then the small config blob it references, read for
`org.opencontainers.image.base.name` in the manifest's annotations or the image's
labels, for an image named in a pipeline, a Dockerfile or a Compose file alike.
No layers are downloaded. A registry requiring a pull token is given the
opportunity to say so, and the token endpoint it names is followed only over
HTTPS, or back to the registry's own host.

The PowerShell Gallery is not queried. Every Maven package on the map names its
artifact (`com.google.guava:guava`, `cheshire:cheshire`), so its POM is read,
whichever plugin placed it there; only a Bazel hub target that no artifact
list names is not asked about. Clojars, where Clojure's libraries are
published, is asked after Maven Central whenever the repository has a Clojure
manifest, as Leiningen and tools.deps do without being told to.

Lock files take precedence: an index is queried only where the repository is
silent, and an entire level of the walk is queried at once rather than one
package at a time. Answers are cached for one day under the cache directory.

#### Authenticated registries

An index behind authentication is read like any other, provided this machine is
already configured for it. Credentials are taken from the user's own files and
environment — never from the repository — and each is sent to the host it was
written for and to no other.

| Source                                                        | Holds                                                                                                             |
|---------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| `~/.npmrc`                                                    | `_authToken`, `_auth`, and `username` with `_password`, per registry                                              |
| `~/.netrc`, `~/_netrc`                                        | the machine/login/password triples git, curl, Go and pip already read                                             |
| `~/.m2/settings.xml`                                          | each `<server>`, matched to the `<mirror>` or profile `<repository>` in the same file                             |
| `~/.nuget/NuGet/NuGet.Config`, `~/.config/NuGet/NuGet.Config` | `<packageSourceCredentials>` (`ClearTextPassword`), matched to its `<packageSources>` entry                       |
| `~/.docker/config.json`                                       | stored `auths`, and the helpers named by `credsStore` and `credHelpers`                                           |
| `~/.config/containers/auth.json`                              | the same, for Podman and Skopeo                                                                                   |
| `~/.cargo/credentials.toml`                                   | a token per registry, matched to its index through `~/.cargo/config.toml` (legacy `credentials` and `config` too) |
| `CARGO_REGISTRIES_<NAME>_TOKEN`, `CARGO_REGISTRY_TOKEN`       | the same token supplied by a pipeline instead                                                                     |
| `~/.terraformrc`, `~/.tofurc`, `TF_CLI_CONFIG_FILE`           | Terraform's and OpenTofu's `credentials "<host>"` tokens; a `host` block names a registry without one             |
| `~/.terraform.d/credentials.tfrc.json`                        | the tokens `terraform login` stores (OpenTofu's under `~/.config/opentofu`)                                       |
| `TF_TOKEN_<host>`                                             | a token supplied by a pipeline, for HCP Terraform and the hosts named above                                       |
| the index URL itself                                          | `https://user:password@host/simple`, as a private pip or Cargo mirror is set                                      |

Between them these cover Nexus, Artifactory, Azure Artifacts, ProGet, GitHub
Packages, Harbor, GHCR, a private crate registry and a private Terraform
registry.

In `~/.npmrc`, `settings.xml` and `NuGet.Config`, a value that is exactly
`${NAME}`, `${env.NAME}` or `%NAME%` is read from the environment, so a password
may be held there. An encrypted password — Maven's `{...}` form,
NuGet's Windows-encrypted form — is left untouched: it cannot be decrypted here,
and transmitting the ciphertext would yield only a 401.

**A credential written into an index URL is removed from that URL before it is
recorded.** The index a package resolves from is drawn on the map, named in the
side panel and written into every export, and the HTML export is a file this
document recommends sharing. Only a URL supplied by this machine's own
configuration contributes a credential; one supplied by the repository is
stripped and discarded, since a repository able to supply a credential would
also be choosing where it is sent.

Most container registries no longer store a credential in `config.json`; they
name a helper instead, and depphunter runs it as `docker login` does —
`docker-credential-<name> get`, with the registry on standard input. This is the
only program depphunter executes that was not named on its command line, so the
helper's name must be a bare name, is resolved on `PATH` only, and is given ten
seconds; a configuration naming a path obtains nothing. A helper that returns an
identity token rather than a password is not used, since only registries accept
one.

### Private and internal dependencies

Most of an enterprise repository is the organization's own code, and two of the
operations depphunter performs for a public package must not be performed for
such packages:

- **querying a public index** for its dependencies, which does not answer and
  discloses to proxy.golang.org, or registry.npmjs.org, that the package exists;
  and
- **querying the OSV database** about it, which discloses the name and version
  of internal code to a third party.

Neither case can be inferred — a module path on a company host is
indistinguishable from any other — so the packages are declared:

```bash
depphunter --private 'corp.example/*' --private 'npm:@acme/*' .
```

A pattern is a glob with `GOPRIVATE`'s semantics: it matches a package whose
leading path elements match it, so that `corp.example/*` covers
`corp.example/team/billing`. It applies equally to an npm scope (`@acme/*`), a
Maven group (`com.acme.*`, which covers the artifact `com.acme.billing:api`;
`maven:com.acme:lib` names one artifact) and a registry path
(`harbor.corp/*`). Prefixing a
pattern with an ecosystem — `npm:`, `go:`, `maven:`, `nuget:`, `oci:`, `pypi:`,
`crates:`, `actions:`, `gitlab-ci:`, `psgallery:`, `c-external:`, `vcpkg:`,
`conan:`, `composer:`, `rubygems:`, `swiftpm:`, `pub:`, `hex:`, `cran:`,
`bioconductor:`, `hackage:`, `terraform-module:`, `terraform-provider:`,
`buf:`, `cmake-fetch:`, `pkg-config:`, `cocoapods:`, `carthage:`, `luarocks:`,
`wally:`, `cpan:`, `opam:`, `julia:`, `zig:`, `bazel:`, `bazel-repo:`, `nix:`,
`nixpkgs:` or `elm:` — restricts it to that ecosystem.

**`GOPRIVATE`, `GONOPROXY`, `GONOSUMDB` and `GONOSUMCHECK` are read in addition
to whatever is configured here**, so a Go project whose machine is already
configured requires no further setting.

A package matched in this way is drawn with a **private** label, is never named
to that ecosystem's public index, and is never sent to the vulnerability
database. It is still queried against an index *this machine* configures, since
an internal registry already knows of it, so a private registry continues to
answer for what its packages depend on. `private` may also be set in a
repository's own `.depphunter.yaml`; the only effect available to it is to make
depphunter disclose less, and the repository is the authority on which of its
dependencies are internal. A Python package installed from a directory, an
archive or a version-control URL is treated as private without any pattern (see
[Languages](#languages)).

### Vouching for an internal index

An index that appears only in the repository is marked **⚠ index** and never
queried, because a repository directing a package manager at an index that
nothing here configures is the form dependency confusion takes. In an
organization whose repositories carry their own `.npmrc` naming the company
registry, this marks every package, and a warning that is always present conveys
nothing.

`--trust-index` identifies an index as the organization's own:

```bash
depphunter --trust-index https://nexus.corp/repository/npm-group .
```

It may be set only in the user's own configuration file, the environment or on
the command line.
A repository cannot vouch for itself; were that permitted, the marking would
guard nothing.

| Setting         | Flag            | Environment                |
|-----------------|-----------------|----------------------------|
| `private`       | `--private`     | `DEPPHUNTER_PRIVATE`       |
| `trust_indexes` | `--trust-index` | `DEPPHUNTER_TRUST_INDEXES` |

Both are repeatable. A `--private` value may itself be a comma-separated list,
and both environment variables take one; a `--trust-index` value is a single
URL.

### The resolution report

None of the preceding resolution is visible in the map it produces. A package
resolving from an internal Nexus and one resolving from registry.npmjs.org are
drawn identically, and a dependency tree that terminates two levels down is
indistinguishable whether the dependencies end there, no lock file covers them,
or a proxy returned 404. The resolution report records the difference.

```bash
depphunter --resolve-depth 2 --online --explain .
```

It is a single account of one analysis, available in three forms:

| Form                            | Intended use                                                                 |
|---------------------------------|------------------------------------------------------------------------------|
| `--explain`, written to the log | a digest to read while the run is still in the terminal                      |
| `GET /api/resolution`           | the complete report as JSON, for comparison between runs or assertions in CI |
| `?format=md`, `?format=text`    | the same report rendered; the Markdown form is what the editor opens         |

The report records five things:

- **the indexes known to the run** — each index's URL, the scope it serves, and
  whether it was learned from this machine, from the repository, from the
  repository with `--trust-index` subsequently vouching for it, from a container
  image reference, or is the ecosystem's public default;
- **what resolved from where** — one row per index, giving the number of
  packages resolving from it and how many of those are private, so that an
  unexpected index is a single row rather than a search through the map;
  packages installed from outside any index form a row of their own;
- **the walk** — per ecosystem and per level: how many packages were queried,
  how many answered, how many were new, and the elapsed time; the answers line
  also counts the questions answered from what a Python environment has
  installed;
- **the ecosystems not walked at all** — when `--online` is not given, an
  ecosystem whose dependency graph is held outside the repository (Go modules,
  NuGet, Maven, container images) is named, rather than silently contributing
  nothing; and
- **the questions nothing answered** — every package for which no answer was
  obtained, with the reason: no lock file covers it; its index is named only by
  the repository; it is private and its index is the public one; the proxy
  requires a version it was not given; depphunter asks no index for this
  ecosystem, or cannot ask one because an import names no artifact; the package
  was installed from outside any index; or a request was made and returned a
  given status.

The last of these is the principal reason for the report. These reasons are
indistinguishable on the map, each drawing a package with nothing beneath it,
yet they mean entirely different things. A 404, or a 401 from a feed whose
credentials are wrong, is a configuration fault that the map can express only as
an absence.

```text
the walk past what the code imports
  PLUGIN      LEVEL  ASKED  ANSWERED  ADDED  EDGES  TIME
  go          0      25     13        6      27     120ms
  javascript  0      7      3         9      10     3346ms

answers
  87 asked; 3 from lock files; 56 from indexes (56 fetched, 0 cached, 0
  already asked); 28 unanswered (4 of them asked and failed); 89 requests; 159
  external packages on the map (87 transitive, 0 private, 0 from an index
  nothing here vouches for)

nothing answered for these
  COUNT  WHY
  15     depphunter asks no index for this ecosystem
  8      this ecosystem's index cannot be asked (an import names no artifact)
  1      https://registry.npmjs.org/@scope%2ftool/1.2.0: 404 Not Found
```

The JSON retains what the written report abbreviates: every question, up to 20
000, ordered by level, ecosystem and package, and for each one the URLs
requested, in the order made, with the status returned by each — which is how
the three round trips a container image requires can be distinguished when one
of them fails.

In the editor, **depphunter: Show the Resolution Report** opens the same report
as a document beside the code, and the `depphunter.explain` setting writes the
digest to the depphunter output channel whenever the map is built. Under
`--watch`, the digest follows a re-analysis only where the map actually changed;
a report after every saved file would obscure the one belonging to the change
under examination.

### CI pipelines

The code that executes with a repository's secrets is also a dependency, and it
is declared nowhere a package manager reads. depphunter takes it from the
pipeline files themselves — `.github/workflows/*.yml` and `*.yaml`,
`action.yml`/`action.yaml`, `.gitlab-ci.yml`, `*.gitlab-ci.yml` and any YAML
file under `.gitlab/` — and places it on the map
alongside the packages:

- **GitHub Actions**: each step's `uses:`, reusable workflows (`jobs.<id>.uses`),
  and a composite action's own steps. A `./path` resolves to the `action.yml` or
  workflow inside this repository, so a local action's own dependencies chain
  on.
- **GitLab CI**: every `include:` form - `local`, `project` (with `ref` and
  `file`), `template`, `remote` and `component` - plus the includes a bridge job
  triggers.
- **Container images**: `container:`, `services:`, `docker://…` and a Docker
  action's `runs.image` on GitHub; `image:`, `services:` and `default:` on
  GitLab, per job and pipeline-wide.

Jobs become the symbols of their file, so a pipeline expands into its jobs the
way a source file expands into its functions.

Pinning is stricter here than in a package ecosystem, because a reference that
can be rewritten is not a pin: **only a commit or a digest qualifies**.
`actions/checkout@v4` floats, since the tag may be moved to other code, and so
does `nginx:1.25.3`, since an image tag may be republished at its owner's
discretion. The hardening convention of pinning to a commit and recording the
version in a trailing comment is read as both: `actions/setup-go@3041bf5… #
v5.0.1` reports the commit as the version and `v5.0.1` as what was requested. A
GitLab template or a remote include names no version at all and may still change
beneath the repository, so it is likewise treated as floating.

### Container builds

A Dockerfile names the images an application is built on, and a Compose file
names the images it runs beside; both are dependencies no package manifest
records. depphunter reads `Dockerfile` and `Containerfile` under any casing,
the variants named after them (`Dockerfile.dev`, `api.Dockerfile`), and the
Compose files `compose.yaml`, `compose.*.yaml` and `docker-compose*.yml`, in
either `.yml` or `.yaml`:

- **Dockerfile**: the image of every `FROM`, `COPY --from=` and
  `RUN --mount=…,from=`, and the frontend a `# syntax=` directive names. A
  reference to an earlier stage, by name or index, is part of the build and not
  a dependency, nor is `scratch`. Named stages become the file's symbols.
- **Compose**: a service's `image:`, unless the service has a `build:`, in
  which case `image:` is only the tag of the result and the service points at
  the Dockerfile inside the repository that builds it, so that file's own base
  images chain on. Images of an inline Dockerfile and `docker-image://`
  build contexts are included; services become the file's symbols.

Build arguments are expanded from their defaults, so `ARG BASE=node:20` and
`FROM ${BASE}` name `node:20`, and Compose's `${VAR:-default}` likewise. A
value only `--build-arg`, the environment or an `.env` file supplies is not in
the repository and is not guessed: an image whose name depends on one is shown
as unresolved, and one whose tag depends on one keeps the tag as written.

The images join those of the CI pipelines in one **Container images** island,
with the same rule that only a digest pins, the same `oci:` private patterns
and the same base-image lookup under `--online`. Docker Hub's long names
(`docker.io/library/nginx`) are shortened to the name used everywhere else
(`nginx`), so one image is one building however it is written.

### Infrastructure as code

A Terraform or OpenTofu configuration installs modules and providers that
execute with the credentials of the cloud it manages, and none of them appears
in a package manifest. depphunter reads `.tf` and `.tofu` files (and their
`.tf.json` form), `.tfvars` files, `.terraform.lock.hcl` and Terragrunt's
`.hcl` files. A module is a directory, and its files are read together:

- **Modules**: a `module` block's `source`. A local path (`./modules/vpc`)
  is an edge to that directory; a registry address
  (`terraform-aws-modules/vpc/aws`, with a host for a private registry and
  `//subdir` for a submodule) and anything fetched from git, a web server or a
  bucket (`git::https://…?ref=v1.2.0`, `github.com/org/repo//sub`) join the
  **Terraform modules** island, the latter named by the normalized URL.
- **Providers**: `required_providers` entries, `provider` blocks and, where a
  module declares nothing, the provider a resource type implies
  (`aws_instance` uses `hashicorp/aws`), in the **Terraform providers**
  island. The lock file pins them for its module and the modules it calls.
  `registry.terraform.io/` and `registry.opentofu.org/` are dropped from
  names, since both registries serve the same namespaces.
- **Inside a module**: `var.x`, `local.x`, `module.x`, `data.t.n` and
  `aws_instance.web` link the referring file to the file declaring them, and
  `file()` and `templatefile()` with a literal path link to the file read.
- **Terragrunt**: the `terraform` block's `source` (with the locals of an
  included file substituted, as in Gruntwork's `_envcommon` layout),
  `dependency` and `dependencies` paths, and what `find_in_parent_folders()`
  finds.

Resources, data sources, modules, variables, outputs, locals and provider
configurations become the file's symbols. Only a commit pins a git module; a
registry module is pinned by an exact `version`. Nothing is evaluated, so a
source or version computed from variables is not followed.

### Nix

A Nix project states how it is built, what it builds with and which
revision of nixpkgs everything comes from. depphunter reads `.nix` files,
flakes (`flake.nix` and `flake.lock`) and the pins of niv
(`nix/sources.json`) and npins (`npins/sources.json`), without evaluating
anything:

- **Files**: `import ./x.nix`, `callPackage ./x { }` and a NixOS module's
  `imports = [ ./a.nix ./b ]` are edges to the file, a directory meaning its
  `default.nix`; any other relative path (`builtins.readFile ./VERSION`,
  `src = ./src`, `"${./script.sh}"`) is an edge to that file or directory.
  Text inside strings is not read, interpolations are.
- **Flake inputs**: each input of `flake.nix` is a package of the **Nix flakes
  and sources** island named by its URL: `github:NixOS/nixpkgs/nixos-24.05` is
  `github.com/nixos/nixpkgs` (GitHub names in lower case), `gitlab:`,
  `sourcehut:`, `git+https://…` and archives by their repository, FlakeHub by
  `flakehub.com/f/owner/repo`. `flake.lock` pins them at the locked commit
  (shown shortened, `ad57eef`), with the branch or tag asked for as the
  requested version; its nodes' own inputs, `follows` resolved, are what
  `--resolve-depth` follows. A `path:` input is an edge to that flake, a
  `follows` the followed input, and `inputs.x` in a module the flake's input.
- **Registry names and channels**: `<nixpkgs>`, `flake:nixpkgs` and an
  `outputs` argument no input declares name the **registry alias** (`nixpkgs`)
  rather than what each machine's `NIX_PATH` or flake registry makes of it, so
  they float. A NixOS channel's tarball is `nixpkgs` at that channel.
- **niv and npins**: each source is a package pinned by its revision or hash,
  and `sources.nixpkgs` (or `pins.nixpkgs`) in a file that imported the
  loader is an edge to it. `builtins.fetchTarball`, `fetchGit`, `fetchTree`
  and `getFlake` with literal arguments are named the same way.
- **Nixpkgs packages**: the attributes in `buildInputs`,
  `nativeBuildInputs`, `propagatedBuildInputs`, `checkInputs`, `packages`
  (`mkShell`, `home.packages`) and `environment.systemPackages` - `pkgs.jq`,
  `with pkgs; [ openssl zlib ]`, the arguments of a `callPackage`-style file -
  are packages of the **Nixpkgs** island, versioned and pinned by the
  project's nixpkgs input (or niv's or npins' `nixpkgs`). Inside nixpkgs
  itself they are its `pkgs/by-name` files.

Top-level `let` bindings and the attributes a file returns (paths cut at two
names) are the symbols, and a flake's outputs (`packages.default`,
`nixosModules.default`). The pinning rule is the one used elsewhere: a lock,
a commit or a content hash pins, a tag neither pins nor floats, a branch or
nothing floats. There is no Nix package index for `--online` to ask, and no
vulnerability database covers Nix.

### Gleam

Gleam compiles to Erlang and JavaScript and publishes to Hex, so its packages
are Hex packages: they share the **Hex** island, its pinning rule, OSV's Hex
advisories and the hex.pm client of `--online` with Elixir and Erlang, and a
package that a Gleam module imports and a `mix.lock` locks is one building.
depphunter reads `.gleam` modules, `gleam.toml` and the `manifest.toml` Gleam
writes beside it, without running gleam:

- **Modules**: `import a/b/c` (with or without `.{type T, f}` and `as c`) is
  an edge to `src/a/b/c.gleam`, `test/…` or `dev/…` of the importing package,
  or of a path dependency; a module of a package gleam downloaded into
  `build/packages/` names that package when the directory is on disk.
- **Packages**: other modules belong to the declared or locked package named
  by their leading segments joined by `_`: `lustre/element` is lustre,
  `gleam/erlang/process` gleam_erlang, `gleam/otp/actor` gleam_otp.
  `gleam/list`, `gleam/string` and the rest of the standard library are
  `gleam_stdlib`, a versioned package like any other, not a hidden island.
- **Externals**: `@external(erlang, "mod", "f")` is an edge to the project's
  `mod.erl`, an Erlang/OTP module, the package of that name or application
  (`hpack` is hpack_erl), or the compiled Gleam module `gleam@list` names;
  `@external(javascript, "./ffi.mjs", "f")` to that file, or to the package a
  path climbing out of the package names; a bare specifier is an npm package.
- **Manifests**: `gleam.toml`'s dependencies and `manifest.toml`'s packages
  are imports of what they name. The manifest pins, keeps a range asked for
  as the requested version, and gives `--resolve-depth` each package's
  requirements; without it `== 1.2.3` or a bare `1.2.3` pins, a git commit
  pins and a branch floats. A path dependency is an edge to its `gleam.toml`.

Functions, constants, types and their constructors (`Order.Cancelled`) are
the symbols. Elixir and Erlang code calling a compiled Gleam module
(`:gleam@list.map`, `gleam@list:map`) reaches the same file or package.

### Elm

Elm applications record the exact version of every package they install, and
packages declare ranges; the **Elm packages** island names them
`author/name`, as `elm.json` and package.elm-lang.org do. depphunter reads
`.elm` modules and `elm.json`, without running elm; `elm-stuff/` is not read:

- **Modules**: `import A.B` (with or without `as` and `exposing`) is an edge
  to `A/B.elm` under the source directories of the project the file belongs
  to: an application's `source-directories`, a package's `src/`, and
  `tests/` for elm-test. An examples application listing `../src` reaches
  the library's files; when several `elm.json` files list a file, the one
  whose own directory holds it comes first.
- **Packages**: a module of a package the compiler installed in `ELM_HOME`
  (else `~/.elm`) is that package, by its `exposed-modules`. Without it,
  elm/core's modules (`Dict`, `List`, `Task`, ...) are `elm/core`, a
  versioned package like any other, and other modules go to the listed
  package a curated table (`Html` elm/html, `Html.Styled` rtfeldman/elm-css,
  `Json.Decode.Pipeline` NoRedInk/elm-json-decode-pipeline) or the package's
  own name (`List.Extra` elm-community/list-extra) names. A module nothing
  names is dropped: its name does not say which author published it. The
  modules Elm imports by default are not edges.
- **Manifests**: every package `elm.json` lists — direct, indirect and test
  dependencies alike — is an import of it, and each source directory an edge
  to that directory. An application's exact versions pin; a package's ranges
  (`1.0.0 <= v < 2.0.0`) float. `--resolve-depth` follows the `elm.json` of
  packages installed in `ELM_HOME`, and `--online` asks package.elm-lang.org.

Functions and values, types and type aliases, the constructors of custom types
(`Msg.Clicked`), ports and `infix` operators are the symbols. OSV has no Elm
ecosystem, so Elm packages are not checked for advisories.

### Interface definitions

Protocol Buffers definitions are shared between services and languages, and
the protos they import from elsewhere — Google's API annotations, validation
rules, gRPC-Gateway's OpenAPI options — are dependencies no language manifest
records. depphunter reads `.proto` files and Buf's `buf.yaml`, `buf.work.yaml`,
`buf.lock` and generation templates (`buf.gen.yaml`, `buf.gen.*.yaml`):

- **Imports**: `import`, `import public` and `import weak` name a file
  relative to an import root. The roots are those Buf's configuration
  declares — the directories of a `buf.work.yaml`, the module paths of a v2
  `buf.yaml`, a v1 `buf.yaml`'s own directory — and, where no Buf
  configuration applies, the ones protoc is usually given: the repository
  root, `proto/`, `protos/`, `api/`, `src/main/proto/` and the importer's
  directory and its ancestors. Failing those, an import resolves to the only
  project file whose path ends in it (a copy under `third_party/`).
- **Well-known types**: `google/protobuf/*.proto` (`timestamp.proto`,
  `descriptor.proto` and the rest that protoc and buf ship) form a hidden
  **Protobuf well-known types** island.
- **Buf Schema Registry**: an import the project does not have resolves to the
  module `buf.yaml` declares in `deps` (or `buf.lock` records) that provides
  it, in the **Buf Schema Registry** island, named
  `buf.build/owner/repository`. A short table knows where common protos come
  from — `google/api/` and `google/type/` from
  `buf.build/googleapis/googleapis`, `validate/` from protoc-gen-validate,
  `buf/validate/` from protovalidate, `protoc-gen-openapiv2/` from
  grpc-gateway, `gogoproto/` from gogo — so a protoc project that declares
  nothing still names the module, marked unresolved; any other missing import
  is unresolved under its first directory.
- **Buf's files**: `deps`, lock entries, remote plugins
  (`buf.build/protocolbuffers/go:v1.35.1`) and module inputs are packages of
  that island; workspace directories and module paths are edges to those
  directories.

Messages (nested ones as `Outer.Inner`), enums, services, their rpc methods
(`Service.Method`), `extend` blocks, oneofs and the package become the file's
symbols. protoc's `-I` flags in a Makefile or script are not read, and a type
used from another file needs no edge of its own: protobuf requires importing
the file that declares it. Only `buf.lock`'s commit, or a commit given as the
ref, pins a module.

### Shell scripts

Build, CI, install and deployment scripts decide what runs as much as any
manifest, and they call each other. depphunter reads shell scripts — `.sh`,
`.bash`, `.zsh`, `.ksh`, `.bats`, Oh My Zsh's `.zsh-theme`, the shells'
startup files (`.bashrc`, `.zshrc`, `.profile` and the rest), direnv's
`.envrc`, and any file without an extension whose `#!` line runs `sh`, `bash`,
`zsh`, `dash`, `ksh`, `mksh` or `ash`, directly or through `env`:

- **Sourced files**: `source` and `.`, with the path worked out from what the
  file says — literals, variables it assigned earlier, and the idioms for the
  script's own directory: `$(dirname "$0")`, `${BASH_SOURCE%/*}`,
  `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`, zsh's
  `${0:A:h}`. `$(git rev-parse --show-toplevel)` is the repository root. A
  bare relative path is relative to the working directory, which a script
  cannot know; it is looked up beside the script, then at the root.
- **Scripts it runs**: a command that is a path (`./build.sh`,
  `"$DIR/deploy"`, also after `exec`, `env`, `sudo` or `time`), and the script
  an interpreter is handed (`bash x.sh`, `python3 tools/gen.py`,
  `node x.js`).
- **Paths below the environment**: `"$PLUGIN_PATH/common/functions"`, where
  the variable comes from whatever runs the script, resolves to the one
  project file ending in `common/functions` (two elements at least).
- **direnv and bats**: `source_env`, `source_up` and `dotenv` in an `.envrc`;
  `load` in a bats test.
- **Installed packages**: `pip install` (and `python -m pip`, uv, pipx),
  `npm install`/`pnpm add`/`yarn add`, `go install pkg@version`,
  `cargo install` and `gem install` add packages to the PyPI, npm, Go modules,
  crates.io and RubyGems islands the manifests use, under the same pinning
  rules, so vulnerability lookups and private patterns cover them;
  `pip install -r requirements.txt` is an edge to that file.

Functions, bats tests, aliases, and the exported, read-only and upper-case
variables a script sets at its top level become its symbols. Nothing is run:
a path under `~` or `$HOME`, one computed in a loop or by `eval`, or from a
variable another file sets is not followed, and packages installed with
`apt-get`, `apk`, `brew` or another system package manager are not read, as
no island holds them.

### Documentation

A README that links to `CONTRIBUTING.md` depends on that file, and one that
links to `internal/server/server.go` depends on that. Both break when the target
is moved, and no package manifest records the relationship, so the links are
placed on the map alongside the imports.

A link to a file or a directory in the repository becomes an edge from the
document to it, drawn as any other dependency is, and the headings become the
file's symbols: a document expands into its sections as a source file expands
into its functions. Inline links, reference definitions, autolinks and the
`href` and `src` attributes of raw HTML are all included, since a badge is also
a link. A link within a fenced block or a code span is not, since it is rendered
as text rather than followed.

There is no island for external hosts. A link to `https://example.test` is not a
dependency the map can characterize, and a legend of third-party domain names
would add nothing.

**A link that leads nowhere becomes a [finding](#findings)** instead:

| What is wrong                      | Reported as                |
|------------------------------------|----------------------------|
| the file or directory is not there | `link/missing-file`        |
| the heading it names is not in it  | `link/missing-anchor`      |
| the reference was never defined    | `link/undefined-reference` |
| the host says the page is gone     | `link/gone` (`--online`)   |

The first three require nothing beyond the repository, so they are checked on
every run and are exact: a path either names something on disk or it does not,
and a fragment either matches a heading of the file it points at or it does not.
A link to a file that exists but is absent from the map — ignored, excluded or
generated at build time — is not broken; it simply produces no edge.

The fourth requires a third-party server and is therefore performed only under
`--online`. Links are checked with the same credentials the indexes use, host by
host, so that a link into a private repository or an internal wiki is checked
rather than reported missing on the strength of an anonymous 404. The check is
otherwise deliberately conservative: a link is reported when a host states that
the page is **gone**, that is 404 or 410, and not when the host refuses a robot,
rate-limits, times out or fails. Those are the responses a
checker receives from Cloudflare and from GitHub's own bot rules, and treating
them as link rot would produce a finding for every link that functions correctly
in a browser. Links that could not be checked are reported in the log rather
than placed on the map. Answers are cached for one day.

Two categories of Markdown are excluded from all of the above, because a finding
against either would be a defect nobody is expected to correct: vendored
documentation, whose links point at the parts of its own repository that
vendoring does not copy, and fixtures under `testdata`, which are incorrect by
design.

`--no-links` disables the whole of it.

### Symbol references

Imports establish which files depend on which. Under `--lsp`, depphunter
additionally queries language servers for which symbols use which. It uses the
servers found on `PATH`, and the `go install` locations for gopls:

| Language                | Server                                                                                       |
|-------------------------|----------------------------------------------------------------------------------------------|
| Go                      | `gopls`                                                                                      |
| JavaScript / TypeScript | `typescript-language-server`                                                                 |
| Python                  | `pyright-langserver`, `basedpyright-langserver` or `pylsp`                                   |
| Rust                    | `rust-analyzer`                                                                              |
| Java                    | `jdtls`                                                                                      |
| Kotlin                  | `kotlin-language-server`                                                                     |
| Scala                   | `metals`                                                                                     |
| C#                      | `csharp-ls`                                                                                  |
| C / C++ / Objective-C   | `clangd`                                                                                     |
| PHP                     | `intelephense` or `phpactor`                                                                 |
| Ruby                    | `ruby-lsp` or `solargraph`                                                                   |
| Swift                   | `sourcekit-lsp`                                                                              |
| Dart                    | `dart language-server`                                                                       |
| Elixir                  | `elixir-ls` (or `language_server.sh`), `lexical` or `nextls`                                 |
| Erlang                  | `elp` or `erlang_ls`                                                                         |
| R                       | `R --slave -e languageserver::run()`                                                         |
| Haskell                 | `haskell-language-server-wrapper` or `haskell-language-server`                               |
| Terraform / OpenTofu    | `terraform-ls serve` or `tofu-ls serve`                                                      |
| Protocol Buffers        | `buf lsp serve`, `bufls serve` or `protols`                                                  |
| Shell (sh, Bash, bats)  | `bash-language-server start`                                                                 |
| CMake                   | `neocmakelsp --stdio` or `cmake-language-server`                                             |
| Lua                     | `lua-language-server`                                                                        |
| Luau                    | `luau-lsp lsp`                                                                               |
| Perl                    | `perlnavigator --stdio`, `pls` or `perl -MPerl::LanguageServer -e Perl::LanguageServer::run` |
| OCaml                   | `ocamllsp`                                                                                   |
| Julia                   | `julia --startup-file=no --history-file=no -e "using LanguageServer; runserver()"`           |
| Zig                     | `zls`                                                                                        |
| Clojure                 | `clojure-lsp`                                                                                |
| Bazel (Starlark)        | `starpls server`, `bazel-lsp` or `bzl lsp serve`                                             |
| Nix                     | `nil` or `nixd`                                                                              |
| Gleam                   | `gleam lsp`                                                                                  |
| Elm                     | `elm-language-server --stdio`                                                                |

The servers run in the background once the map is displayed — gopls requires
approximately 7 s for this repository — within the budget set by
`--lsp-timeout`; results are cached until the map's files, symbols or imports
change, or a different set of language servers is installed. Servers that index
slowly, rust-analyzer, jdtls, metals, clangd and the PHP, Ruby, Swift, Elixir,
Erlang, Haskell, Julia and Clojure servers in particular, may answer before
indexing has finished, so a first run can report fewer references than a later
one. The legend's **Imports / References** switch then determines what the
selection arcs and the side panel show: for a function, what it uses and what
uses it.
The JSON and GraphML exports include the reference edges.

### Languages

| Ecosystem               | Imports resolved through                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Islands                                                                    |
|-------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------|
| Go                      | every `go.mod` (multi-module, local `replace`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Go modules, Go standard library                                            |
| JavaScript / TypeScript | relative paths, `tsconfig`/`jsconfig` `paths`, workspaces, `package.json` + `package-lock.json` / `yarn.lock` / `pnpm-lock.yaml`; also the scripts of Vue, Svelte and Astro components, and SvelteKit's `$lib`                                                                                                                                                                                                                                                                                                       | npm, Node.js built-ins                                                     |
| Python                  | relative imports, `src/` layouts, requirements files, `setup.cfg`, literal `setup.py` lists, `pyproject.toml`, `Pipfile`, `poetry.lock`/`uv.lock`/`pdm.lock`/`Pipfile.lock`, installed environments (below)                                                                                                                                                                                                                                                                                                          | PyPI, Python standard library                                              |
| Rust                    | the module tree (`crate::`, `self::`, `super::`, `mod x;`), workspace and path crates, `Cargo.toml` (renamed and workspace dependencies) + `Cargo.lock`                                                                                                                                                                                                                                                                                                                                                              | crates.io, Rust standard library                                           |
| Java                    | source files by package path (any source root), `pom.xml` (properties, dependency management), Gradle scripts (string and map notation) and version catalogs, sbt builds; imports to the declared `group:artifact` shipping the package                                                                                                                                                                                                                                                                              | Maven, Java standard library                                               |
| Kotlin                  | source files by the package they declare (any directory; Java files by path), the Java manifests                                                                                                                                                                                                                                                                                                                                                                                                                     | Maven, Kotlin and Java standard libraries                                  |
| Scala                   | source files by the package they declare (any directory; Java files by path), `build.sbt` (`%`, `%%` with the `scalaVersion` suffix, versions held in a `val`), the Java manifests                                                                                                                                                                                                                                                                                                                                   | Maven, Scala and Java standard libraries                                   |
| C#                      | namespaces to project folders (`RootNamespace` + folder), `PackageReference`, `Directory.Packages.props`                                                                                                                                                                                                                                                                                                                                                                                                             | NuGet, .NET base library                                                   |
| C / C++                 | `#include` beside the includer, the include paths of `compile_commands.json` (`-I`, `-iquote`, `-isystem`, `/I`), `include/` and `src/`, and a unique project file ending in the included path; libraries by `vcpkg.json`, `conanfile.txt`, `conanfile.py` and `conan.lock`                                                                                                                                                                                                                                          | vcpkg, Conan, C/C++ external, C and C++ standard libraries, system headers |
| CMake                   | `add_subdirectory`, `include` of files and of modules on `CMAKE_MODULE_PATH`, target sources, `configure_file` templates, presets; `find_package` as the includes resolve, FetchContent, ExternalProject, CPM.cmake, `pkg_check_modules`                                                                                                                                                                                                                                                                             | vcpkg, Conan, C/C++ external, fetched content, pkg-config, CMake modules   |
| PHP                     | `use` statements (grouped, `function`, `const`) and fully qualified names in code, same-namespace `extends`/`implements`, `require`/`include` of spelled-out paths; project files by what they declare and `composer.json` PSR-4/PSR-0; packages by the autoload prefixes of `composer.lock` or `installed.json`                                                                                                                                                                                                     | Packagist, PHP standard library                                            |
| Ruby                    | `require`/`require_relative`/`load`/`autoload` of spelled-out paths on a guessed load path (`lib`, `test`, `spec`, gemspec require paths, path gems); gems by `Gemfile`, gemspecs and `Gemfile.lock`, whose `gem` lines are imports; Rails constants by Zeitwerk naming                                                                                                                                                                                                                                              | RubyGems, Ruby standard library                                            |
| Swift                   | `import` (every `#if` branch); `Package.swift` targets to their directories, else a directory named after the module; packages by `Package.swift`, `Package.resolved` and Xcode's `project.pbxproj`, products first, whose `.package` lines are imports; types used across a module's files                                                                                                                                                                                                                          | Swift packages, Swift standard library, Apple SDKs                         |
| Objective-C             | `#import`/`#include` as C includes (beside the file, `compile_commands.json`, `include/` and `src/`, a unique file ending in it), `@import`; framework headers and modules to pods by `Podfile`, `Podfile.lock` and podspecs, or to Carthage by `Cartfile` and `Cartfile.resolved`, whose entries are imports; headers under `Pods/` and `Carthage/` to their dependency                                                                                                                                             | CocoaPods, Carthage, Apple SDKs, C/C++ islands                             |
| Dart                    | `import`, `export` (every configurable URI), `part` and `part of`; relative URIs, `package:` URIs to a package's own `lib/`, path dependencies, pub workspace members and melos packages; packages by `pubspec.yaml` and `pubspec.lock`, whose dependencies are imports                                                                                                                                                                                                                                              | pub, Dart SDK libraries, Flutter SDK                                       |
| Elixir / Erlang         | Elixir `alias`/`import`/`require`/`use` and every module reference, Erlang remote calls, `-behaviour`, `-include`/`-include_lib`; modules to the files defining them (umbrella apps too); packages by `mix.exs`, `rebar.config`, `mix.lock` and `rebar.lock`, whose dependencies are imports                                                                                                                                                                                                                         | Hex, Elixir standard library, Erlang/OTP                                   |
| R                       | `library`/`require`/`requireNamespace`/`loadNamespace`, `pkg::`, pacman, `box::use`, roxygen `@import`; `source()` paths, knitr children; calls to a package's own functions across its files; packages by `DESCRIPTION`, `NAMESPACE`, `renv.lock` and `packrat.lock`, whose dependencies are imports                                                                                                                                                                                                                | CRAN, Bioconductor, R base packages                                        |
| Haskell                 | `import` (every CPP branch, PackageImports, `{-# SOURCE #-}`); modules to the files whose headers declare them (the importer's package, its project's and its dependencies' local packages), Happy/Alex sources by path; packages by `.cabal`, `package.yaml`, `cabal.project`, `stack.yaml`, the freeze file, `stack.yaml.lock` and `plan.json`, whose dependencies are imports                                                                                                                                     | Hackage, GHC libraries                                                     |
| Lua / Luau / Teal       | `require` (a string, `pcall(require, …)`, Luau paths and `.luaurc` aliases, Roblox instances), `dofile`/`loadfile`; modules to files by a rockspec's `build.modules`, else `?.lua`/`?/init.lua` under the file's directories and their `lua/`, `src/` and `lib/` and `.luarc.json`'s roots; Roblox instances through Rojo projects; rocks by rockspecs and `luarocks.lock`, Wally packages by `wally.toml` and `wally.lock`, whose dependencies are imports                                                          | LuaRocks, Wally, Lua standard library, Lua host runtimes                   |
| Perl                    | `use`/`no`/`require` (and in a string `eval`), `use parent`/`use base`, Moose's `with`/`extends`, Corinna's `:isa`, `require`/`do` of files; modules to `Foo/Bar.pm` under `use lib` (literal, FindBin, `__FILE__`, Mojo::File and Path::Tiny chains), the distribution's `lib/` and `t/lib`, each `lib/` above the file, else a file declaring the package; distributions by `cpanfile.snapshot`, `cpanfile`, `META.json`/`META.yml`, `Makefile.PL`, `Build.PL` and `dist.ini`, whose requirements are imports      | CPAN, Perl core modules                                                    |
| OCaml                   | module paths (`Foo.bar`, `open`, `include`, `module M = Foo`, functor arguments), `#require`; modules to the files of the importer's dune library, executable or test (`include_subdirs`, `modules`), to what an opened module of the project declares and to local libraries (`Lib.Module` of a wrapped one); libraries and packages by `dune` files, `dune-project`, `*.opam`, `*.opam.locked` and `dune.lock/`, whose dependencies are imports                                                                    | opam, OCaml standard library                                               |
| Julia                   | `using`/`import` (relative `.Sub`/`..Parent` through the `include` graph), `include`/`includet` (`joinpath(@__DIR__, …)`); the package's own `src/Name.jl` and submodules, local packages by UUID, `[sources]` or a manifest `path`; packages by `Project.toml` (`[deps]`, `[weakdeps]`, `[extras]`, `[compat]`, `[extensions]`, `[workspace]`) and `Manifest.toml`, whose entries are imports                                                                                                                       | Julia, Julia standard library                                              |
| Zig                     | `@import` of files (`@embedFile` too), `std`/`builtin`, `root` (the compilation's root source file) and module names to what `build.zig` wires (`b.addModule`, `b.createModule`, `.imports`, `addImport`, a dependency's `.module()`), else the `build.zig.zon` dependency of that name; `b.path()` in build code; `@cInclude` as C includes; packages by `build.zig.zon` (named by URL, pinned by `.hash`), whose dependencies are imports                                                                          | Zig, Zig standard library, C/C++ islands                                   |
| Clojure                 | `ns` `:require`/`:use`/`:import`, top-level `require`/`import`/`load` (prefix lists, every reader-conditional branch) to files under the source paths of `deps.edn`, `project.clj`, `shadow-cljs.edn`, `bb.edn` (`a.b-c` is `a/b_c.clj`), `:local/root` projects, Clojure's own namespaces, artifacts the manifests declare (a table and naming rules), JDK classes; npm strings in ClojureScript                                                                                                                    | Maven, Clojure standard library, JDK, npm, Node.js built-ins               |
| Bazel                   | `load()` and label attributes (`srcs`, `hdrs`, `deps`, `data`, ...) to files and to the BUILD file of each package, `glob()` expanded within the package; other repositories by `MODULE.bazel` (`bazel_dep`, overrides, `MODULE.bazel.lock`), WORKSPACE and `.bzl` repository rules (`http_archive`, `git_repository`, `local_repository`, `go_repository`), and the hub repositories of rules_jvm_external, rules_python, Gazelle, rules_js and rules_rust                                                          | Bazel modules and repositories, Maven, PyPI, Go modules, npm, crates.io    |
| Nix                     | `import`, `callPackage` and NixOS module `imports` of paths (a directory is its `default.nix`), other path literals; flake inputs (`github:`, `gitlab:`, `git+https:`, tarballs, `path:`, registry names, `follows`) pinned by `flake.lock`, `inputs.x`; `<nixpkgs>`; niv and npins sources; `builtins.fetchTarball`/`fetchGit`/`fetchTree`; nixpkgs attributes in `buildInputs`, `nativeBuildInputs`, `packages`, `systemPackages`                                                                                  | Nix flakes and sources, Nixpkgs                                            |
| Gleam                   | `import` (with unqualified lists and aliases) to the `src/`, `test/` and `dev/` modules of the package, of path dependencies and of `build/packages` when on disk; `gleam/*` to `gleam_stdlib`, `gleam_erlang`, `gleam_otp` and the like, other modules to the package their leading segments name; `@external` Erlang modules (compiled Gleam modules, `.erl` files, OTP, packages) and JavaScript files or npm packages; packages by `gleam.toml` and `manifest.toml`, whose dependencies and packages are imports | Hex, Erlang/OTP, npm                                                       |
| Elm                     | `import` to `A/B.elm` under the source directories of every `elm.json` listing the file (an application's `source-directories`, a package's `src/`, `tests/` for elm-test), kernel modules to their `.js`; other modules to the package whose installed `elm.json` in `ELM_HOME` exposes them, elm/core's modules to `elm/core`, else a curated module table or the listed package the module spells; packages by `elm.json`, whose dependencies are imports                                                         | Elm packages                                                               |
| PowerShell              | `using module`, `Import-Module`, dot-sourced and `&`-invoked scripts (`$PSScriptRoot`), `#Requires -Modules`, module manifests (`RequiredModules`, `RootModule`, `NestedModules`)                                                                                                                                                                                                                                                                                                                                    | PowerShell Gallery, built-in modules                                       |
| CI pipelines            | GitHub workflows and composite actions (`uses:`, reusable workflows, `container:`, `services:`), GitLab pipelines (every `include:` form, components, `image:`, `services:`)                                                                                                                                                                                                                                                                                                                                         | GitHub Actions, GitLab CI, Container images                                |
| Protocol Buffers        | `import` (`public`, `weak`) under the import roots of `buf.work.yaml` and `buf.yaml` (v1 and v2), else the repository root, `proto/`, `protos/`, `api/`, `src/main/proto/` and the importer's directories, else a unique project file ending in the path; modules by `buf.yaml` `deps` and `buf.lock` and a table of common protos; `buf.gen.yaml` remote plugins                                                                                                                                                    | Buf Schema Registry, Protobuf well-known types                             |
| Terraform / OpenTofu    | `module` sources to local directories, registry and remote modules; `required_providers`, `provider` blocks and resource type prefixes to providers, pinned by `.terraform.lock.hcl`; references to what other files of the module declare; `file()`/`templatefile()` paths; Terragrunt `source`, `dependency` and `find_in_parent_folders()`                                                                                                                                                                        | Terraform modules, Terraform providers                                     |
| Shell scripts           | `source`/`.` and scripts run by path or interpreter, with `$(dirname "$0")`, `${BASH_SOURCE%/*}`, `SCRIPT_DIR` variables, zsh's `${0:A:h}` and `git rev-parse --show-toplevel` evaluated; direnv `source_env`/`source_up`/`dotenv`, bats `load`; packages installed with pip, npm, pnpm, yarn, `go install`, `cargo install` and `gem install`                                                                                                                                                                       | PyPI, npm, Go modules, crates.io, RubyGems                                 |
| Dockerfile / Compose    | `FROM`, `COPY --from`, `RUN --mount from=` and `# syntax=` with `ARG` defaults expanded and stages told apart; Compose `image:`, and `build:` to the Dockerfile in the repository                                                                                                                                                                                                                                                                                                                                    | Container images                                                           |
| Markdown                | links to files and directories in the repository (inline, reference, autolink, and the `href` and `src` of raw HTML); headings become the file's symbols                                                                                                                                                                                                                                                                                                                                                             | *(none: a link is not a package)*                                          |

Python packages that no index has - an in-house package installed from a
directory, a wheel file or a Git repository - are resolved from what a Python
environment has installed. The environment is the interpreter named with
`--python` (`DEPPHUNTER_PYTHON`, or `python:` in the user's own configuration
file), otherwise the activated virtual environment (`VIRTUAL_ENV`), otherwise
the project's own `.venv` or `venv`; the per-user site directory (`pip install
--user`) is not read. Its site-packages directories are read, along with the
base interpreter's when the virtual environment includes system site-packages;
the interpreter itself is never run. A distribution counts only when its
metadata records that it was installed from somewhere other than an index
(`direct_url.json`, PEP 610). Such a package takes its name and version from its
installed metadata, and its dependencies from its `Requires-Dist`. It is treated
as private: it is never named to an index or to OSV, it is attributed to where
it was installed from rather than to any index (so it is never marked **⚠
index**), and the side panel says where that was. An import that an
index-installed distribution provides but no manifest declares remains
unresolved, but under that distribution's name. The extension has no setting for
this; pass `--python` through `depphunter.args`.

Java imports name packages rather than artifacts, so each is matched to the
declared Maven artifact that ships it, and the package is named
`group:artifact` — as POMs, Maven Central, OSV and Trivy name it, and as the
Clojure and Bazel plugins do, so a library several builds reach is one
building. The longest package prefix wins: one from a table of well-known
libraries (`com.google.common` is `com.google.guava:guava`,
`org.apache.commons.io` is `commons-io:commons-io`, `okhttp3` is
`com.squareup.okhttp3:okhttp`), one the artifact's name suggests
(`org.springframework:spring-context` gives `org.springframework.context`,
`com.fasterxml.jackson.core:jackson-databind` gives
`com.fasterxml.jackson.databind`, `cats-effect` gives `cats.effect`), or its
group. Among artifacts matching equally, the one whose group names the
import's root package wins (`liquibase` is `org.liquibase`'s, not an
extension's named after it), then the one whose name the import
spells (`io.ktor.client.engine.cio` is `ktor-client-cio`), then the
family's main one (`spring-boot`, a `-core`). A table artifact the build does
not declare still matches when another of its group is declared, since it
comes with it: `com.fasterxml.jackson.annotation` is `jackson-annotations`
beside `jackson-databind`, at its version, and a Spring Boot starter brings
`spring-boot`. A package the table gives an artifact that is not declared is
no other artifact's (`com.google.common.jimfs` is `com.google.jimfs:jimfs`, not
Guava's). An import nothing declared matches is unresolved, named after
the table's artifact (`javax.servlet:javax.servlet-api`) or guessed from its
package (`net.sf.saxon.s9api` becomes `net.sf.saxon:saxon`).

Vue, Svelte and Astro components are part of the JavaScript/TypeScript
ecosystem: the code in each component's `<script>` blocks (`<script setup>`,
Svelte's module script) and in an Astro component's `---` frontmatter is read as
JavaScript or TypeScript, by its `lang`, and resolved like any other; a
`<script src>` is an import too, and a component imported from a script, as
`./Button.vue`, is an edge to that file. Scripts that are markup rather than the
component's code - inside a Vue `<template>` or `<svelte:head>`, or an Astro
script left inline - are not read, nor is `@import` in a `<style>`. Each
component is a symbol named after its file, beside its functions and constants.
A `.ts` file that opens with an XML declaration or document type is a Qt
Linguist translation, not TypeScript: it is labelled XML and not parsed.

Kotlin and Scala share Java's resolution: the same manifests, the same Maven
islands, the same pinning rule, and the JDK. A Kotlin or Scala file need not sit
in a directory named after its package, so each is found by the `package` it
declares and the definitions starting in its first column; this index serves all
three languages, so a Java class may import a Kotlin one and the reverse.
`kotlin.*` and `scala.*` form their own standard-library islands; `kotlinx.*`
and the modules split from the Scala library, such as `scala.xml`, are ordinary
Maven dependencies. Scala imports are relative, so each is tried against the
packages around the file first, then against `scala._` unless a dependency owns
that root (`io.circe` is not `scala.io`); an import of a value's members
(`import builder._`) is not a dependency and is dropped, as is an import of a
class in the default package, which Gradle scripts declare. sbt's `%%` appends
the Scala binary version to an artifact's name, as Maven Central publishes it:
with `scalaVersion := "3.3.3"`, `"org.typelevel" %% "cats-effect"` is
`org.typelevel:cats-effect_3` (a `val` holding the version works too; the
file's own `scalaVersion`, else the root build's). Without a `scalaVersion` the
name stays as written, and `%%%` is read as `%%`, since the Scala.js or Native
platform suffix depends on the project.

C and C++ are one plugin, since their files include each other: `.c` files are
read as C and every other extension (`.h`, `.cc`, `.cpp`, `.cxx`, `.c++`,
`.hpp`, `.hh`, `.hxx`, `.h++`, `.ipp`, `.inl`) as C++, which covers nearly all
C headers too. Includes are read the way the preprocessor reads
them, line by line, and resolved in the compilers' order: `#include "x"` first
beside the including file; then the include directories of a
`compile_commands.json` (at the root or in a `build*/` or `cmake-build-*/`
directory, read from disk even when git ignores it), the file's own entry or,
for a header, every entry's; then `include/`, `src/` and the root; and last the
one project file whose path ends in the include (`foo/bar.h` in
`libs/foo/bar.h`), or the nearest one to the includer. Projects often include
their own headers with `<>`, so that form is looked up the same way, except
that a standard or system header is taken from the project only through the
compilation database. What the project does not have is a C or C++ standard
header (`<stdio.h>`, `<vector>`), a system header (POSIX, `sys/*`, Windows,
Apple frameworks, intrinsics), or else a third-party library named after its
first directory (`<boost/asio.hpp>` is `boost`, `<zlib.h>` is `zlib`), shown
unresolved; a quoted bare name the project lacks, such as a generated
`config.h`, is dropped. Only `#if 0` and `#if 1` are evaluated: the includes of
every other branch count, whatever the platform, and macros are not expanded,
so `#include CONFIG_H` is not followed.

A library that a vcpkg or Conan manifest declares takes the header's place
under its package: the manifests in the including file's directory and those
above it are searched, the nearest first (or, for a file under no manifest,
every manifest in the project, the shallowest first — sibling directories are
often built under one), for a package named like the header's library
(ignoring case and `-` against `_`), a known alias of it
(`gtest`/`googletest`, `nlohmann`/`nlohmann-json`, `Eigen`/`eigen3`,
`SDL2`, `GLFW`/`glfw3`, `google`/`protobuf`, `absl`/`abseil` and a few more),
or its name after `lib` (`<curl/curl.h>` is Conan's `libcurl`); a Boost header
belongs to the port of its directory (`<boost/asio.hpp>` to `boost-asio`)
before `boost`, and a Qt module's directory (`<QtCore/QString>`) to its own
port before `qtbase` and `qt`. vcpkg's `vcpkg.json` is read for its
dependencies and those of its features (names or objects with `version>=`;
platforms are not evaluated) and `overrides`; Conan's `conanfile.txt` for its `[requires]`
and tool sections, a `conanfile.py` for the string literals given to
`self.requires()` and its kin or assigned to `requires` and `tool_requires`,
and a `conan.lock` beside them in either Conan 1 or Conan 2 form, whose
libraries count as declared since the build installs them. The recipe is not
run, so a reference built at run time (an f-string) is not seen and a
conditional one counts whatever the condition. A vcpkg port pins only by an
override: a `builtin-baseline` fixes versions through the registry's history,
which the repository does not carry, so a port without a version is then
neither pinned nor floating, while without a baseline it floats. Everything
else stays in C/C++ external, without a version.

CMake builds — `CMakeLists.txt`, `*.cmake`, `*.cmake.in` package configuration
templates and `CMakePresets.json`/`CMakeUserPresets.json` — tie the build to
the code and the libraries. `add_subdirectory(dir)` is an edge to
`dir/CMakeLists.txt`; `include()` of a path to that file, of a module name to
`Name.cmake` on `CMAKE_MODULE_PATH` (as `set()` and `list(APPEND)` in the file
and the `CMakeLists.txt` above it build it), else to a module CMake ships
(`FetchContent`, `GNUInstallDirs`, `CTest`, the `Check*` modules) in a hidden
**CMake modules** island; the sources of `add_library()`, `add_executable()`
and `target_sources()` and `configure_file()`'s template are edges to those
files; presets' `include` and `toolchainFile` too. Paths are evaluated from
the file's variables, those the `CMakeLists.txt` files above it set, and
`CMAKE_CURRENT_SOURCE_DIR`, `CMAKE_CURRENT_LIST_DIR`, `CMAKE_SOURCE_DIR`,
`PROJECT_SOURCE_DIR` and `<Project>_SOURCE_DIR`; conditions and loops are not
evaluated, and binary-directory, environment and configure-time values are not
followed. `find_package(X)` lands where an `#include` of X's headers lands, so
the build file and the sources meet on one node: a vcpkg or Conan package the
manifests declare, else the C/C++ external library named after the include
directory (`ZLIB` is `zlib`, `nlohmann_json` is `nlohmann`, `Eigen3` is
`Eigen`, each Boost and Qt component on its own: `Qt6 Widgets` is
`QtWidgets`). Before that fallback come a project of that name in the
repository, the project's own `FindX.cmake`, and content it fetches under that
name; CMake's find modules for tools and the platform (`Threads`, `OpenMP`,
`Python3`, `Git`, `Doxygen`, `CUDAToolkit`) are CMake modules.
`FetchContent_Declare()`, `ExternalProject_Add()` and CPM.cmake's
`CPMAddPackage()` (`"gh:owner/repo@1.2.3"` or keywords) are packages of the
**CMake fetched content** island named by repository or download URL
(`github.com/google/googletest`; a GitHub release asset or archive by its
repository and ref), and `FetchContent_MakeAvailable(name)` links to the
declaration. A `GIT_TAG` commit or a `URL_HASH` pins; a tag is shown but can be
moved, so it neither pins nor floats; a branch or no tag floats.
`pkg_check_modules()` modules are the **pkg-config modules** island. Targets,
functions, macros, options, cache variables, projects and presets become
symbols.

PHP files (`.php`, `.phtml`, `.inc`) are read for their `use` statements,
grouped ones (`use App\{A, B as C}`) and `use function`/`use const`
included, and for the classes code names: those of `new`, `X::`, `extends`,
`implements`, a trait `use` and `catch`, qualified as PHP qualifies them (the
file's namespace, then its aliases), and fully qualified function calls
(`\f()`). A `require` or `include` counts when its path is spelled out:
string literals, `__DIR__`, `dirname(__FILE__)` and `dirname(__DIR__, n)`
joined with `.`; a relative path is looked up beside the file, then at the
project roots, as PHP's include path would. A name resolves to the project file
that declares it - read from every PHP file's `namespace` and definitions, so
classmaps and projects without Composer work - or by the PSR-4 and PSR-0 rules
of each `composer.json` (a path repository's package counts as the project's
own); then to PHP itself, grouped by extension (`Exception` is `core`,
`DateTime` is `date`, `PDO` is `pdo`); then to the Composer package whose
autoload prefix it falls under, which `composer.lock` records for every
installed package (`Symfony\Component\HttpFoundation\` is
`symfony/http-foundation`), or `vendor/composer/installed.json` where there is
no lock. The platform requirements (`php`, `ext-*`, `lib-*`) are not
packages. A namespace no prefix claims goes to the declared package its
segments name (`PHPUnit` is `phpunit/phpunit`, `Psr\Http\Message` is
`psr/http-message`), or else is shown unresolved under the name its first two
segments suggest. Composer reads a bare version as exact, so `1.2.3` pins even
without a lock; the lock pins everything it holds and gives
`--resolve-depth` its edges. A global function nobody here defines (a
framework helper loaded by a `files` autoload) is dropped, since its package
cannot be told from its name.

Ruby files (`.rb`, `.rake`, `.gemspec`, `.ru`, and `Gemfile`, `Rakefile`,
`Guardfile`, `Capfile`) are read for `require`, `require_relative`, `load` and
`autoload` whose path is spelled out: string literals, `__dir__`,
`File.dirname(__FILE__)`, `File.expand_path(path, base)`, `File.join` and `+`.
A `require` is looked up on the load path Bundler, Rake and RSpec would set
up: the `lib`, `test` and `spec` directories of the file's directory and those
above it, every gemspec's require paths, the `lib` of path gems, and a Rails
application's `app/*`; then in Ruby itself (`json`, `set`, `net/http`, `yaml`
as `psych`), unless the project declares or locks that library as a gem, and
then to a gem: `active_support` is `activesupport`, `rails` is `railties`,
`rspec/core` is `rspec-core`, and a path nothing declares is shown unresolved
under its first segment. A `Gemfile`'s `gem` lines and a gemspec's
`add_dependency` calls are imports of what they declare, since a Rails
application's gems are loaded by `Bundler.require` and seldom required by
name; a gem the repository builds itself is its gemspec. `Gemfile.lock` pins
every gem it holds (a git gem by its revision) and gives `--resolve-depth` its
edges; without it a bare `1.2.3` or `= 1.2.3` pins. In a Rails application
(`config/application.rb`), constants resolve to the files Zeitwerk loads them
from - every `app/*` directory and its `concerns` are roots, `lib` too under
`autoload_lib` - trying the modules around the reference innermost first, so
models, controllers and concerns are connected without a `require`.

Swift files are read for `import` declarations, with every branch of an `#if`
block read, since each is built on some platform. A module the project builds
resolves to its sources' directory: a target of any `Package.swift` at its
`path:` or `Sources/<name>` (`Tests/<name>` for tests), or, for an Xcode
project, whose targets are not read, a directory named after the module. The
toolchain's modules (`Foundation`, `XCTest`, `Glibc`) form a Swift standard
library island and Apple's frameworks (`SwiftUI`, `UIKit`, `Combine`) an Apple
SDKs island. Any other module is looked up among the packages the project
declares: the product a target takes from a package
(`.product(name: "NIOCore", package: "swift-nio")`, or an Xcode product
dependency), a table of well-known modules (`Logging` is swift-log), then the
package whose name the module's name spells (`Collections` is
swift-collections); a module nothing declares is shown unresolved. A package
is named by its URL without scheme or `.git` (`github.com/apple/swift-nio`),
the name OSV and Trivy use. `Package.resolved` (beside `Package.swift` or in
an Xcode project's `xcshareddata/swiftpm`) pins the packages it holds, and a
`Package.swift`'s `.package` lines are imports of what they declare. Since a
module's files see each other's declarations without imports, the type names
a file uses connect it to the file of its module, or of a project module it
imports, that declares them. A module no SwiftPM manifest provides may be a pod
or a Carthage framework: an app's Podfile or Cartfile is read for it too.
Swift is read by a small scanner: the tree-sitter grammar failed on about one
file in seven and spent up to three seconds on each of them.

Objective-C sources (`.m`, `.mm`, and `.h` files that show Objective-C in
their first lines: `#import`, `@interface`, `@protocol`, `@class`) share the
C/C++ plugin's include reading and resolution, `#import` included; a `.h`
file without those stays C or C++, and a `.m` file without a preprocessor
line, a `//` comment or an Objective-C keyword is MATLAB (or Mercury, by its
`:-` declarations) and is not read. An include or `@import` of an Apple
framework (`<UIKit/UIKit.h>`, `<objc/runtime.h>`) goes to the Apple SDKs
island Swift uses. A framework header of a pod (`<AFNetworking/AFNetworking.h>`,
`<SDWebImage/UIImageView+WebCache.h>`), a module (`@import Firebase;`) and a
bare header named like a pod (`"Masonry.h"`) resolve to the pod the nearest
`Podfile` or podspec declares or `Podfile.lock` pins, matched by name, its
module spelling (`libPhoneNumber_iOS`), without a platform suffix
(`lottie-ios` is `Lottie`) or by a table of modules named otherwise (`GRDB` is
`GRDB.swift`'s); a header found in a committed `Pods/` directory is its pod's,
not the project's. A `pod` line's subspec (`Firebase/Analytics`) is its pod,
`:path` pods are the project's own directories, and `Podfile.lock` gives every
pod's version and what it depends on. `Cartfile` entries are the Carthage
island, named by repository as Swift packages are (`github.com/Mantle/Mantle`)
and pinned by `Cartfile.resolved`. Classes, categories (`NSString(Shop)`),
protocols, methods by selector (`Cart.addItem:count:`), properties, C
functions, `NS_ENUM`s, typedefs, constants and macros are symbols. Objective-C
is read by a small scanner: the tree-sitter grammar took 19 to 36 ms per file
and failed on one file in twenty.

Dart files are read for their `import`, `export`, `part` and `part of`
directives; a conditional import contributes every URI it names (`if
(dart.library.io) 'io.dart'`), since each is compiled on some platform. A
relative URI resolves against the file, and `package:<name>/<path>` to
`lib/<path>` of the package named: the file's own (the nearest
`pubspec.yaml`), a path dependency or override (`pubspec_overrides.yaml`
included), a path package of `pubspec.lock`, a member of the same pub workspace
or a package of the same melos repository. Any other package comes from pub,
as `pubspec.lock` (read from disk when git-ignored; a workspace member's is
the root's) pins it or as `pubspec.yaml` declares it; `dart:` libraries form
a Dart SDK island and Flutter's own packages (`flutter`, `flutter_test`,
`flutter_localizations`, anything taken `sdk: flutter`) a Flutter SDK island.
A pubspec's dependencies are imports of what they declare, and a workspace's
members imports of their pubspecs. Dart is read by a small scanner rather than
the tree-sitter grammar, which on real Flutter code was slow and failed on one
file in ten.

<!-- cSpell: words behaviour -->
Elixir files are read for the modules they name — in `alias` (including
`alias Foo.{A, B}` and `__MODULE__`), `import`, `require` and `use`, and in any
other reference: a remote call, a struct, a behaviour — expanded through the
aliases in effect, those that the quote blocks of a used module of the project
inject (`use MyAppWeb, :controller`) and a Phoenix router's `scope` alias; and
for the Erlang modules they call (`:ets.new`). Erlang files are read for
`-include`, `-include_lib`, `-behaviour`, `-import` and remote calls
(`mod:fun`). A module resolves to the file that defines it — every
`defmodule` in the repository, umbrella applications included, and
`<module>.erl` — or, when the project defines only a prefix of it (generated
route helpers), to that prefix's file. Elixir's own modules form an Elixir
standard library island and OTP's modules and applications an Erlang/OTP
island. Any other module is attributed to a Hex package: the one that defines
it under `deps/` or `_build/` when those are on disk, else the package a
curated table or its name prefix names among those `mix.exs` and `rebar.config`
declare and `mix.lock` and `rebar.lock` lock (`Phoenix.LiveView` to
`phoenix_live_view`, `cowboy_req` to `cowboy`). The locks pin, and `mix.lock`'s
requirements give the edges between packages. Manifest dependencies and an
`.app.src`'s applications are imports of what they name. Both languages are
read by small lexers: the tree-sitter grammars were slower and lost Erlang
files to macros in patterns.

R scripts, packages and the R chunks of R Markdown and Quarto documents are
read for `library()`, `require()`, `requireNamespace()`, `loadNamespace()`,
`pacman::p_load()`, `box::use()`, every `pkg::` qualifier and roxygen
`@import` tags, and for the files `source()` (with `here::here()` or
`file.path()`), `box::use(./module)`, `targets::tar_source()` and a knitr
`child` pull in. A package name resolves to the importing package's own `R/`
directory, another package of the repository, R's base packages (and a
recommended package such as MASS or survival that the project neither declares
nor locks) as a hidden island, and otherwise to CRAN or Bioconductor as
`renv.lock` or `packrat.lock` pins it or the nearest `DESCRIPTION` declares
it. Files of one package import nothing from each other, so a call to a
function the package defines links the calling file to the defining one.
`DESCRIPTION` and `NAMESPACE` are imports of what they list. R is read by a
small lexer; the tree-sitter grammar parsed well but was about nine times
slower.

Haskell modules, literate modules (bird tracks and `\begin{code}`), boot and
hsc2hs files are read for their `import` declarations, PackageImports and
`{-# SOURCE #-}` included. A module resolves to the file whose header declares
it - in the importer's own package, then a package of its project or one it
depends on - and otherwise to the package that provides it: `base`,
`ghc-prim`, `template-haskell`, `ghc` and GHC's other own libraries as a
hidden island, anything else to Hackage, found by a curated module table and
by matching the module's name against the packages `build-depends` declares
(`Network.HTTP.Client` is `http-client`). cabal's build plan
(`dist-newstyle/cache/plan.json`), `cabal.project.freeze`, exact
`cabal.project` constraints, `stack.yaml.lock` and `extra-deps` pin; a range
floats, and a package a Stackage snapshot fixes shows the snapshot's name, as
its version is not known offline. `build-depends`, hpack dependencies,
`cabal.project` and `stack.yaml` packages and extra-deps are imports of what
they name. Haskell is read by a small lexer; the tree-sitter grammar was
several times slower and lost 18% of the files measured to CPP and extensions.

Lua, LuaJIT, Luau (Roblox's, in `.luau` and `.lua` files) and Teal files are
read for their `require` calls, `dofile` and `loadfile`. A module resolves to
the file a rockspec's `build.modules` maps it to, else to `?.lua` or
`?/init.lua` under the requiring file's directory, each directory above it and
their `lua/` (a Neovim plugin's), `src/` and `lib/`, then `.luarc.json`'s
library; the standard library and LuaJIT's modules are a hidden island, and so
are the modules host programs provide (Neovim's `vim.*`, LÖVE's `love.*`,
OpenResty's `ngx.*` and bundled `resty.*` libraries, Lune's `@lune/*`).
Anything else is a rock, found among what the rockspecs declare and
`luarocks.lock` pins by a curated table (`lfs` is luafilesystem, `ssl` luasec)
and the usual spellings of its name. Roblox's `require(script.Parent.X)` and
`game:GetService("ReplicatedStorage").Shared.X` are placed as Rojo builds the
game from its project files, Luau's `require("./x")`, `"@self/x"` and
`.luaurc` aliases by path, and a path through a `Packages` folder to the Wally
package `wally.toml` names so. Rockspec dependencies and `wally.toml` entries
are imports of what they name, and the top-level functions, methods, module
tables, exported fields and Luau and Teal types are the files' symbols. Lua is
read by a small lexer; the tree-sitter grammar took 3 to 5 ms per file and
failed on Roblox's `.lua` files, which are Luau.

Perl files (`.pl` unless it reads as Prolog, `.pm`, `.t`, `.psgi`,
`Makefile.PL` and scripts whose `#!` line runs perl) are read for `use`,
`no`, `require` and `do`, the classes `use parent`, `use base`, Mojo::Base,
Moose's `extends` and `with` and Corinna's `:isa` name, and Test::More's
`use_ok`. A module resolves to `Foo/Bar.pm` under the file's `use lib`
directories (literal, or computed from FindBin, `__FILE__`, Mojo::File's
`curfile` or Path::Tiny), its distribution's `lib/` and `t/lib`, the `lib/`
of each directory above it and the repository's; the modules perl ships are a
hidden island, unless a manifest requires one with a version or Carton's
`cpanfile.snapshot` installs it (a dual-life module such as List::Util).
Anything else is a CPAN distribution, named as MetaCPAN names it
(libwww-perl for `LWP::UserAgent`): the one `cpanfile.snapshot` says provides
the module, else the one the manifests (`cpanfile`, `META.json`,
`META.yml`, `Makefile.PL`, `Build.PL`, `dist.ini`) require the module or a
namespace above it from (`Plack::Request` is Plack's), else a distribution
named by a curated table or after the module, unresolved. The manifests'
requirements are imports, and packages, subs, constants and Moose attributes
are the files' symbols. Perl is read by a small lexer that knows POD,
here-documents and quote-like operators; the tree-sitter grammar took 5 to
23 ms per file and failed on 3 to 6% of the files measured.

OCaml files (`.ml`, `.mli`, ocamllex's `.mll` and Menhir's `.mly`) have no
imports: they name modules, and dune decides what a name means. Every module
path a file names (`Foo.bar`, `open Foo`, `include Foo`, `module M = Foo`, a
functor's argument; not a constructor such as `Some x`) resolves to the file
of the module in the same dune library, executable or test (its directory,
the tree `include_subdirs` adds, the modules its `modules` field selects), to
what a module of the project the file opens declares (`open Import`), to a
library of the repository the component uses (`Shop.Cart` of a wrapped
library, any module of an unwrapped one), to OCaml's standard library (a
hidden island; `List` after `open Core` is Core's), or to the opam package of
a library the component's `libraries` names (`lwt.unix` is lwt, `Lwt_io` is
lwt's). dune's `libraries` and ppx rewriters, dune-project's and opam files'
dependencies and `pin-depends` are imports of what they name, pinned by
`*.opam.locked`, dune's `dune.lock/`, `{= "1.2"}` or a pinned commit. The
top-level `let`s, types, modules, exceptions, classes and `val`s, and dune's
libraries and executables, are the symbols. OCaml is read by a small lexer;
the tree-sitter grammar took 3.7 to 7 ms per file and failed on 2 to 11% of
the files measured.

Julia files (`.jl`) name modules with `using` and `import` and pull files in
with `include`. A file is placed in a module through the `include` graph, so
a relative `using .Sub` or `..Parent` resolves to the file defining that
module; `using Shop` from the package's own tests, docs or extensions is its
`src/Shop.jl` (a submodule, `Shop.Cart`, the file defining it), and a
dependency the nearest `Project.toml` declares is a package of the repository
when its UUID, a `[sources]` path or the manifest's `path` says so, a
standard library (a hidden island, unless the manifest installed an
upgradable one such as Statistics from a registry), or a Julia package pinned
by `Manifest.toml` — found on disk when not committed, a versioned
`Manifest-v1.11.toml` first — or floating on its `[compat]` entry, where a
bare `1.2` is a caret range and only `=1.2.3` pins. `include("x.jl")` and
Revise's `includet` are edges to the file, `joinpath(@__DIR__, ...)`
included. `Project.toml`'s dependencies, extensions and workspace projects
and `Manifest.toml`'s entries are imports of what they name. Modules,
functions (each once, whatever its methods), macros, types, constants and
enums at the top level of modules are the symbols. Julia is read by a small
lexer; the tree-sitter grammar took 21 to 314 ms per file and failed on 6
to 28% of the files measured.

Zig files (`.zig`) name files, the standard library and modules with
`@import`. A path (`"cli/args.zig"`) and `@embedFile` are relative to the
importing file, `std` and `builtin` are the hidden standard library, and
`root` is the root source file of the compilation that reaches the file. A
module name resolves through the package's build code, read without running
it: `b.addModule` and `b.createModule` with a `root_source_file`, `.imports`
lists, `addImport` and `addAnonymousImport` (and Zig 0.11's `step.addModule`),
followed through variables, `if (...) |dep|` captures, struct fields and
function returns, to a project file, a `build.zig.zon` dependency's
`.module()` or a generated options module, which is dropped; unwired, it is
the dependency of that name. `b.path()` in build code is an edge to the file,
`b.dependency()` to the package. `@cInclude` inside `@cImport` resolves as a C
`#include` does. `build.zig.zon`'s dependencies are imports: a `.path` is a
directory of the repository, a `.url` a package named after its repository
(`github.com/ziglibs/known-folders`, from an archive or a `git+https` URL)
with the URL's commit or tag as its version, pinned by its `.hash`. Functions,
tests, containers (with their members as `Type.member`), constants and
variables at the top level are the symbols. Zig is read by a small lexer; the
tree-sitter grammar took 8 to 14 ms per file and failed on up to 7% of the
files measured.

Clojure, ClojureScript and babashka files (`.clj`, `.cljs`, `.cljc`, `.bb`,
and scripts whose `#!` line runs `bb`) name namespaces in the `ns` form's
`:require`, `:use` and `:require-macros` and in top-level `require` calls,
prefix lists included, and every branch of a reader conditional is read; `#_`
discards a form and `:as-alias` loads nothing. A namespace is the project file
at its path (`shop.db-util` is `shop/db_util.clj`, `.cljs` or `.cljc` by the
importer's platform) under the source paths of the manifests above the file -
`deps.edn` `:paths` and aliases' `:extra-paths`, `project.clj` `:source-paths`
and `:test-paths`, `shadow-cljs.edn` `:source-paths`, `bb.edn` `:paths` - and
of the projects their `:local/root` dependencies name, else any file whose
`ns` form declares it. `clojure.core`, `clojure.string` and the rest of
Clojure's own namespaces, ClojureScript's `cljs.*` and the Closure Library
(`goog`) are the hidden Clojure standard library, as are the libraries built
into babashka for its scripts; anything else is the Maven artifact a manifest
declares for it, found by a table of popular libraries (`ring.util.*` is
`ring/ring-core`, `honey.sql` is `com.github.seancorfield/honeysql`) and by
naming habits (`next.jdbc`, `cheshire.core`, `taoensso.timbre`,
`reitit.ring` from `metosin/reitit-ring`), else an unresolved one.
`:import` names JDK classes (the Java standard library), records and types of
the project's namespaces, Java files of the project, and classes of declared
artifacts, matched as Java imports are (see above: `com.google.common` is
`com.google.guava/guava`, `com.fasterxml.jackson.annotation` is
`jackson-annotations` beside a declared `jackson-databind`, at its version),
else by their coordinates; a class of a jar only a dependency brings is left
out rather than guessed; a
ClojureScript string require (`["react" :as react]`) is an npm package from
the `package.json` beside the build. The manifests' dependencies - `deps.edn`
and `bb.edn` `:deps` and aliases, `project.clj` `:dependencies`, profiles and
`:plugins`, `shadow-cljs.edn` and `build.boot` `:dependencies` - are imports:
Maven artifacts named `group:artifact` (`[ring "1.9.0"]` is `ring:ring`), git
dependencies by their lib name with their repository as origin, and
`:local/root` projects. Namespaces, top-level `def`, `defn`, `defmacro`,
`defmulti` and `defmethod`, protocols (with their methods), records, types
and `deftest` are the symbols. Clojure is read by a small reader; the
tree-sitter grammar took 3.7 to 6.6 ms per file (28 s for metabase).

Bazel's BUILD, `.bzl`, `MODULE.bazel` and WORKSPACE files are Starlark. A
`load()` and the labels of a target's label attributes (`srcs`, `hdrs`,
`data`, `deps`, `runtime_deps`, `exports`, `proto` and any attribute ending
in `deps`) name files of the repository - `"util.cc"`, `":util"`,
`"//lib:helpers"`, `"@//lib"` - which resolve to the file, else to the BUILD
file of the package (a rule, or a file a rule generates); `glob()` is
expanded against the package's files, not descending into subpackages. A
label of another repository (`@repo//pkg:x`) resolves to what declares it: a
`bazel_dep` in `MODULE.bazel` - a module of the Bazel Central Registry at the
version `MODULE.bazel.lock` selected or the one declared, or what a
`single_version_override`, `git_override`, `archive_override` or
`local_path_override` puts in its place - a WORKSPACE (or `.bzl` macro)
`http_archive` named by its URL and pinned by its `sha256`, a
`git_repository`, a `local_repository` directory, or the hub repository of a
module extension: `@maven//:com_google_guava_guava` and `artifact()` are the
Maven artifact `com.google.guava:guava` at the version `maven_install.json`
pins, `@pypi//requests` and `requirement("requests")` the PyPI distribution
of the requirements lock, `@com_github_pkg_errors//:errors` the Go module
`go_deps` read from `go.mod`, `//:node_modules/lodash` (rules_js) the npm
package `pnpm-lock.yaml` resolved, `@crates//:serde` the crate of
`Cargo.lock` - the same packages the other plugins name, so they meet on one
island. Repositories Bazel provides (`@bazel_tools`, `@local_config_cc`) are
hidden; one nothing declares is an unresolved module. `MODULE.bazel`'s and
WORKSPACE's declarations are imports themselves. Targets (`//pkg:name`,
kind = the rule or macro), `.bzl` functions, rules, providers and globals,
and the module's name are the symbols. Macros are not expanded. Starlark is
read by a small scanner; the tree-sitter grammar took 2.2 to 7.2 ms per file
(4.6 s for envoy's 1981 files).

Terraform and OpenTofu configurations, variable files, lock files and
Terragrunt configurations are read by a small HCL scanner: the tree-sitter
grammar parsed every file measured correctly but was about twenty times
slower. See [Infrastructure as code](#infrastructure-as-code).

Nix expressions are read by a small lexer and parser: the tree-sitter grammar
parsed almost every file measured correctly but took 1.9 ms per file, and 12 s
for nixpkgs' `python-packages.nix` alone. See [Nix](#nix).

Gleam modules are read by a small lexer: the tree-sitter grammar parsed 207 of
209 files measured correctly but took 4.9 ms per file, and everything needed
is token-level. See [Gleam](#gleam).

Elm modules are read by a small lexer too: the tree-sitter grammar parsed 411
of 412 files measured correctly but took 4 to 7 ms per file, and Elm's layout
rule puts every top-level declaration in column 0. See [Elm](#elm).

Protocol Buffers definitions are read by a small scanner: the tree-sitter
grammar took 2.4 to 3 ms per file and failed on every file using editions.
See [Interface definitions](#interface-definitions).

Shell scripts are read by a small scanner: the tree-sitter bash grammar took
2.7 to 3.6 ms per file, about twenty times longer, and failed on two thirds of
the zsh files measured. See [Shell scripts](#shell-scripts).

CMake files are read by a small scanner: the tree-sitter cmake grammar parsed
every file measured correctly but took 6.4 ms per file on average.

C and C++ definitions are read by a scanner too: the tree-sitter C and C++
grammars parsed more than half of the files of the projects measured with
errors, and grpc's generated protobuf tables ran each into the parse bound,
90 s for one analysis (3.5 s with the scanner).

Files in other languages appear on the map without dependency edges. Parsing
uses a pure-Go tree-sitter runtime for JavaScript/TypeScript, Python, Rust,
Java, Kotlin, Scala, PHP and Ruby; Go uses the standard library's own parser,
CI, Compose and Buf files a YAML parser, and C, C++, C#, PowerShell, Markdown,
Dart, Elixir, Erlang, R, Haskell, HCL, Protocol Buffers, shell scripts, CMake
files, Swift, Objective-C, CocoaPods and Carthage manifests, Lua, Luau, Teal and
LuaRocks files, Perl and its CPAN manifests, OCaml, dune and opam files, Julia,
Zig and `build.zig.zon`, Clojure and its EDN manifests, Bazel's Starlark files,
Nix expressions, Gleam and Elm modules, Dockerfiles, the markup of Vue,
Svelte and Astro components, R Markdown chunks and C preprocessor directives
small built-in scanners — so the binary continues to cross-compile without a
C toolchain.

## Security

The server binds to loopback by default and prints a URL containing a random
token, which the browser exchanges for a cookie. Requests lacking the token,
requests carrying a foreign `Host` header while bound to loopback — that is,
DNS rebinding — and
requests for files outside the analyzed project are all rejected. Nothing may
frame the map: `X-Frame-Options: DENY` and `frame-ancestors 'none'`. A file's
raw bytes, which the side panel previews pictures, clips and recordings from,
are served only under a known image, video or audio type or as
`application/octet-stream`, and always with `default-src 'none'; sandbox`, so
nothing in a repository can run as the map.

The side panel shows a file's source; a binary file's content stays hidden
behind a button that shows its first 64 KB as a hex dump, since its bytes are
rarely worth reading and there can be a great many of them. For such a file the
corner's open button becomes **Hex editor ↗**: in the VS Code extension it opens
the file in Microsoft's Hex Editor, which edits the bytes and their text side by
side, and offers to install it if it is missing. A launcher on the command line
cannot ask for one, so elsewhere the file opens as it is, and the status line
says how to reopen it in the Hex Editor. depphunter itself never writes to the
repository.

### Inside an editor

That last provision is also what prevents an editor from displaying the map in
its own built-in browser, which is a frame like any other. `--embed <origin>`
permits the origins it names and no others; it is what a wrapper such as an
editor extension passes when it starts the server, naming every frame above the
page, since `frame-ancestors` is evaluated against the entire chain rather than
the immediate parent (see [How the framing works](#how-the-framing-works)).
Three consequences follow, and nothing else changes:

- `frame-ancestors` names those origins rather than `'none'`, and
  `X-Frame-Options` is not sent, since it provides no means of naming an origin
  that browsers still honour.
- The token remains in the address rather than being exchanged for a cookie. A
  cookie set by the map is a third-party cookie within another origin's frame,
  and browsers do not return those; the page reads the token from its own URL
  and supplies it on every call, in a header, or in the query string for the
  event stream, which cannot set headers. `Referrer-Policy: no-referrer` keeps
  it out of any outgoing request.
- The interface's static files — the page, `app.js` and its imports, styles and
  models — are served without the token, since a frame cannot attach one to a
  `<script src>`. They are identical in every release and disclose nothing about
  the project. Everything under `/api`, and the entry address `/`, still
  requires it.

The option is deliberately a flag and nothing else: no configuration file and no
environment variable can enable it, so a repository cannot arrange to be framed
by a page of its choosing. Each origin is validated before it reaches the
header, so nothing passed on the command line can terminate the directive early
or begin another.

## Contributing

Building and testing, the structure of the code, and working on the VS Code
extension are documented in [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[BSD 3-Clause](LICENSE) © 2026 Dawid Ciepiela. The embedded three.js (MIT),
highlight.js (BSD 3-Clause), potpack (ISC) and fzf-for-js (BSD 3-Clause), and
the 3D models built from webxr-input-profiles (MIT) and a low-poly nature pack
(CC0), retain their own licenses; see
[web/static/vendor](web/static/vendor/README.md).
