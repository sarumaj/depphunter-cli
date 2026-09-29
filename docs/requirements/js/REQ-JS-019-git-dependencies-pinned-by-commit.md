---
id: REQ-JS-019
title: Git dependencies pinned by commit
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A package that a lock file installs from a git repository **shall** be pinned
to the full commit (40 hex digits, or 64) the lock records, which **shall** be
its version, and the repository **shall** be its origin, written without any
password or token the lock's URL holds. The plugin **shall** read the commit
and the repository from:

- `package-lock.json` (and `npm-shrinkwrap.json`) v2 and v3: a package's
  `resolved` (`git+ssh://…#<sha>`, `git+https://…#<sha>`); v1: its `version`
  (`git+…#<sha>`, `github:owner/repo#<sha>`);
- a classic `yarn.lock` entry's `resolved`: `git+…#<sha>` or a GitHub archive
  `https://codeload.github.com/<owner>/<repo>/tar.gz/<sha>`;
- a Berry `yarn.lock` entry's `resolution`:
  `<name>@https://github.com/<owner>/<repo>.git#commit=<sha>` (or an SSH or
  other git URL);
- a `pnpm-lock.yaml` package's `resolution`: `{commit, repo, type: git}`, or a
  GitHub archive's `tarball`; the package is found by its key (v5, v6) or by
  its name and reference (v9);
- a `bun.lock` package's `git+<url>#<sha>` resolution, and the repository of a
  `github:owner/repo#<abbreviated commit>` one, whose version stays as Bun
  writes it (REQ-JS-016).

This **shall** hold for a package the project imports and for one another
package depends on (REQ-SUP-009). A registry tarball whose URL ends in
`#<its SHA-1>` **shall not** be read as a git dependency. A git dependency whose
lock entry names only a branch or a tag (Berry's `#head=main`, `#tag=v1`)
**shall not** be pinned: an import keeps `package.json`'s range, the repository
is still its origin, and the lock file **shall** be noted with the code
`git-unpinned` (REQ-TRC-017). A range in `package.json` that names a git
repository **shall** make that repository the origin of a package no lock
file pins.

## Rationale

A git dependency's code is the repository's at a commit, not the registry's
release of the same name: asking the registry, or the vulnerability database by
the version its `package.json` states, is asking about something else, and
names what may be the organization's own repository to both. The commit is
what the vulnerability database can answer for (REQ-FND-026).

## Acceptance criteria

1. In each gitlock fixture (npm v1 and v3, classic and Berry `yarn.lock`,
   pnpm v6 and v9, `bun.lock`), `pub` (`github:acme-oss/pub`) resolves to its
   commit, pinned, requested `github:acme-oss/pub`, from
   `github.com/acme-oss/pub`; `corp` resolves to its commit from
   `git.corp.example/team/corp`; the registry package `dep` to `1.0.0` with no
   origin; and `pub`'s dependency `tr` to its commit from
   `gitlab.com/acme-oss/tr`.
2. A classic `yarn.lock` URL with a token in it gives an origin without the
   token; a registry tarball URL ending in `#<sha1>` is no git dependency.
3. Berry's `branchy` at `#head=main` keeps its declared range unpinned, has
   its repository as origin, and `yarn.lock` is noted `git-unpinned`; no other
   fixture is noted.
4. With `--online`, `pub`'s and `tr`'s commits are asked about and `corp` is
   asked about in no way (REQ-FND-026).

## Notes

A dependency on a git repository at a commit is private for having an origin
(REQ-PY-015): no package index is asked about it, and the vulnerability
database only by its commit, only where its repository is on a public forge.
