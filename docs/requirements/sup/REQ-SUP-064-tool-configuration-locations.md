---
id: REQ-SUP-064
title: Tool configuration locations
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** look for this machine's index configuration where each
tool looks for it, reading the variables from the machine's environment only:

- npm: the `npm_config_*` variables in any case (the lower-case spelling
  winning; `registry`, `@scope:registry`), then the user's npmrc
  (`npm_config_userconfig`, else `~/.npmrc`), then the global npmrc
  (`npm_config_globalconfig`, else `etc/npmrc` under `npm_config_prefix`).
- pip: its files in pip's load order, each setting replacing the same setting
  of an earlier file - the site-wide files (`$XDG_CONFIG_DIRS/pip/pip.conf`,
  default `/etc/xdg`, then `/etc/pip.conf`; macOS
  `/Library/Application Support/pip/pip.conf`; Windows
  `%ProgramData%\pip\pip.ini`), the user's (`~/.pip/pip.conf`, Windows
  `~/pip/pip.ini`, then `$XDG_CONFIG_HOME/pip/pip.conf` or
  `~/.config/pip/pip.conf`; macOS `~/Library/Application Support/pip` when it
  exists; Windows `%APPDATA%\pip\pip.ini`) unless `PIP_CONFIG_FILE` names an
  existing file, then `PIP_CONFIG_FILE`; `PIP_INDEX_URL` and
  `PIP_EXTRA_INDEX_URL` over all of them. `PIP_CONFIG_FILE` set to the null
  device reads no file.
- Cargo: `config` (else `config.toml`) in `CARGO_HOME`, else in `~/.cargo`;
  `CARGO_REGISTRIES_<NAME>_INDEX` defining a registry or overriding its
  index, the name matched as Cargo spells it in a variable (upper case, `-` as
  `_`).
- Go: `GOPROXY` from the environment, else from the go env file (`GOENV`,
  else `go/env` in the user configuration directory; none for `GOENV=off`).
- NuGet: `%APPDATA%\NuGet\NuGet.Config` on Windows.
- Composer: `config.json` in the one home Composer takes (`COMPOSER_HOME`;
  `%APPDATA%\Composer` on Windows; else the first existing of
  `$XDG_CONFIG_HOME/composer` or `~/.config/composer`, and `~/.composer`), the
  same home its credentials are read from.

## Rationale

CI images and Windows machines seldom keep the configuration at the Unix
defaults: a pipeline points `CARGO_HOME`, `NPM_CONFIG_USERCONFIG` or
`PIP_CONFIG_FILE` elsewhere, and `go env -w` writes GOPROXY to a file. Reading
fixed `$HOME` paths missed the feed the machine actually uses.

## Acceptance criteria

1. With `CARGO_HOME` set, its `config.toml` is read and `~/.cargo`'s is not;
   `CARGO_REGISTRIES_MY_REG_INDEX` serves crates declaring `my-reg`.
2. `npm_config_registry` wins over `NPM_CONFIG_REGISTRY`; the user's npmrc
   wins over the global npmrc; `NPM_CONFIG_USERCONFIG` replaces `~/.npmrc`.
3. pip's layers resolve as pip does, including an existing `PIP_CONFIG_FILE`
   hiding the user's files and `/dev/null` switching all files off.
4. GOPROXY in the go env file is used when the environment does not set it.
5. The Windows paths (`%APPDATA%`, `%ProgramData%`) are used when the platform
   is Windows, tested without running on Windows.
6. Tests read no file of the machine running them.

## Notes

The locations are found in one place, `internal/userconf`, for index discovery
and the credential store alike. Gradle's `GRADLE_USER_HOME` is not needed yet:
nothing is read from Gradle's user home. pip's per-interpreter
`sys.prefix/pip.conf` is not read (the interpreter is not known), nor npm's
built-in prefix guessed.
