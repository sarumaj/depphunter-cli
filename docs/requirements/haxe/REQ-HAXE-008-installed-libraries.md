---
id: REQ-HAXE-008
uuid: 239edf6d-2f8c-4a02-99b7-99ee30c94702
title: Installed libraries
scope: haxe
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** read the libraries haxelib installed: every library
of a local repository (`.haxelib/` at the repository root or beside a build
file) and the declared libraries in the global repository (`HAXELIB_PATH`,
else the path in `~/.haxelib`, else `~/haxelib`), names matched
case-insensitively (`.` written `,` in directory names), at the version
`.dev` points to, else the declared version when installed, else the one
`.current` names; its class path is the `classPath` of its `haxelib.json`.
lix's libraries **shall** be probed in lix's cache (`HAXE_LIBCACHE`, else
`haxe_libraries/` under `HAXESHIM_ROOT` or `~/haxe`). `--resolve-depth`
**shall** follow a library's `-lib` lines in its lix pin, else its installed
`haxelib.json` dependencies. An installed version **shall** be shown for a
library declared without one; it still floats.

## Rationale

What is installed says exactly which packages a library provides.

## Acceptance criteria

1. With a global repository holding format 3.7.0 (current) and 3.5.0, a
   library named `HxWidgets` and a development library, `format.zip.Reader`,
   `wx.Frame` and `dev.Tool` resolve to them; `format.old.Gone`, only in
   3.5.0, is `format` by the table; format depends on `hxcpp` and a pinned
   `tink_core`.
2. A module in lix's cache resolves to the lix-pinned library, whose
   dependencies are its pin's `-lib` lines.
