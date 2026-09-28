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
it, each entry optionally `name=url`, then the `[[index]]` entries (and the
legacy `index-url` and `extra-index-url`) of the user's and the system's
`uv.toml` ([REQ-SUP-064](REQ-SUP-064-tool-configuration-locations.md)): an
index with `default = true` replacing PyPI, one with `explicit = true` serving
only the packages pinned to it, any other asked beside PyPI. It **shall** read
PDM's global `config.toml` with `PDM_PYPI_URL` over it: `[pypi] url` replacing
PyPI and each `[pypi.<name>]` asked beside it. Poetry's `config.toml`
repositories and `POETRY_REPOSITORIES_<NAME>_URL` **shall not** be sources.

The system **shall** read the repository's Python tool configuration, each
index untrusted:

- uv: the `[[tool.uv.index]]` entries of `pyproject.toml` as above, replaced
  by those of a `uv.toml` in the same directory (not under `UV_NO_CONFIG` or
  `UV_CONFIG_FILE`), and the packages `[tool.uv.sources]` pins with
  `index = "<name>"`, resolved against the directory's indexes, then this
  machine's (such an index stays trusted).
- Poetry: the `[[tool.poetry.source]]` entries - the sources of priority
  `default` and then `primary` (a source stating none) replacing PyPI together
  as an ordered list, a source named PyPI without a URL being PyPI at its
  place in it; `supplemental` (and the legacy `secondary`) asked beside
  them; `explicit` serving only the dependencies (of any group) that name it
  with `source = "<name>"`.
- Pipenv: the `[[source]]` entries of a `Pipfile` and the `_meta.sources` of
  a `Pipfile.lock` - the first replacing PyPI, the others asked beside it -
  and the packages of any category pinned with `index = "<name>"`.
- PDM: the `[[tool.pdm.source]]` entries - one named `pypi` replacing PyPI,
  the others asked beside it, a `find_links` one no index - with each
  `include_packages` pattern serving the matching packages alone; a
  `pdm.toml`'s `[pypi]` and `[pypi.<name>]` as above.

A package pinned to an index **shall** be asked of that index alone. Package
names **shall** be compared as PEP 503 normalizes them, and an
`include_packages` pattern as a glob. A repository index URL whose host or
path holds a variable reference **shall not** be recorded, and the
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
   are not sources; `UV_NO_CONFIG` leaves `uv.toml` unread.
2. A directory's `uv.toml` replaces its `pyproject.toml`'s uv indexes, whose
   pins still resolve against `uv.toml` or this machine's indexes.
3. Two Poetry primary sources are asked in order and PyPI not at all; a
   supplemental one is asked too; an explicit one serves only its pinned
   dependency; a source named PyPI keeps its place.
4. A `Pipfile`'s first source replaces PyPI, the second is asked beside it, a
   pinned package is served by its source alone; `Pipfile.lock` reads alike.
5. PDM's `pypi` source replaces PyPI, a `find_links` source is ignored, and a
   package matching `include_packages` is asked of its source alone.
6. End to end, a package the first Poetry primary lacks is answered by the
   second, and a `UV_INDEX`-named index is asked with its credential.

## Notes

Where several tools of this machine name a replacement, the first read is the
one used, in the order pip, uv, PDM; a repository's replacement comes after
this machine's, although uv itself would put the project's indexes first.
Indexes asked beside PyPI are asked before it, although Poetry asks its
supplemental sources after its primary ones: the organization's index is
asked first so that a package it has is never named to PyPI. PDM's
`exclude_packages` and `respect-source-order`, uv's `index-strategy` and
`flat` indexes, and `[tool.uv.sources]` of workspace members other than the
file's own are not read. The system-wide uv file on Windows is
`%ProgramData%\uv\uv.toml`.
