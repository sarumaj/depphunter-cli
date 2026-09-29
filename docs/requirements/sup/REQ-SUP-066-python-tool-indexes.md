---
id: REQ-SUP-066
title: uv, Poetry, Pipenv and PDM indexes
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The system **shall** read the PyPI indexes of this machine's uv
configuration: `UV_DEFAULT_INDEX` (or the legacy `UV_INDEX_URL`) replacing
PyPI, the space-separated `UV_INDEX` (and `UV_EXTRA_INDEX_URL`) asked beside
it, each entry optionally `name=url`, the comma-separated `UV_FIND_LINKS`, then
the `[[index]]` entries (and the legacy `index-url` and `extra-index-url`) and
the `find-links` of the user's and the system's `uv.toml`
([REQ-SUP-064](REQ-SUP-064-tool-configuration-locations.md)): an index with
`default = true` replacing PyPI, one with `explicit = true` serving only the
packages pinned to it, any other asked beside PyPI. An index with
`format = "flat"` and a `find-links` location are flat indexes, pages or
directories listing distribution files
([REQ-SUP-067](REQ-SUP-067-pypi-simple-api-fallback.md)), a relative path
taken from the file naming it (from the analyzed directory for the
variable); a `find-links` location is asked beside PyPI. It **shall** read
PDM's global `config.toml` with `PDM_PYPI_URL` over it: `[pypi] url` replacing
PyPI and each `[pypi.<name>]` asked beside it. Poetry's `config.toml`
repositories and `POETRY_REPOSITORIES_<NAME>_URL` **shall not** be sources.

The system **shall** read the repository's Python tool configuration, each
index untrusted:

- uv: the `[[tool.uv.index]]` entries and `find-links` of `pyproject.toml` as
  above, replaced by those of a `uv.toml` in the same directory (not under
  `UV_NO_CONFIG` or `UV_CONFIG_FILE`), asked before the indexes of this
  machine's `uv.toml` files (uv reads a project's configuration first) and
  after those of uv's variables, a repository default index replacing the
  user's; an index both explicit and default dropping PyPI; a flat index only
  when it is an `http` or `https` URL; and the packages `[tool.uv.sources]`
  pins with `index = "<name>"`, resolved as uv resolves them: against the
  indexes uv's variables name (such an index stays trusted), then the
  `pyproject.toml`'s own `[[tool.uv.index]]` entries, also beside a
  `uv.toml`, and never a `uv.toml`'s.
- Poetry: the `[[tool.poetry.source]]` entries - the sources of priority
  `default` and then `primary` (a source stating none) replacing PyPI together
  as an ordered list, a source named PyPI without a URL being PyPI at its
  place in it; the legacy `secondary` and then `supplemental` ones asked after
  them (after PyPI when no source is primary); `explicit` serving only the
  dependencies (of any group) that name it with `source = "<name>"`. PyPI
  **shall** be asked only where a source names it when the file declares a
  primary or default source or a source named PyPI of any priority, which
  drops Poetry's implicit PyPI.
- Pipenv: the `[[source]]` entries of a `Pipfile` and the `_meta.sources` of
  a `Pipfile.lock` - the first replacing PyPI, the others asked beside it -
  and the packages of any category pinned with `index = "<name>"`.
- PDM: the `[[tool.pdm.source]]` entries - one named `pypi` replacing PyPI,
  the others asked beside it, a `find_links` one no index - where a package
  some sources include (`include_packages`) is asked of those sources alone,
  in order, and a source never of a package it excludes
  (`exclude_packages`), PyPI included when the `pypi` source excludes it;
  under `[tool.pdm.resolution] respect-source-order` the sources after PyPI
  (all of them without a `pypi` source) asked after it; a `pdm.toml`'s
  `[pypi]` and `[pypi.<name>]` (with their patterns) as above.

