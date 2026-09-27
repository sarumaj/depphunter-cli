---
id: REQ-SHELL-007
uuid: b3a5440d-ede9-4601-82b5-8b2cd3cec4b5
title: Packages installed by scripts
scope: shell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Packages a script installs with a language's package manager **shall** be
imports of packages in that manager's ecosystem, under the ids and names the
manifests' plugins use: `pip install`, `pipN install`, `python -m pip install`,
`uv pip install`, `uv tool install` and `pipx install` requirement specifiers
(extras and markers dropped) in `pypi`, with `-r FILE` an import of that
file (as REQ-SHELL-004's bare paths); `npm install|i|add`, `pnpm add|install`,
`yarn add`, `yarn global add` and `bun add|install` `name[@version]` in `npm`;
`go install|run|get pkg@version` in `go`, named by the module: the path before
`/cmd/`, on github.com, gitlab.com, bitbucket.org and codeberg.org
host/owner/repo (plus a major-version element), a few known nested modules
(`golang.org/x/tools/gopls`) kept; `cargo install name[@version]` or
`--version` in `crates` (not with `--git` or `--path`); `gem install
name[:version]` or `-v` in `rubygems`. Paths, URLs, archives, git specs and
aliases **shall not** be recorded. A version written with a variable the file
assigned a literal **shall** use that literal.

## Rationale

A CI script's `pip install awscli==1.32.0` or `go install ...golangci-lint@v1.55.2`
decides what runs in the build as much as a manifest does, and landing in the
ecosystems the manifests use means vulnerability lookups, pinning reports and
private-pattern rules cover them.

## Acceptance criteria

1. The fixture's CI script yields `requests` (from `"requests[socks]==2.31.0"`),
   `flask`, `black` (via `python3 -m pip`), `pip`, the file
   `requirements-dev.txt`, `typescript`, `@angular/cli`, `yarn`,
   `github.com/golangci/golangci-lint` at `v1.55.2` (from
   `@${GOLANGCI_VERSION}`), `golang.org/x/tools` (goimports),
   `golang.org/x/tools/gopls`, `ripgrep`, `cargo-edit`, `bundler`, `rake` and
   `rubocop`, and nothing for `npm i ./local-pkg`, `go install ./cmd/...`,
   `cargo install --git ...`, `apt-get install` or `brew install`.
