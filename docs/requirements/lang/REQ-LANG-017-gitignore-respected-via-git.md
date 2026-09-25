---
id: REQ-LANG-017
uuid: 8ff0abf0-a7ef-4750-9ade-85b3b261b518
title: Git ignore rules respected
scope: lang
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

When the analyzed directory is inside a git work tree and `git` is available,
the system **shall** list the files through `git ls-files` (tracked files plus
untracked files not ignored), so that every `.gitignore`, `.git/info/exclude`
and the global excludes file apply.

## Rationale

git already implements its ignore rules exactly; reusing it avoids a second,
diverging implementation.

## Acceptance criteria

1. A file matched by `.gitignore` in a git repository does not appear in the
   graph.
2. An untracked file that no ignore rule matches appears in the graph.
3. A file listed more than once by git (merge conflict stages) appears once.

## Notes

Symbolic links and entries that are not regular files are not listed (a security
measure; see the report).
