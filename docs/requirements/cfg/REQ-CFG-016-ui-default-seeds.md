---
id: REQ-CFG-016
uuid: 305efcbd-c2e0-45b9-bb20-d648a8c75f27
title: View settings seeded by --ui-default
scope: cfg
type: functional
priority: must
status: implemented
verification:
  - unit
  - extension
---

## Statement

The flag `--ui-default key=value`, repeatable, **shall** set the default of the
view setting `key` (one of `theme`, `color_by`, `height_scale`, `style`,
`show_std`, `expand_depth`, `tool`, `path_filter`); every config file,
environment variable and flag **shall** override it, and `--theme` and the other
view flags **shall** still override the project file.

## Rationale

An editor's appearance settings used to be passed as flags, which beat the
project file, so Save silently stopped working for anybody who had set a theme
in the editor. A seed decides how a repository that has never been saved opens
and stops deciding once somebody presses Save, while a flag typed for one run
still wins.

## Acceptance criteria

1. `--ui-default theme=dark` in a directory without a project file opens in the
   dark theme.
2. The same seed with a project file `ui: {theme: light}` opens in the light
   theme.
3. `--theme auto` with both the seed and the project file yields theme `auto`.

## Notes

The VS Code extension's use of the flag belongs to scope `ext`.
