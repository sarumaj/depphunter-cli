---
id: REQ-DLANG-001
uuid: 051654a6-52af-4b08-af2c-613a959eed05
title: Files claimed
scope: dlang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The D plugin **shall** claim D modules (`.d` files scan labels D, and `.di`
interface files), dub's recipes `dub.json` and `dub.sdl`, and
`dub.selections.json`, telling them apart from other JSON files by name. A
`.d` file that is a make dependency file or a DTrace script (REQ-LANG-015)
**shall** not be claimed, and nothing in dub's `.dub/` directory (build
output, packages fetched with `--cache=local`) **shall** be analyzed.

## Rationale

`.d` is shared with make dependency files (gcc -MD, dmd -makedeps) and
DTrace scripts, which C projects keep beside their sources; reading them
as D would invent modules and imports.

## Acceptance criteria

1. The fixture's D modules, `dub.json`, `dub.selections.json` and both
   `dub.sdl` files are claimed; `deps/app.d` (a dependency file),
   `probes/trace.d` and `probes/provider.d` (DTrace) and
   `.dub/packages/leftpad/1.0.0/leftpad/source/leftpad.d` are not.
2. `import/shop.di` is claimed without a label.
