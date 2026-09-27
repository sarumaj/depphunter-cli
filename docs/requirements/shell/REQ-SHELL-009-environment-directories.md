---
id: REQ-SHELL-009
title: Paths below environment directories
scope: shell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A sourced or run path that starts with a variable the file does not assign
(other than `HOME`) and continues with at least two path elements
(`"$PLUGIN_AVAILABLE_PATH/config/functions"`) **shall** resolve to the one
project file whose path ends in those elements, and be dropped when no file
or several files do.

## Rationale

Plugin systems and installers source files through a directory their
environment names at run time: dokku's plugins source
`$PLUGIN_CORE_AVAILABLE_PATH/common/functions`, which is `plugins/common/functions`
in the repository. Without this, dokku's 600 scripts had 136 file edges; with
it, 705, each to the plugin directory it names. One element alone
(`$X/functions`) is too common a name to be told apart.

## Acceptance criteria

1. With `plugins/common/functions`, `plugins/git/functions`, `a/lib/x.sh` and
   `b/lib/x.sh` in the project, `common/functions` and `plugins/git/functions`
   resolve, `lib/x.sh` (two files) and `available/common/functions` (none)
   do not, and `"$PLUGIN_PATH/functions"` records nothing.
