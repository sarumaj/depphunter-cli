---
id: REQ-DLANG-005
title: dub recipes
scope: dlang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The D plugin **shall** read `dub.json` (encoding/json) and `dub.sdl` (a small
SDLang reader: tags, values, attributes, children, `//`, `#`, `--` and
`/* */` comments, `\` continuations, escaped and backtick strings): the
package `name`, `dependencies` (a version string, or an object with
`version`, `path`, `repository` and `optional`), the dependencies of every
`configuration`, `subPackages` written inline or as directories,
`sourcePaths`, `importPaths` and `stringImportPaths` (platform-suffixed
ones too), and the recipe a single-file package embeds in a leading
`/+ dub.sdl: ... +/` or `/+ dub.json: ... +/` comment. Every dependency of a
recipe, its configurations and its inline sub-packages **shall** be an
import of the recipe file (the module's, for a single-file package), and
every sub-package directory an edge to it. `buildTypes` carry no
dependencies and are not read.

## Rationale

The recipe is where dub packages declare what they use; sub-packages and
configurations are how larger packages (vibe.d) split theirs.

## Acceptance criteria

1. The fixture's `dub.json` imports its ten dependencies, `requests` of the
   `web` configuration, `darg` and `shop` of the inline `cli` sub-package
   and the `tools/` directory, each on the line that names it.
2. `scripts/hello.d`'s embedded recipe makes `scriptlike` a floating
   package of that module.
