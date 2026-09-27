---
id: REQ-DLANG-008
uuid: 134583b9-8640-40eb-8814-b67d89fb19a7
title: Installed packages
scope: dlang
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The D plugin **shall** read the packages the repository declares or selects
that dub fetched onto this machine - the repository's
`.dub/packages`, then `$DUB_HOME/packages`, else `$DPATH/dub/packages`,
else `~/.dub/packages` (`%LOCALAPPDATA%\dub\packages` on Windows), in
dub 1.31's `<name>/<version>/<name>/` and the older
`<name>-<version>/<name>/` layout, the selected version else the newest -
and map every module under their import directories (their sub-packages'
too) to the package, authoritatively. `--resolve-depth` **shall** follow
such a package's recipe dependencies (its inline and directory
sub-packages' too, not optional, path or own sub-package ones), each pinned
as the repository's selections pin it, reported as installed.

## Rationale

dub.selections.json is flat; what dub fetched is the only offline source
of the packages' own dependencies and modules.

## Acceptance criteria

1. With a DUB_HOME holding ddata 1.2.0 (new layout, modules under `lib/`, a
   sub-package directory) and 1.1.0 (old layout), `datastructs.tree`,
   `datastructs` and `ddinternal.util` resolve to ddata 1.2.0, while
   `datastructs.old` (only in 1.1.0) does not; ddata depends on dyaml and
   vibe-d (pinned by the selections) and taggedalgebraic (floating).
