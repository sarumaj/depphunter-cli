---
id: REQ-SHELL-006
uuid: 31d7f8ad-9373-46d4-9428-c15404cbb5e6
title: direnv and bats commands
scope: shell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

In a `.envrc`, direnv's `source_env` and `source_env_if_exists` **shall** be
imports of the file named, or of the `.envrc` in the directory named, relative
to the `.envrc`'s directory; `source_up` and `source_up_if_exists` of the
nearest file of that name (`.envrc` by default) in a directory above; `dotenv`
and `dotenv_if_exists` of the file named (`.env` by default). In a `.bats` file,
`load NAME` **shall** be an import of `NAME.bash`, else `NAME`, relative to the
test file's directory.

## Rationale

direnv loads environments along the directory tree, and bats tests load their
helpers with `load`; both are sourcing under another name.

## Acceptance criteria

1. The root `.envrc`'s `dotenv` is `.env`, `source_env sub` is `sub/.envrc`,
   `source_env_if_exists .envrc.local` (absent) is dropped; `sub/.envrc`'s
   `source_up` is the root `.envrc` and `dotenv_if_exists ../.env` is `.env`.
2. `load test_helper` in `test/app.bats` is `test/test_helper.bash`.