A package pinned to an index **shall** be asked of that index alone. Package
names **shall** be compared as PEP 503 normalizes them, and an
`include_packages` or `exclude_packages` pattern as a glob. Where the tool
configured merges the versions of several indexes - uv's `index-strategy`
`unsafe-best-match` (`UV_INDEX_STRATEGY`, then the repository's setting, then
the user's `uv.toml`), uv's `find-links`, PDM without `respect-source-order` -
the indexes **shall** still be asked in order, the first that has the package
answering, and the report **shall** say so once
([REQ-TRC-017](../trc/REQ-TRC-017-resolver-notes.md)). A repository index
URL whose host or path holds a variable reference **shall not** be recorded,
and the
user name and password written into a repository index URL **shall** be
removed from it ([REQ-AUTH-023](../auth/REQ-AUTH-023-repository-feed-credentials-from-the-environment.md)).

## Rationale

uv, Poetry, Pipenv and PDM do not read pip's configuration: a project managed
with one of them names its company index in the tool's own files and
variables, and without them every package of that index was attributed to
PyPI or to nothing.

## Acceptance criteria

1. `UV_DEFAULT_INDEX` wins over the user `uv.toml`'s default; `UV_INDEX` and
   the file's other indexes are asked before it; `PDM_PYPI_URL` wins over the
   config file's `[pypi] url`; an explicit uv index and a Poetry repository
   are not sources; `UV_NO_CONFIG` leaves `uv.toml` unread; `UV_FIND_LINKS`
   and `find-links` are flat, relative paths resolved as above.
2. A directory's `uv.toml` replaces its `pyproject.toml`'s uv indexes; that
   file's pins resolve against uv's variables, then its own indexes, never
   against a `uv.toml`'s.
3. Two Poetry primary sources are asked in order and PyPI not at all; a
   supplemental one is asked after them, or after PyPI when no source is
   primary; an explicit one serves only its pinned dependency; a source named
   PyPI keeps its place, and one of priority `explicit` leaves only the
   supplemental sources for the other packages.
4. A `Pipfile`'s first source replaces PyPI, the second is asked beside it, a
   pinned package is served by its source alone; `Pipfile.lock` reads alike.
5. PDM's `pypi` source replaces PyPI, a `find_links` source is ignored, a
   package matching `include_packages` of two sources is asked of both alone,
   one excluded is not asked of the excluding source (nor of PyPI when the
   `pypi` source excludes it), and under `respect-source-order` the sources
   are asked in their order around PyPI.
6. End to end, a package the first Poetry primary lacks is answered by the
   second, and a `UV_INDEX`-named index is asked with its credential.
7. A repository's uv indexes are asked before the user `uv.toml`'s and after
   `UV_INDEX`'s, its default wins over the user's and loses to
   `UV_DEFAULT_INDEX`.
8. `unsafe-best-match` (by variable, repository or user file, in that
   precedence), a `find-links` location and PDM sources without
   `respect-source-order` add one note when a package is asked of two
   indexes; `first-index`, `unsafe-first-match` and a single index add none.
9. A repository flat index that is a page stays untrusted until vouched for
   and is then read as a flat page; one that is a path is not recorded.

## Notes

Where several tools of this machine name a replacement, the first read is the
one used, in the order pip, uv, PDM; a repository's uv default index comes
before the user's `uv.toml` default as in uv, but after pip's `index-url`,
which uv does not read. Other repository replacements come after this
machine's. Poetry searches all its primary sources and merges what they have;
the first one that has the package answers here. Under uv's default
`first-index` strategy, an index that has the package but not the pinned
release ends uv's search; here the next index is asked. A repository index
of the same name as one of the user's `uv.toml` does not hide it, the
workspace root's indexes are not pin targets for a member's
`[tool.uv.sources]`, a repository flat index that is a directory is not read,
PDM's `pypi.ignore_stored_index` is not read, and `pdm.toml` sources are
asked beside PyPI under `respect-source-order` too. The system-wide uv file on
Windows is `%ProgramData%\uv\uv.toml`.
