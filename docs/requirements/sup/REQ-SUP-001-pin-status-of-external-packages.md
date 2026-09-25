---
id: REQ-SUP-001
uuid: 91b3c296-8f5e-4be8-a191-28ada5b1f6bf
title: Pin status of every external package
scope: sup
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The system **shall** determine for every external package whether anything fixes
it to one version. A lock file entry, an exact version specifier, a
single-version range, a full git commit and an OCI digest **shall** pin a
package; a range, a wildcard, a snapshot version and a moving tag **shall not**.
A package that does not pin **shall** carry `floating` on the graph.

## Rationale

A dependency that moves when it is next installed is a supply-chain exposure the
map is meant to show. A package required by several manifests is only as fixed
as its loosest requester.

## Acceptance criteria

1. A package whose version is `1.2.3`, `==1.2.3`, `[1.2.3]`, a 40- or
   64-character hexadecimal commit or `sha256:` followed by 64 hexadecimal
   characters is not floating.
2. A package whose specifier is `^2.0.0`, `~1.2`, `>=1.0`, `1.2.x`, `*`,
   `latest` or `[1.0,2.0)` is floating.
3. A package required pinned by one manifest and as a range by another is
   floating.
4. A package whose version nothing states is not floating, unless its plugin
   marks the reference as moving although it names no version.

## Notes

The ecosystem-specific rules are
[REQ-SUP-002](REQ-SUP-002-ecosystem-own-pinning-rule.md). The unit tests are
`TestPinned` and `TestPackageVersions`.
