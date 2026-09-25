---
id: REQ-CLI-001
uuid: ecc29afb-ca8c-43fc-b469-70cd97f6fc83
title: Analyze a directory and serve its map
scope: cli
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
verification:
  - integration
  - e2e
---

## Statement

The command `depphunter [path]` **shall** analyze the directory `path`, or the
current working directory when no path is given, serve the interactive map of it
on the local server, and open that map in the default web browser.

## Rationale

A user unfamiliar with a repository reaches its map with one command and no
arguments; the path argument lets the command run from anywhere.

## Acceptance criteria

1. `depphunter` run in a directory analyzes that directory and logs the URL it
   serves.
2. `depphunter <dir>` analyzes `<dir>`; a symbolic link to a directory analyzes
   the directory it points to.
3. Unless opening is turned off (REQ-CFG-008), the default browser opens the
   served URL; a failure to open it is logged and the server keeps running.
4. A path that is not a directory is refused with an error.

## Notes

Opening the browser uses `cli/browser` (REQ-DIST-016). Exports (`--export`)
replace serving; they belong to scope `exp`.
