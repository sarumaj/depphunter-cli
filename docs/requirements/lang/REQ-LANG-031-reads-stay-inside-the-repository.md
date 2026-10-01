---
id: REQ-LANG-031
title: Repository files read only inside the repository
scope: lang
type: constraint
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The system **shall not** read, list or measure any file through a path of the
analyzed repository that leads outside the repository, whether by `..` or by
a symbolic link. The scan **shall not** list a symbolic link at all; every
other file of the repository that is read - a lock file git ignores, an
installed dependency tree, a configuration or report file the repository
names, the project's `.depphunter.yaml` - **shall** be opened through a
confined root, which follows a symbolic link only while it stays inside the
repository. A file so read **shall** be no larger than the bound its reader
applies (MaxParseSize, 1 MiB, unless stated otherwise), measured on the file
once it is open.

## Rationale

A repository is untrusted input, and a symbolic link is an ordinary thing to
commit. A lock file committed as a link to a file elsewhere on this machine
would otherwise be read and parsed: its content would appear in the graph as
pinned versions, and with `--online` be sent to package indexes and to OSV. A
size measured on the link (Lstat) rather than on what is read would let a
link pass off a large file as a small one.

## Acceptance criteria

1. A repository committing `pubspec.lock`, `mix.lock`, `Package.resolved`,
   `shard.lock`, `Gemfile.lock` or `Podfile.lock` as a symbolic link to a
   file outside it holding a pinned version does not get that version in its
   graph; the same content committed as a regular file, or behind a relative
   link to a file inside the repository, does.
2. A read through a confined root refuses a symbolic link to a file or a
   directory outside the root, `..`, an absolute path outside the root, a
   file over its bound, and a symbolic link to a file over its bound; it
   follows a relative or absolute symbolic link to a file or a directory
   inside the root.
3. A scanner report the configuration names by a path in the repository, a
   document a Markdown link points at, and the project's `.depphunter.yaml`,
   committed as a link out of the repository, are not read.
4. What this machine keeps outside the repository - package managers'
   configuration (`~/.cargo`, `~/.m2`, NuGet.Config files above the
   checkout), their caches and depots, the trees they install under the
   user's home, `CUE_CACHE_DIR`, and the Python interpreter's site-packages -
   is read as before.
5. A package manager's directory named by a variable of the environment
   (`ZIG_GLOBAL_CACHE_DIR`, `XDG_CACHE_HOME` for Zig, `XDG_DATA_HOME` and
   `ALIRE_SETTINGS_DIR` for Alire, `DUB_HOME`, `DPATH`, `NIMBLE_DIR`,
   `ELM_HOME`, `CUE_CACHE_DIR`, `HAXELIB_PATH`, `HAXE_LIBCACHE`,
   `HAXESHIM_ROOT`), when relative, is taken against the repository root,
   where the tool is taken to run, as `PYTHONPATH`'s entries are; one that
   lies in the repository is read through its root. A tool's configuration
   is not: a relative XDG variable is ignored there (REQ-SUP-064).

## Notes

The scan's own skipping of links is REQ-LANG-017's and REQ-LANG-018's
listing; this requirement extends it to every read the resolvers, the index
discovery, the findings and the configuration make of the repository.
`lang.Root` implements the confinement over `os.Root`, after resolving the
path's links, so that a link inside the repository, relative or absolute
(pnpm's `node_modules/.pnpm`, a shards path dependency), still works.
