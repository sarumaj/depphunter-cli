---
id: REQ-CFG-005
uuid: ef418e59-b990-41b3-8758-398e90d07f4e
title: Flags declared and resolved by the config package
scope: cfg
type: constraint
priority: must
status: implemented
verification:
  - unit
  - inspection
---

## Statement

The configuration package **shall** declare every command-line flag in one
function (`RegisterFlags`) and resolve the settings with viper; every registered
flag **shall** set a setting or act on the configuration.

## Rationale

One declaration keeps the flags, the help text and the resolution in step; a
flag bound to nothing would parse, show in `--help` and change nothing.

## Acceptance criteria

1. `config.RegisterFlags` declares all flags of the command.
2. Every registered flag is bound to a setting, is a `--no-*` flag, or is one of
   the flags handled on their own (`config`, `export`, `output`, `exclude`,
   `findings`, `private`, `trust-index`, `embed`, `ui-default`).
