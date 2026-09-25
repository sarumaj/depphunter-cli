---
id: REQ-CFG-006
uuid: a3959387-e4e0-499d-b34e-8b1979b94941
title: Environment variable for every scalar setting
scope: cfg
type: interface
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** provide an environment variable for every scalar setting
except `ui.path_filter`, named `DEPPHUNTER_` followed by the upper-case key with
the `ui.` prefix dropped (for example `DEPPHUNTER_THEME`,
`DEPPHUNTER_MAX_FILE_SIZE`, `DEPPHUNTER_EXPAND_DEPTH`,
`DEPPHUNTER_HISTORY_COMMITS`, `DEPPHUNTER_LSP_TIMEOUT`).

## Rationale

Viper binds each variable by name to keep the established short names (`THEME`,
not `UI_THEME`).

## Acceptance criteria

1. `DEPPHUNTER_ADDR`, `DEPPHUNTER_CACHE`, `DEPPHUNTER_EDITOR`,
   `DEPPHUNTER_HISTORY_COMMITS`, `DEPPHUNTER_LSP_TIMEOUT` and
   `DEPPHUNTER_EXPAND_DEPTH` set their settings.
2. No variable sets `ui.path_filter`.

## Notes

`ui.tool` is bound to `DEPPHUNTER_TOOL`, which it once lacked. List settings
(`exclude`, `findings`, `private`, `trust_indexes`) take comma-separated
variables that add to the configured lists (REQ-CFG-009).
