---
id: REQ-JS-010
uuid: 89a93622-b66d-4bae-a48a-aecbded027f1
title: npm partial versions float
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Without a lock file entry, the plugin **shall** treat a declared npm version as
pinned only when it is a complete `major.minor.patch` version (optionally with
pre-release or build suffix), a git commit or a digest; a shortened version such
as `1.2` **shall** be treated as a range.

## Rationale

npm reads `1.2` as `1.2.x`, so only a complete version names one release.

## Acceptance criteria

1. `1.2.3` pins; `1.2`, `^1.2.3`, `~1.2.3` and `*` float.
2. `chalk` declared `^5.3.0` without a lock entry is floating.

## Notes

The general floating rule is scope `sup`.
