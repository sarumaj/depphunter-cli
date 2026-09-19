# depphunter

Browse any code base as an interactive isometric archipelago in your browser.

- **Mainland** = your repository. Directories are terraces, files are buildings
  (height = lines of code, colour = language).
- **Islands** = external ecosystems (Go modules, the standard library, …) with one
  building per dependency.
- **Click** anything to see what it depends on and what uses it; **double-click**
  to expand or collapse directories and files (files expand into their symbols).

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
```

| Flag              | Default                   |                                                  |
|-------------------|---------------------------|--------------------------------------------------|
| `--addr`          | `127.0.0.1:0`             | listen address; port 0 picks a free port         |
| `--no-open`       |                           | print the URL instead of opening the browser     |
| `--exclude`       |                           | glob of paths to skip (repeatable)               |
| `--max-file-size` | `2097152`                 | larger files are listed but not read             |
| `--config`        | `<path>/.depphunter.yaml` | config file to use                               |
| `--theme`         | `auto`                    | `auto`, `light`, `dark`                          |
| `--color-by`      | `language`                | `language`, `size`                               |
| `--height-scale`  | `sqrt`                    | `linear`, `sqrt`, `log`                          |
| `--show-std`      | `false`                   | show standard-library islands                    |
| `--expand-depth`  | `0`                       | initially expanded depth; `0` = auto, `-1` = all |
| `--watch`         | `false`                   | re-analyse on file changes, update the browser live |
| `--no-cache`      |                           | neither read nor write the analysis cache        |
| `--editor`        | auto-detected             | editor command template, e.g. `"code -g {file}:{line}"` |
| `--export`        |                           | write the graph as `json`, `graphml` or `dot` and exit |
| `-o`              | stdout                    | output file for `--export`                       |

Settings are resolved from, in increasing precedence: built-in defaults, the user
config (`$XDG_CONFIG_HOME/depphunter/config.yaml`, or the OS equivalent), the project
config `.depphunter.yaml`, `DEPPHUNTER_*` environment variables (`ADDR`, `OPEN`,
`EXCLUDE`, `THEME`, `COLOR_BY`, `HEIGHT_SCALE`, `SHOW_STD`, `WATCH`, `CACHE`, `EDITOR`),
and flags. The project config cannot set `editor`: it arrives with the repository, and
the editor is a command depphunter runs.

```yaml
# .depphunter.yaml
exclude: [testdata, "*.pb.go"]
ui:
  color_by: language
  height_scale: sqrt
  show_std: false
  expand_depth: 0
```

## Watch mode and cache

Parsing results are cached per file content under the user cache directory
(`~/.cache/depphunter` on Linux), so a second run only parses files that changed — the
CPython standard library goes from 1.2 s to 20 ms. With `--watch`, depphunter watches the
directories it analysed, re-analyses after changes settle (300 ms), and pushes the new
map to the browser, which keeps your expansion, selection and filters and briefly
highlights the files that changed.

## Exports

`--export` (or the **Export** menu in the browser) writes:

| Format | Contents |
|---|---|
| `json` | the full graph document the UI uses (nodes, symbols, edges) |
| `graphml` | the full graph with all attributes, for Gephi, yEd or NetworkX |
| `dot` | the dependency graph for Graphviz: files, package directories and external packages, clustered per directory; standard-library packages and files without imports are left out |

Dependency graphs are shallow, so Graphviz draws them long and thin; for big ones,
`unflatten -l 3 -c 5 deps.dot | dot -Tsvg -o deps.svg` spreads them out.

## Opening files in your editor

The side panel's **Open in editor** button (or `O`) opens the selected file at the
selected symbol's line. depphunter uses `--editor` / `DEPPHUNTER_EDITOR` / the user config,
or detects a GUI editor from `$VISUAL`, `$EDITOR` or `PATH` (VS Code, Cursor, Zed, Sublime
Text, JetBrains IDEs, …). Without one, the button hands the file to VS Code's `vscode://`
URL handler.

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

The server binds to loopback by default and prints a URL containing a random token,
which the browser exchanges for a cookie. Requests without it, requests with a foreign
`Host` header (DNS rebinding), and requests for files that are not part of the analysed
project are rejected.

## Languages

| Ecosystem | Imports resolved through | Islands |
|---|---|---|
| Go | every `go.mod` (multi-module, local `replace`) | Go modules, Go standard library |
| JavaScript / TypeScript | relative paths, `tsconfig`/`jsconfig` `paths`, workspaces, `package.json` + `package-lock.json` | npm, Node.js built-ins |
| Python | relative imports, `src/` layouts, requirements files, `pyproject.toml`, `Pipfile`, `poetry.lock`/`uv.lock`/`pdm.lock`/`Pipfile.lock` | PyPI, Python standard library |

Files in other languages appear on the map without dependency edges. Parsing uses a
pure-Go tree-sitter runtime, so the binary still cross-compiles without a C toolchain.

## Status

Milestones 1–3 of [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md) are done. Rust, Java and
C#, a static HTML export and saving UI settings follow.
