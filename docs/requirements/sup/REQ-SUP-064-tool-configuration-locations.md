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
- Yarn Berry: `YARN_NPM_REGISTRY_SERVER`, then the file `YARN_RC_FILENAME`
  names (else `.yarnrc.yml`) in the home directory, its `${VAR}` references
  resolved from the environment; Yarn 1: `~/.yarnrc`.
- Bun: `$XDG_CONFIG_HOME/.bunfig.toml` when it exists, else
  `~/.bunfig.toml`, its `$VAR` references resolved from the environment.
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
- uv: the file `UV_CONFIG_FILE` names alone, when set; else the user's
  `uv.toml` (`$XDG_CONFIG_HOME/uv` or `~/.config/uv`; Windows
  `%APPDATA%\uv`) and the system's (the first `$XDG_CONFIG_DIRS/uv/uv.toml`
  that exists, else `/etc/uv/uv.toml`; Windows `%ProgramData%\uv`); none
  under `UV_NO_CONFIG`.
- Poetry: `config.toml` and `auth.toml` in `POETRY_CONFIG_DIR`, else
  `%APPDATA%\pypoetry` on Windows, `~/Library/Application Support/pypoetry`
  on macOS, else `$XDG_CONFIG_HOME/pypoetry` (`~/.config/pypoetry`).
- PDM: `PDM_CONFIG_FILE`, else `config.toml` in `%LOCALAPPDATA%\pdm\pdm` on
  Windows, `~/Library/Application Support/pdm` on macOS, else
  `$XDG_CONFIG_HOME/pdm` (`~/.config/pdm`).
- Cargo: `config` (else `config.toml`) in `CARGO_HOME`, else in `~/.cargo`;
  `CARGO_REGISTRIES_<NAME>_INDEX` defining a registry or overriding its
  index, the name matched as Cargo spells it in a variable (upper case, `-` as
  `_`).
- Go: `GOPROXY` and `GOAUTH` from the environment, else from the go env file
  (`GOENV`, else `go/env` in the user configuration directory; none for
  `GOENV=off`).
- containers: registries.conf and its drop-ins, and the Docker daemon's
  `daemon.json`
  ([REQ-SUP-068](REQ-SUP-068-container-registry-mirrors.md)).
- NuGet: `%APPDATA%\NuGet\NuGet.Config` on Windows; then the machine-wide
  `*.config` files of `NuGet\Config` under `%ProgramFiles(x86)%` (else
  `%ProgramFiles%`) on Windows, elsewhere under
  `NUGET_COMMON_APPLICATION_DATA`, else `/Library/Application Support` on
  macOS and `/etc/opt` on Linux.
- Composer: `config.json` in the one home Composer takes (`COMPOSER_HOME`;
  `%APPDATA%\Composer` on Windows; else the first existing of
  `$XDG_CONFIG_HOME/composer` or `~/.config/composer`, and `~/.composer`), the
  same home its credentials are read from.
- Maven: `~/.m2/settings.xml`, then `conf/settings.xml` under `MAVEN_HOME`,
  else `M2_HOME`.
- Gradle: `init.gradle(.kts)` and `init.d/*.gradle(.kts)` in
  `GRADLE_USER_HOME`, else `~/.gradle`.
- Clojure: `deps.edn` in `CLJ_CONFIG`, else `$XDG_CONFIG_HOME/clojure`, else
  `~/.clojure`; Leiningen's `profiles.clj` in `LEIN_HOME`, else `~/.lein`.
- sbt: `-Dsbt.repository.config`, else `repositories` in
  `-Dsbt.global.base`, else `~/.sbt`, those properties (and
  `-Dsbt.override.build.repos`) taken from `JAVA_OPTS`, then `SBT_OPTS`.
- Dart: `pub-tokens.json` in `%APPDATA%\dart` on Windows,
  `~/Library/Application Support/dart` on macOS, else
  `$XDG_CONFIG_HOME/dart` (`~/.config/dart`).
