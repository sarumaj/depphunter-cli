---
id: REQ-NIM-006
title: Pins and lock files
scope: nim
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`nimble.lock` (its `packages` and each task's) and `atlas.lock` (its
`items`) beside a `.nimble` file **shall** pin a package, with its version
shown (the locked revision when the version is special, `#head`) and the
requirement as requested when it differs; a lock entry of a URL
requirement is matched by its URL and named as the requirement. Without a
lock, `== 1.2.3`, a bare `1.2.3` (exact in nimble) and a `#<commit>` (six
to 64 hexadecimal digits with a letter) **shall** pin; a `#tag` is shown,
neither pinned nor floating; `#head`, a branch, a range (`>=`, `^=`, `~=`,
`&`) and no version float. Lock entries **shall** be imports of their lock
file, and a locked package's `dependencies` **shall** be its dependencies
for `--resolve-depth` without asking a registry. A package's repository on
a server other than GitHub, GitLab, Bitbucket, Codeberg or sourcehut is its
origin.

## Rationale

nimble and Atlas lock git revisions; ranges and branches move.

## Acceptance criteria

1. The fixture's chronos is pinned at 4.0.3 by `nimble.lock` and depends on
   results, stew and bearssl; unittest2, locked only by the test task, is
   pinned with `>= 0.2` requested; the widgets URL requirement is pinned by
   its lock entry's revision with `#head` requested; Atlas's malebolgia and
   sat are pinned by `atlas.lock`.
