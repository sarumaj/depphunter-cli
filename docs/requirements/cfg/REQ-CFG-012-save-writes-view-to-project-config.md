---
id: REQ-CFG-012
uuid: a2cbbbe0-c145-4962-bd3a-b1803f2150bf
title: Save writes the view to the project config
scope: cfg
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M4
  - docs/REQUIREMENTS.md M30
verification:
  - integration
  - e2e
---

## Statement

The UI **shall** offer a Save action that sends the current view settings
(theme, style, color-by, height scale, expand depth, standard-library islands,
hidden languages, hidden islands, path filter and tool) to `POST /api/settings`,
and the server **shall** write them to the project config file, or to the
`--config` file when one was given.

## Rationale

The project file is where a view lives: it is per-repository, it is the file the
command line reads, and it is the one file both the map and the editor extension
can see (M30).

## Acceptance criteria

1. Pressing Save (or `K`) in the map writes the view settings into
   `<path>/.depphunter.yaml` and reports the file in the status line.
2. With `--config <file>`, Save writes to `<file>`.
3. A failed save is reported in the status line.

## Notes

The static HTML export has no server and hides the Save button.
