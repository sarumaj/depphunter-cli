---
id: REQ-AUTH-026
title: uv, Poetry and PDM index credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The system **shall** read the index credentials this machine's Python tools
keep by index name, for the indexes this machine's own configuration names:

- uv: `UV_INDEX_<NAME>_USERNAME` and `UV_INDEX_<NAME>_PASSWORD` (the name
  upper-cased, every character other than a letter or digit as `_`) for the
  index of that name in the user's or system's `uv.toml` or in `UV_INDEX` or
  `UV_DEFAULT_INDEX`.
- Poetry: the `[http-basic.<name>]` `username` and `password` of its
  `config.toml`, `auth.toml` over them, and
  `POETRY_HTTP_BASIC_<NAME>_USERNAME` and `_PASSWORD` over both (each half on
  its own), for the repository of that name in `config.toml` or
  `POETRY_REPOSITORIES_<NAME>_URL`.
- PDM: the `username` and `password` of `[pypi]` and of each `[pypi.<name>]`
  in its global `config.toml`, `PDM_PYPI_USERNAME` and `PDM_PYPI_PASSWORD`
  over `[pypi]`'s.

Each credential **shall** be sent as Basic credentials to its index's path -
the index URL without its trailing `/simple`, the whole host for an index at
the host's root - over what netrc holds for the host.

A credential of these kinds for an index only the repository defines (a
`[[tool.uv.index]]`, `[[tool.poetry.source]]`, `[[tool.pdm.source]]` or
`pdm.toml` source of that name), and a user name or password that is exactly
`$NAME` or `${NAME}` in a repository `Pipfile` or PDM source URL, **shall**
be lent only as
[REQ-AUTH-023](REQ-AUTH-023-repository-feed-credentials-from-the-environment.md)
allows.

## Rationale

A developer or pipeline using uv, Poetry or PDM keeps the company index's
password in the tool's own variables and files, by index name; pip's netrc
and URL forms are not used, so the index answered 401.

## Acceptance criteria

1. `UV_INDEX_<NAME>_*` reach the index of that name in `uv.toml` and in
   `UV_INDEX`, only under its path, over netrc; a name no machine index
   carries sends nothing; under `UV_NO_CONFIG` a name only `uv.toml` defines
   sends nothing.
2. Poetry's `auth.toml` wins over `config.toml`, and the environment's
   password over both, for the repository's URL.
3. PDM's `[pypi]` credential with `PDM_PYPI_PASSWORD` over it and a
   `[pypi.<name>]` credential reach their URLs' paths.
4. End to end, a `UV_DEFAULT_INDEX=name=url` index answers with its
   credential.

## Notes

Poetry's and pip's keyring
([REQ-AUTH-014](REQ-AUTH-014-pip-keyring-not-consulted.md)) is not read.
`POETRY_PYPI_TOKEN_<NAME>` is for publishing and is not read. When several of
these tools file a credential for the same index path, uv's wins, then
Poetry's, then PDM's.
