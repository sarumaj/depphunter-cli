---
id: REQ-AUTH-020
title: Credential file locations
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read credentials from the files each tool itself reads,
found through the tool's variables and platform paths in the machine's
environment:

- npm: the global npmrc, then the user's (`npm_config_userconfig`, else
  `~/.npmrc`), then `npm_config_//<host>/:<field>` variables in any case, each
  key replacing the same key before it.
- Yarn Berry and Bun: the home `.yarnrc.yml` (named by `YARN_RC_FILENAME`)
  with the `YARN_NPM_*` variables over it, and Bun's global bunfig
  ([REQ-AUTH-024](REQ-AUTH-024-yarn-and-bun-credentials.md)).
- netrc: `NETRC`; else `~/_netrc` on Windows when it exists; else `~/.netrc`.
- containers: `REGISTRY_AUTH_FILE` alone when set; else
  `$XDG_RUNTIME_DIR/containers/auth.json` on Linux
  (`~/.config/containers/auth.json` elsewhere),
  `$XDG_CONFIG_HOME/containers/auth.json` (`~/.config/containers/auth.json`),
  then `config.json` in `DOCKER_CONFIG`, else `~/.docker`; the first file
  holding a registry's credential being the one used, as
  containers-auth.json(5) orders them.
- Cargo: `credentials` (else `credentials.toml`) and `config` (else
  `config.toml`) in `CARGO_HOME`, else `~/.cargo`, and the registries
  `CARGO_REGISTRIES_<NAME>_INDEX` defines.
- NuGet: `%APPDATA%\NuGet\NuGet.Config` on Windows, and the machine-wide
  files index discovery reads
  ([REQ-SUP-064](../sup/REQ-SUP-064-tool-configuration-locations.md)).
- Composer: the same single home index discovery reads.
- Maven: `~/.m2/settings.xml`, then `conf/settings.xml` under `MAVEN_HOME`,
  else `M2_HOME`; the Clojure CLI's `deps.edn` in `CLJ_CONFIG`, else
  `$XDG_CONFIG_HOME/clojure`, else `~/.clojure`, for its repository names.
- sbt, Coursier and Gradle: the files of
  [REQ-AUTH-021](REQ-AUTH-021-jvm-build-tool-credentials.md).
- uv, Poetry and PDM: the `uv.toml` files, Poetry's configuration directory
  and PDM's `config.toml` index discovery reads, and the tools' variables
  ([REQ-AUTH-026](REQ-AUTH-026-python-tool-credentials.md)).
- pub: `pub-tokens.json` in Dart's configuration directory
  ([REQ-AUTH-027](REQ-AUTH-027-pub-tokens.md)).
- Hex: `hex.config` in `HEX_HOME`, the XDG directory under `MIX_XDG`, else
  `~/.hex`; rebar3's `.config/rebar3/hex.config` under
  `REBAR_GLOBAL_CONFIG_DIR`, else `REBAR_CACHE_DIR`, else the home directory;
  and Hex's variables
  ([REQ-AUTH-028](REQ-AUTH-028-hex-keys.md)).

## Rationale

A credential kept where the tool looks for it - a CI job's `NETRC`, a
`DOCKER_CONFIG` directory, Podman's runtime `auth.json` - is the one the tool
sends; reading only the `$HOME` defaults left those registries answering 401.

## Acceptance criteria

1. `NETRC` replaces `~/.netrc`; on Windows `~/_netrc` is preferred.
2. `$XDG_RUNTIME_DIR/containers/auth.json` wins over `DOCKER_CONFIG`'s
   `config.json` for the same registry; `REGISTRY_AUTH_FILE` is read alone.
3. `npm_config_//a.corp/:_authToken` replaces the user's npmrc key, which
   replaces the global npmrc's.
4. A token for a registry `CARGO_REGISTRIES_<NAME>_INDEX` defines is sent to
   that index's host; `CARGO_HOME`'s files replace `~/.cargo`'s.
5. Nothing under the repository is read (REQ-AUTH-012).
