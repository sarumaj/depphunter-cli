---
id: REQ-EXT-033
uuid: f69b5349-b392-43b4-9249-055d4097df3d
title: The binary on the terminals' PATH
scope: ext
type: functional
priority: should
status: implemented
verification:
  - extension
---

## Statement

The extension **should** put the folder of the depphunter binary it runs first
on `PATH` in the editor's integrated terminals, through the editor's environment
variable collection, and **shall** change nothing else: no shell profile and no
environment outside the editor. It **shall** add nothing when the binary is a
bare name the shell finds on `PATH` anyway, and nothing when the setting
`depphunter.addToPath` is off; and it **shall** replace what it added when
`depphunter.path` or `depphunter.addToPath` changes, without offering to restart
the server.

## Rationale

A released build keeps its binary inside the extension's directory, where no
shell looks for it, so the command in the terminal beside the map is either
missing or a different version from the one the map runs.

## Acceptance criteria

1. With the bundled binary, the terminals' `PATH` starts with its `bin` folder.
2. With `depphunter.path` set to an absolute path, it starts with that file's
   folder instead, and the bundled one is gone.
3. With a build that ships no binary, a bare name in `depphunter.path`, or
   `depphunter.addToPath` off, nothing is added.

## Notes

The editor keeps the change across reloads, applies it to terminals opened from
then on, and offers to relaunch those already open.
