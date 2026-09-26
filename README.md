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
  shot over the edge is the way down; the jet backpack flies; the water
  skimmers make the bay walkable, passing under the bridges rather than over
  them and stepping back up onto a shore that stands half a unit above the
  water. The latter two run on a tank, which empties only while the tool is
  doing its work and fills again whenever it is not, so neither is a way of
  getting everywhere; a gauge beside the health bar says what is left, running
  dry in the air is a fall, and a tool that has run out stays stopped until it
  is put away and taken out again. One tool of each kind is carried at a time,
  one to a hand — the
  primary in the right, the secondary in the left — so the map can be flown
  over and its bugs netted without putting either down. A click uses the right
  hand, and `F`, `C` or the middle button the left. The HUD lays the slots out
  the way the walker is: what the left hand carries on the left and what the
  right hand hunts with on the right, each group behind a small hand of its
  own, because a row read at a glance in the middle of something else should
  not have to be parsed. The rod
  also pulls on its line, drawing the walker to the wall it struck. The right
  button holds the scope, and the mouse wheel zooms the view.

  Running and jumping are paid for out of a second gauge, the walker's **wind**.
  A sprint drains it in a few seconds and each jump takes a little more; it
  fills again while walking or standing, and a walker who has run it out has to
  get a quarter of it back before they can run or jump again, so the way across
  a district is a series of dashes rather than one long one. Flying costs nothing:
  that is the jet's tank, not the walker's chest.

  The walker has a **health bar**. A fall of more than about ten storeys costs some
  of it, a bug's bite costs more the worse the finding is, and deep water with
  nothing to float on takes all of it in a couple of seconds — so stowing the
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
  [CI pipelines](#ci-pipelines) · [Documentation](#documentation) ·
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

| Endpoint                                               | What it is                                                                                |
|--------------------------------------------------------|-------------------------------------------------------------------------------------------|
| `GET /api/graph`                                       | the graph document: nodes, symbols and edges                                              |
| `GET /api/config`                                      | the view the map opens with, and the server's capabilities                                |
| `GET /api/file?path=`                                  | the source of a file present on the map, and of no other file                             |
| `GET /api/history`, `/api/references`, `/api/findings` | computed in the background: `202` while in progress, `204` if there is no result          |
| `GET /api/export?format=&ui=`                          | `json`, `graphml`, `dot` or `html`; `ui` is the view (JSON) an `html` export opens with   |
| `GET /api/resolution?format=`                          | how the dependencies were resolved: `json` (default), `md` or `text`                      |
| `GET /api/events`                                      | Server-Sent Events: `graph`, `history`, `references`, `findings`, `selection`, `backpack` |
| `GET /api/session`                                     | the state shared between clients: the selected node and the backpack                      |
| `POST /api/selection`                                  | `{"id": "f:src/main.go", "origin": "…"}`; the map follows                                 |
| `GET /api/backpack?format=`                            | the collected findings as `json`, `csv` or `md`                                           |
| `PUT /api/backpack`                                    | `{"items": [...], "origin": "…"}`; replaces the contents                                  |
| `POST /api/open`                                       | `{"path": "…", "line": 12}`; opens the file in the configured editor                      |
| `POST /api/settings`                                   | writes the `ui:` section of the configuration file                                        |

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

The side panel's **Open in editor** button, or `O`, opens the selected file at
the line of the selected symbol. The command is taken from `--editor`,
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

|                           |                                                                                                                 |
|---------------------------|-----------------------------------------------------------------------------------------------------------------|
| Drag / right-drag / wheel | pan, orbit, zoom                                                                                                |
| Click / double-click      | select, expand or collapse; double-click on open ground enters walk mode                                        |
| Middle-drag               | zoom                                                                                                            |
| `Enter`, `Backspace`      | expand or collapse the selection, select parent                                                                 |
| `→` `←` in the panel      | expand or collapse a dependency row                                                                             |
| `Enter` while reading     | close the details and resume                                                                                    |
| `E` `Q` in walk mode      | the next tool for the right hand, the left hand                                                                 |
| `Q` `E`                   | rotate by 90°                                                                                                   |
| `Home`                    | fit the map to the view                                                                                         |
| `R`                       | reset the view                                                                                                  |
| `+` `-`                   | expand or collapse one level throughout                                                                         |
| `/`                       | search files, symbols and packages                                                                              |
| `O`                       | open the selected file in the editor                                                                            |
| `P`                       | write the map to a PNG image                                                                                    |
| Legend click              | show or hide a language                                                                                         |
| Pin click                 | read the findings recorded on a building                                                                        |
| `+` beside a finding      | add it to the backpack                                                                                          |
| `B`                       | open the backpack                                                                                               |
| `G`                       | open the photographs the camera has taken; from the street each can be put up on the camera and looked at there |
| `X`                       | open the export menu                                                                                            |
| `K`                       | save settings to the config file                                                                                |
| The figure                | the walker's last position in walk mode                                                                         |
| `Esc`                     | close the photographs or the backpack, or clear the selection                                                   |
| `V`                       | enter walk mode                                                                                                 |
| `?`                       | show all the controls                                                                                           |

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
| Jet backpack in hand    | flight. While flying, `W` and `S` move along the view direction — looking down and pressing `W` descends — and `C` descends vertically. `F` (or middle click) opens the throttle for a burst                                                                                                                                                                                                                                                                                                                                                                                                               |
| Water skimmers in hand  | the surface of the water is walkable, passing under the bridges rather than over them; stowing them over deep water drowns the walker                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| Click                   | use the right hand; held down, the nail gun and the extinguisher keep firing. A module within reach is selected, and a bug that is caught is displayed and retained                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `F`, `C` / middle click | use the left hand. A secondary tool selects and catches nothing: the grapple hooks the building being looked at, the jet gives a burst of thrust. While flying, `C` descends instead                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `B`                     | the backpack, from the street as well as from the map                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `X` / `K`               | open the export menu; save the current view to the config file                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `H`                     | stow or draw both hands. A stowed tool remains functional and throws from the walker's eye                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Hold right button       | look through the scope                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `Enter`                 | show the details of whatever the reticle is on, as a second use of the tool would. This releases the pointer; a click on the map resumes                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
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
walker is never a target, so aiming at the street selects nothing. Expanding and
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
packages each, followed by the advisories it matched — covering Go,
npm, PyPI, crates.io, Maven, NuGet and GitHub Actions. Floating packages are not
queried, since they resolve to a different version on the next installation.
Answers are cached for six hours. `--no-vulns` disables all of this.

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

| Ecosystem          | pinned by                                                            | floats on                                                   |
|--------------------|----------------------------------------------------------------------|-------------------------------------------------------------|
| Go modules         | the version in `go.mod`, which the build selects                     | — (a `require` always names a version)                      |
| npm                | `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, an exact `1.2.3` | any range, including `1.2`, which denotes 1.2.x             |
| crates.io          | `Cargo.lock`                                                         | the manifest alone, where `"1.2.3"` denotes `^1.2.3`        |
| PyPI               | `poetry.lock`, `uv.lock`, `pdm.lock`, `Pipfile.lock`, `==1.2.3`      | `>=`, `~=`, `^` and other ranges                            |
| Maven              | a plain version, `[1.2.3]`                                           | ranges, `LATEST`, `RELEASE`, `-SNAPSHOT`, unexpanded `${…}` |
| NuGet              | an exact version, `[1.2.3]`                                          | wildcards (`2.*`) and ranges                                |
| PowerShell Gallery | `RequiredVersion`                                                    | `ModuleVersion`, which is a minimum                         |
| GitHub Actions     | a full commit SHA                                                    | tags, branches, or no ref                                   |
| GitLab CI includes | a commit                                                             | tags, branches, templates, remote includes, or no ref       |
| Container images   | an `@sha256:` digest                                                 | tags                                                        |

The JSON and GraphML exports carry `requested` and `floating` per package.

### Dependencies of dependencies

`--resolve-depth` extends the graph beyond what the code imports directly to
what those packages themselves require: `1` adds one level, `2` adds two, and
`-1` continues as far as the available information reaches. That information
comes from the lock files the repository already carries; nothing is fetched,
and the analysis remains offline.

| Lock file                            | gives                                            |
|--------------------------------------|--------------------------------------------------|
| `package-lock.json` (v1-v3)          | every installed package and what it requires     |
| `pnpm-lock.yaml` (v5-v9)             | `packages:` and, since v9, `snapshots:`          |
| `yarn.lock` (classic)                | each entry's resolved version and `dependencies` |
| `Cargo.lock`                         | `dependencies` per crate                         |
| `uv.lock`, `poetry.lock`, `pdm.lock` | each distribution's own requirements             |
| an installed Python environment      | each distribution's `Requires-Dist`              |

Packages added in this way are marked **transitive**, meaning that no file in
the repository imports them. Edges between packages are of kind `depends`, as
distinct from the `import` edges that originate at a file, so that the count of
files importing a package remains exactly that. Two versions of one package
remain a single building, so an edge between packages is an edge between names.

A Python package that no lock file gives edges for (`Pipfile.lock` records
none) falls back to the installed environment (see [Languages](#languages)).
Ecosystems that keep the dependency graph outside the repository — Go modules,
NuGet and container images — require `--online`, described below; Maven and the
PowerShell Gallery are not resolved beyond the first level at present.

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
`pyproject.toml`; `NuGet.config`; a POM's `<repositories>`; the mirrors in
`~/.m2/settings.xml`; `.cargo/config.toml`; and `GOPROXY` — and the side panel
names the index each package resolves from. A container image requires no
configuration, since `ghcr.io/org/app` names its registry directly.

The two sources are not treated alike. An index named by **this machine's** own
configuration is trusted. One that appears only in the repository is recorded
and marked **⚠ index**, because a repository directing a package manager at an
index that nothing here configures is the form a dependency-confusion attack
takes. No request is ever made to such an index, unless it is vouched for with
[`--trust-index`](#vouching-for-an-internal-index).

`--online` permits depphunter to query the trusted indexes for dependencies the
repository does not record, which is how `--resolve-depth` reaches the
ecosystems whose graph is held outside the repository:

| Ecosystem | asked for                                                                 | answer                                           |
|-----------|---------------------------------------------------------------------------|--------------------------------------------------|
| Go        | `<proxy>/<module>/@v/<version>.mod`                                       | its direct (non-`// indirect`) `require` entries |
| npm       | `<registry>/<package>/<version>`, or `/latest` when unpinned              | its `dependencies`                               |
| PyPI      | `<host>/pypi/<name>/<version>/json`, or `/pypi/<name>/json` when unpinned | `requires_dist`, excluding extras                |
| crates.io | `<index>/<se>/<rd>/<name>`, sparse index                                  | its normal `deps`, excluding optional ones       |
| NuGet     | `<feed>/<id>/<version>/<id>.nuspec`                                       | `<dependencies>`, both flat and by group         |
| OCI       | the manifest, then its config blob                                        | the **base image** it was built on               |

A container image has no dependency list. What it has is the image it was built
on, which is the source of its unpatched vulnerabilities, and that is what is
followed: the manifest — one platform's, where the manifest is a multi-platform
index — and then the small config blob it references, read for
`org.opencontainers.image.base.name` in the manifest's annotations or the image's
labels. No layers are downloaded. A registry requiring a pull token is given the
opportunity to say so, and the token endpoint it names is followed only over
HTTPS, or back to the registry's own host.

Maven and the PowerShell Gallery are not queried. Maven cannot readily be: an
import names a package, a package does not identify the artifact that ships it,
and a POM is addressed by group *and* artifact.

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
| the index URL itself                                          | `https://user:password@host/simple`, as a private pip or Cargo mirror is set                                      |

Between them these cover Nexus, Artifactory, Azure Artifacts, ProGet, GitHub
Packages, Harbor, GHCR and a private crate registry.

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
Maven group (`com.acme.*`) and a registry path (`harbor.corp/*`). Prefixing a
pattern with an ecosystem — `npm:`, `go:`, `maven:`, `nuget:`, `oci:`, `pypi:`,
`crates:`, `actions:`, `gitlab-ci:` or `psgallery:` — restricts it to that
ecosystem.

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

| Language                | Server                                                     |
|-------------------------|------------------------------------------------------------|
| Go                      | `gopls`                                                    |
| JavaScript / TypeScript | `typescript-language-server`                               |
| Python                  | `pyright-langserver`, `basedpyright-langserver` or `pylsp` |
| Rust                    | `rust-analyzer`                                            |
| Java                    | `jdtls`                                                    |
| C#                      | `csharp-ls`                                                |

The servers run in the background once the map is displayed — gopls requires
approximately 7 s for this repository — within the budget set by
`--lsp-timeout`; results are cached until the map's files, symbols or imports
change, or a different set of language servers is installed. Servers that index
slowly, rust-analyzer and jdtls in particular, may answer before indexing has
finished, so a first run can report fewer references than a later one. The
legend's **Imports / References** switch then determines what the selection arcs
and the side panel show: for a function, what it uses and what uses it. The JSON
and GraphML exports include the reference edges.

### Languages

| Ecosystem               | Imports resolved through                                                                                                                                                                                    | Islands                                     |
|-------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------|
| Go                      | every `go.mod` (multi-module, local `replace`)                                                                                                                                                              | Go modules, Go standard library             |
| JavaScript / TypeScript | relative paths, `tsconfig`/`jsconfig` `paths`, workspaces, `package.json` + `package-lock.json` / `yarn.lock` / `pnpm-lock.yaml`                                                                            | npm, Node.js built-ins                      |
| Python                  | relative imports, `src/` layouts, requirements files, `setup.cfg`, literal `setup.py` lists, `pyproject.toml`, `Pipfile`, `poetry.lock`/`uv.lock`/`pdm.lock`/`Pipfile.lock`, installed environments (below) | PyPI, Python standard library               |
| Rust                    | the module tree (`crate::`, `self::`, `super::`, `mod x;`), workspace and path crates, `Cargo.toml` (renamed and workspace dependencies) + `Cargo.lock`                                                     | crates.io, Rust standard library            |
| Java                    | source files by package path (any source root), `pom.xml` (properties, dependency management), Gradle scripts and version catalogs                                                                          | Maven, Java standard library                |
| C#                      | namespaces to project folders (`RootNamespace` + folder), `PackageReference`, `Directory.Packages.props`                                                                                                    | NuGet, .NET base library                    |
| PowerShell              | `using module`, `Import-Module`, dot-sourced and `&`-invoked scripts (`$PSScriptRoot`), `#Requires -Modules`, module manifests (`RequiredModules`, `RootModule`, `NestedModules`)                           | PowerShell Gallery, built-in modules        |
| CI pipelines            | GitHub workflows and composite actions (`uses:`, reusable workflows, `container:`, `services:`), GitLab pipelines (every `include:` form, components, `image:`, `services:`)                                | GitHub Actions, GitLab CI, Container images |
| Markdown                | links to files and directories in the repository (inline, reference, autolink, and the `href` and `src` of raw HTML); headings become the file's symbols                                                    | *(none: a link is not a package)*           |

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

Java imports name packages rather than artifacts, so they are matched to Maven
group identifiers by prefix, by shared leading segments, by artifact name, and
by a short table of well-known exceptions such as Guava, JUnit 4 and Lombok.
Imports that cannot be matched are shown as unresolved.

Files in other languages appear on the map without dependency edges. Parsing
uses a pure-Go tree-sitter runtime for JavaScript/TypeScript, Python, Rust and
Java; Go uses the standard library's own parser, CI files a YAML parser, and C#,
PowerShell and Markdown small built-in scanners — so the binary continues to
cross-compile without a C toolchain.

## Security

The server binds to loopback by default and prints a URL containing a random
token, which the browser exchanges for a cookie. Requests lacking the token,
requests carrying a foreign `Host` header while bound to loopback — that is,
DNS rebinding — and
requests for files outside the analyzed project are all rejected. Nothing may
frame the map: `X-Frame-Options: DENY` and `frame-ancestors 'none'`.

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
