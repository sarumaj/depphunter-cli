---
id: REQ-EXT-032
uuid: d6b87fb2-5895-49bd-9a9f-4e8ba861b53a
title: Extension packaged per platform and published
scope: ext
type: functional
priority: must
status: implemented
verification:
  - inspection
---

## Statement

A pushed tag `v*` **shall** package one VSIX per target (`win32-x64`,
`win32-arm64`, `linux-x64`, `linux-arm64`, `linux-armhf`, `darwin-x64`,
`darwin-arm64`, `alpine-x64`, `alpine-arm64`), each bundling the depphunter
binary built for that platform from the same tag, and one universal VSIX without
a binary; attach them to the tag's GitHub release with their checksums in
`checksums.txt`; and, when the tag is a plain `vX.Y.Z`, package them at version
`X.Y.Z` and publish every one of them to the Visual Studio Marketplace and to
the Open VSX Registry.

## Rationale

Installing the extension from a registry installs the server it starts, at the
same version, so the two cannot diverge; the registries serve each machine the
build for its platform, and the universal build covers any platform not listed,
falling back to `depphunter` on `PATH`.

## Acceptance criteria

1. The release of a tag `vX.Y.Z` holds ten VSIX files, named
   `depphunter_X.Y.Z_vscode_<target>.vsix`, and their checksums.
2. Each platform VSIX contains exactly one binary, for its own platform, whose
   `--version` names the tag.
3. For a plain `vX.Y.Z` tag, every VSIX is published to both registries; a
   re-run skips what a registry already has.
4. A tag that is not a plain `vX.Y.Z` (a release candidate) is packaged at the
   manifest's version, attached to its release and published to neither
   registry.

## Notes

The command-line archives of the same release are
[REQ-DIST-009](../dist/REQ-DIST-009-tagged-release-publication.md).
