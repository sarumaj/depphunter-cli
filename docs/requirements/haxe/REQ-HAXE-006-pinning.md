---
id: REQ-HAXE-006
uuid: 99d1136b-53ef-4bb9-be5b-daf61ccab768
title: Library versions and pinning
scope: haxe
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A haxelib library **shall** be named by its haxelib name and versioned by
what pins it for the importing file: lix's `haxe_libraries/<name>.hxml` in
the nearest directory at or above the file that has one - whose
`# @install: lix download` URL pins a haxelib version
(`haxelib:/name#1.2.3`) or a git commit (`gh://github.com/o/r#<commit>`),
shows a tag, neither pinned nor floating, and floats on a branch; a pin with
no install line is a library in development, and one whose class path is in
the repository (`${SCOPE_DIR}/...`) **shall** resolve to that directory -
else the nearest build file declaring a version: an exact version
(`-lib x:1.2.3`, `"x": "1.2.3"`, `version="1.2.3"`) pins, a
`git:<url>#<commit>` pins, a tag is shown and a branch, a bare git URL or no
version floats. A declared version lix resolves differently **shall** be
the requested one. A git server other than the public forges **shall** be
the library's origin. A library that is the repository itself (the name
of its `haxelib.json`) **shall** resolve to its class path.

## Rationale

lix is haxelib's lock file; haxelib itself installs whatever is current
unless a version is named.

## Acceptance criteria

1. In `game/`, `tink_core` is pinned at 2.1.1, `tink_unittest` at its
   commit, `tink_testrunner` shows `v0.9.0`, `acmeapi` floats on `main` with
   `https://git.acme.dev/acmeapi.git` as origin and `mylib` resolves to
   `game/libs/mylib/src`.
2. `openfl` is pinned at 9.2.0 at the root and at 9.3.0 in `app/`; `fancy`
   (a commit on `git.acme.dev`) is pinned with its origin, `tagged` shows
   `v1.2.0` and `branchy` floats.
