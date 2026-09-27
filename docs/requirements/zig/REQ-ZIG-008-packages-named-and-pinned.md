---
id: REQ-ZIG-008
uuid: 31086f06-b94a-4477-a7e0-8eaff8e8d19d
title: Packages named and pinned
scope: zig
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A dependency fetched from a URL **shall** be a package of the `zig` island
named by `lang.RepoName`: a GitHub, GitLab, Codeberg or sourcehut archive
(`/archive/<ref>`, `/-/archive/<ref>/...`, `/tarball/<ref>`,
`/releases/download/<tag>/`) and a `git+https` URL by the repository, with
the ref (or the `#commit` of a git URL, whose `?ref=` becomes the requested
version) as the version; another download by its URL without an archive
extension, a trailing `-<version>` becoming the version; an archive named by
a package hash by its host and the dependency's name. A `.hash` **shall**
pin the package, since Zig verifies the fetched content against it (as CMake
does a `URL_HASH`); its version is the URL's ref, else the version inside
the hash (`name-1.2.3-...`), else the hash. Without a hash a commit pins, a
tag or other ref is shown and neither pins nor floats, and a branch archive
(`refs/heads/`) or no ref floats.

## Rationale

A URL names a package where Zig has no registry, and the hash is the lock: a
changed download fails the build.

## Acceptance criteria

1. `known_folders` (a GitHub archive of a commit with a hash) is
   `github.com/ziglibs/known-folders` pinned at the commit;
   `git+https://...libvaxis.git?ref=v0.5.1#dc0a...` is pinned at the
   commit with v0.5.1 requested.
2. A Codeberg tag archive without a hash is neither pinned nor floating;
   `refs/heads/main` floats; a mirror archive named by its hash is
   `deps.example.org/tracy`.
