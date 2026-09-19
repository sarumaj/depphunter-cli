# depphunter

Browse any code base as an interactive isometric archipelago in your browser.

- **Mainland** = your repository. Directories are terraces, files are buildings
  (height = lines of code, colour = language).
- **Islands** = external ecosystems (Go modules, the standard library, …) with one
  building per dependency.
- **Click** anything to see what it depends on and what uses it; **double-click** to
  expand or collapse directories and files (files expand into their symbols).

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
```

| Flag | Default | |
|---|---|---|
| `--addr` | `127.0.0.1:0` | listen address; port 0 picks a free port |
| `--no-open` | | print the URL instead of opening the browser |
| `--exclude` | | glob of paths to skip (repeatable) |
| `--max-file-size` | `2097152` | larger files are listed but not read |
| `--config` | `<path>/.depphunter.yaml` | config file to use |
| `--theme` | `auto` | `auto`, `light`, `dark` |
| `--color-by` | `language` | `language`, `size` |
| `--height-scale` | `sqrt` | `linear`, `sqrt`, `log` |
| `--show-std` | `false` | show standard-library islands |
| `--expand-depth` | `0` | initially expanded depth; `0` = auto, `-1` = all |

Settings are resolved from, in increasing precedence: built-in defaults, the user config
(`$XDG_CONFIG_HOME/depphunter/config.yaml`, or the OS equivalent), the project config
`.depphunter.yaml`, `DEPPHUNTER_*` environment variables (`ADDR`, `OPEN`, `EXCLUDE`,
`THEME`, `COLOR_BY`, `HEIGHT_SCALE`, `SHOW_STD`), and flags.

```yaml
# .depphunter.yaml
exclude: [testdata, "*.pb.go"]
ui:
  color_by: language
  height_scale: sqrt
  show_std: false
  expand_depth: 0
```

## Keyboard & mouse

| | |
|---|---|
| Drag / right-drag / wheel | pan / orbit / zoom |
| Click / double-click | select / expand–collapse |
| `Enter`, `Backspace` | expand–collapse selection, select parent |
| `Q` `E` | rotate 90° |
| `F` | fit to screen |
| `+` `−` | expand / collapse one level everywhere |
| `Esc` | clear selection |

## Security

The server binds to loopback by default and prints a URL containing a random token,
which the browser exchanges for a cookie. Requests without it, requests with a foreign
`Host` header (DNS rebinding), and requests for files that are not part of the analysed
project are rejected.

## Status

Milestone 1 of [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md): Go is analysed (imports,
symbols, multi-module `go.mod`); files in other languages appear on the map without
edges. Pluggable tree-sitter languages, search, watch mode and exports follow.
