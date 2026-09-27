---
id: REQ-SHELL-008
uuid: 96c8f662-676d-4103-8aab-5178f0a10ef5
title: Pinning rule for installed packages
scope: shell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An installed package **shall** be pinned by the rule of its ecosystem: pip
`==1.2.3` (shown as `1.2.3`), npm and Go a complete version (`5.3.3`,
`v0.14.2`, a Go pseudo-version), cargo a complete version (`=` dropped), gem an
exact version (`=` dropped). Any other version or range **shall** be shown but
not pinned; no version, or `latest`, **shall** float; a version still holding a
variable **shall** be shown as written and pin nothing.

## Rationale

`pip install x` and `go install x@latest` install whatever is newest when the
script runs; `npm i x@1.2` and `go install x@v1.2` are queries for the newest
1.2.x.

## Acceptance criteria

1. `requests[socks]==2.31.0`, `black==24.1.0`, `typescript@5.3.3`,
   golangci-lint at `v1.55.2`, gopls at `v0.14.2`, `ripgrep --version 13.0.0`,
   `bundler -v 2.5.3` and `rake:13.1.0` are pinned; `flask>=2.0`,
   `@angular/cli@^17` and `cargo-edit@0.12` are not; `pip`, `yarn`,
   `rubocop` and `goimports@latest` float.
