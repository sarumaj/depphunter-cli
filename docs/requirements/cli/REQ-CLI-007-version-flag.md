---
id: REQ-CLI-007
uuid: a110c9de-27f2-499c-a8e9-5811df47fefa
title: Version flag
scope: cli
type: interface
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The command **shall** print `depphunter <version>` and exit with status 0 for
`-v` and for `--version`, where `<version>` is the value set at build time, or
`dev` for an untagged build.

## Rationale

Users and the VS Code extension need to know which release runs; release builds
set the version with `-ldflags "-X main.version=..."`.

## Acceptance criteria

1. `depphunter --version` prints `depphunter dev` for a development build.
2. `depphunter -v` prints the same.
3. A release archive built from tag `vX.Y.Z` prints that tag.
