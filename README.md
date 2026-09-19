# depphunter

Browse any code base as an interactive isometric archipelago in your browser.

- **Mainland** = your repository. Directories are terraces, files are buildings
  (height = lines of code, colour = language).
- **Islands** = external ecosystems (Go modules, the standard library, …) with
  one building per dependency.
- **Click** anything to see what it depends on and what uses it;
  **double-click** to expand or collapse directories and files (files expand
  into their symbols).

Everything runs locally: one binary, no network access, no Node.js.

## Install

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
| `--editor`        | auto-detected             | editor command template, e.g. `"code -g {file}:{line}"` |
| `--export`        |                           | write `json`, `graphml`, `dot` or `html` and exit       |
| `-o`              | stdout                    | output file for `--export`                              |

Settings are resolved from, in increasing precedence: built-in defaults, the
user config (`$XDG_CONFIG_HOME/depphunter/config.yaml`, or the OS equivalent),
the project config `.depphunter.yaml`, `DEPPHUNTER_*` environment variables
(`ADDR`, `OPEN`, `EXCLUDE`, `THEME`, `COLOR_BY`, `HEIGHT_SCALE`, `SHOW_STD`,
`WATCH`, `CACHE`, `EDITOR`, `HISTORY`), and flags. The project config cannot set
`editor`: it arrives with the repository, and the editor is a command depphunter
runs.

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

The browser's **Save view** button writes the current colour, height, theme,
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
authors. Renames are not followed, so a renamed file's history starts at the
rename. With `--watch`, a new commit updates the overlay.

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
| Legend click              | hide / show a language                   |
| `Esc`                     | clear selection                          |

## Security

The server binds to loopback by default and prints a URL containing a random
token, which the browser exchanges for a cookie. Requests without it, requests
with a foreign `Host` header (DNS rebinding), and requests for files that are
not part of the analysed project are rejected.

## Languages

| Ecosystem               | Imports resolved through                                                                                                                                                          | Islands                              |
|-------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------|
| Go                      | every `go.mod` (multi-module, local `replace`)                                                                                                                                    | Go modules, Go standard library      |
| JavaScript / TypeScript | relative paths, `tsconfig`/`jsconfig` `paths`, workspaces, `package.json` + `package-lock.json`                                                                                   | npm, Node.js built-ins               |
| Python                  | relative imports, `src/` layouts, requirements files, `pyproject.toml`, `Pipfile`, `poetry.lock`/`uv.lock`/`pdm.lock`/`Pipfile.lock`                                              | PyPI, Python standard library        |
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

## Status

Milestones 1–5 of [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md) are done. Next:
symbol-level references.
