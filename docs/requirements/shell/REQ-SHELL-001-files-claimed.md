---
id: REQ-SHELL-001
title: Shell scripts claimed
scope: shell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The shell plugin **shall** claim files with the extensions `.sh`, `.bash`,
`.zsh`, `.ksh`, `.bats` and `.zsh-theme` (case-insensitively), the startup
files shells and direnv read by name (`.envrc`, `.profile`, `.bashrc`,
`.bash_profile`, `.bash_login`, `.bash_logout`, `.bash_aliases`, `.zshrc`,
`.zshenv`, `.zprofile`, `.zlogin`, `.zlogout`, `.kshrc`), and files whose name
gives no known language and whose first line is a `#!` line running `sh`,
`bash`, `zsh`, `dash`, `ksh`, `mksh` or `ash`, directly or through
`/usr/bin/env` (with its options and assignments skipped). The scan
**shall** record the program of every file's `#!` line from the bytes it
already reads to tell binary files apart, and label such an extensionless
shell script `Shell`.

## Rationale

Build, CI, install and deployment scripts are often extensionless executables
under `bin/`, `hack/` or `scripts/` (dokku's plugins are `commands`,
`functions`, `subcommands/*`); only the `#!` line says they are shell. The scan
already peeks at the first 8000 bytes of every file, so reading the line costs
nothing more.

## Acceptance criteria

1. `a.sh`, `lib/x.BASH`, `themes/r.zsh-theme`, `t/x.bats`, `home/.zshrc` and
   `.envrc` are claimed; `bin/deploy` whose first line is
   `#!/usr/bin/env -S bash -e` is claimed and labeled Shell; `bin/tool`
   running `python3` and `lib/x.py` running `sh` are not claimed (`lib/x.py`
   stays Python); a binary `a.sh` and `README` are not claimed.