- Hex: `HEX_API_URL`, then `HEX_API`, then the `api_url` of `hex.config` in
  `HEX_HOME`, else `$XDG_CONFIG_HOME/hex` (`~/.config/hex`) under `MIX_XDG=1`
  or `true`, else `~/.hex`. rebar3's Hex repositories: the `{hex, [{repos,
  ...}]}` of its global `rebar.config`, `.config/rebar3/rebar.config` under
  `REBAR_GLOBAL_CONFIG_DIR`, else the home directory.
- dub: `settings.json` in `DUB_HOME`, else in `dub` under `DPATH`, else in
  `%APPDATA%\dub` on Windows, else in `~/.dub`; then the system's,
  `%ProgramData%\dub` on Windows, else `/etc/dub` and `/var/lib/dub`.
- Quicklisp: the `distinfo.txt` of each dist in `~/quicklisp/dists`.
- Bazel: `~/.bazelrc` and the files it imports.
- cpanm and Carton: `PERL_CPANM_OPT` and `PERL_CARTON_MIRROR`
  ([REQ-SUP-053](REQ-SUP-053-metacpan-releases.md)).
- opam: its root, `OPAMROOT`, else `%LOCALAPPDATA%\opam` on Windows, else
  `~/.opam` ([REQ-SUP-054](REQ-SUP-054-opam-repository-files.md)).
- Alire: its settings, `ALIRE_SETTINGS_DIR` (Alire 1: `ALR_CONFIG`), else
  `%USERPROFILE%\.config\alire` on Windows, else `$XDG_CONFIG_HOME/alire`
  (`~/.config/alire`) ([REQ-SUP-061](REQ-SUP-061-alire-community-index.md)).
- Julia: the depots of `JULIA_DEPOT_PATH` (`;`-separated on Windows, else
  `:`-separated; an empty entry the default depot, `~` the home directory),
  else `~/.julia` ([REQ-SUP-055](REQ-SUP-055-julia-registry-files.md)).

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
7. `MAVEN_HOME`'s settings, `GRADLE_USER_HOME`'s init scripts, `CLJ_CONFIG`'s
   and `$XDG_CONFIG_HOME/clojure`'s `deps.edn`, `LEIN_HOME` and a moved sbt
   repositories file are read instead of, or beside, the home defaults.
8. `YARN_RC_FILENAME` renames the home Yarn file, and
   `YARN_NPM_REGISTRY_SERVER` replaces its registry; Bun's
   `$XDG_CONFIG_HOME/.bunfig.toml` wins over the home one.
9. `UV_CONFIG_FILE` and `UV_NO_CONFIG`, `$XDG_CONFIG_DIRS` for uv's system
   file, `POETRY_CONFIG_DIR` and `PDM_CONFIG_FILE` are honoured, and the
   Windows and macOS directories used on those platforms (falling through to
   the XDG ones on a Windows without its variables).
10. Dart's configuration directory and Hex's home follow `XDG_CONFIG_HOME`,
    `MIX_XDG`, `HEX_HOME` and the platform, and `HEX_API_URL` wins over
    `HEX_API`, which wins over `hex.config`.
11. rebar3's global `rebar.config` follows `REBAR_GLOBAL_CONFIG_DIR`, and its
    `hex.config` `REBAR_GLOBAL_CONFIG_DIR`, then `REBAR_CACHE_DIR`.
12. dub's user settings follow `DUB_HOME`, `DPATH` and `%APPDATA%` (falling
    through to `~/.dub` on a Windows without it), its system settings
    `%ProgramData%` or `/etc/dub` and `/var/lib/dub`; `~/quicklisp`'s dists
    are listed by name.
13. opam's root follows `OPAMROOT` and `%LOCALAPPDATA%` on Windows; Alire's
    settings follow `ALIRE_SETTINGS_DIR`, `ALR_CONFIG`, `XDG_CONFIG_HOME` and
    the Windows home.
14. Julia's depots follow `JULIA_DEPOT_PATH` with the platform's separator,
    an empty entry being `~/.julia`, each listed once.

## Notes

The locations are found in one place, `internal/userconf`, for index discovery
and the credential store alike. Gradle's installation-wide `init.d` and
`-Dgradle.user.home` in `GRADLE_OPTS` are not read. pip's per-interpreter
`sys.prefix/pip.conf` is not read (the interpreter is not known), nor npm's
built-in prefix guessed. dub's settings beside its executable
(`../etc/dub`) and a system `dubHome` are not followed; Roswell's Quicklisp
(`~/.roswell/lisp/quicklisp`) is not read.
