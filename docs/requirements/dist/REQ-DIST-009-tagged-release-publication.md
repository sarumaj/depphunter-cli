---
id: REQ-DIST-009
uuid: e96de81e-382f-49ed-b07f-f2f26f1cd29f
title: Releases published from tags
scope: dist
type: functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

Pushing a tag `v*` **shall** run the tests, build the archives of every target
with `scripts/dist.sh`, smoke-test the Linux amd64 binary and publish the
archives and `checksums.txt` as a GitHub release of that tag.

## Rationale

A release is complete only when every listed target has its archive.

## Acceptance criteria

1. The release workflow runs on `push` of tags `v*`.
2. The release of a tag holds one archive per target and `checksums.txt`.
3. The released binary's `--version` names the tag.

## Notes

The same workflow publishes the VS Code extension (scope `ext`).
