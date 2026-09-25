---
id: REQ-EXT-018
uuid: e7ebe8ce-67f7-437f-b3b0-9148ea3b8735
title: Appearance settings passed as seeds
scope: ext
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M30
  - README.md Settings
  - README.md Configuration
verification:
  - extension
  - e2e
---

## Statement

The extension **shall** pass the appearance settings `depphunter.style`,
`theme`, `colorBy`, `heightScale`, `expandDepth` and `showStd` as `--ui-default
key=value` seeds and **shall not** pass them as `--style`, `--theme`,
`--color-by`, `--height-scale`, `--expand-depth` or `--show-std`.

## Rationale

As flags, the editor's settings beat the project file where the map's Save
writes, so Save silently stopped working for anyone with a theme set in the
editor. A seed only decides how a repository that has never been saved opens.

## Acceptance criteria

1. Settings `style=galaxy`, `theme=dark`, `colorBy=churn`, `heightScale=log`,
   `expandDepth=2`, `showStd=true` produce six `--ui-default` arguments with
   keys `style`, `theme`, `color_by`, `height_scale`, `expand_depth`, `show_std`
   and none of the six flags.
2. An editor with a theme set opens an unsaved repository in that theme.
3. Save in the map changes the view for good: a restarted server shows the saved
   view although the editor setting differs.

## Notes

The `--ui-default` flag and its precedence belong to scope `cfg`.
