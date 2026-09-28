---
id: REQ-SUP-015
title: Index configuration sources
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read index configuration from this machine, each file
found where its tool finds it
([REQ-SUP-064](REQ-SUP-064-tool-configuration-locations.md)): npm's registry
settings (`npm_config_registry` and `@scope:registry`, the user's and the
global npmrc), pip's `index-url` and `extra-index-url` (`PIP_INDEX_URL`,
`PIP_EXTRA_INDEX_URL` and pip's configuration files), `GOPROXY` (its proxies
up to the first `direct` or `off`, each with the separator after it; from the
environment or the go env file), Cargo's `config.toml` and
`CARGO_REGISTRIES_<NAME>_INDEX`, the mirrors of `~/.m2/settings.xml`, the
user's `NuGet.Config`, and the `composer` repositories of the `config.json`
in Composer's home (whose credentials are read from
Composer's `auth.json` and `COMPOSER_AUTH`,
[REQ-AUTH-016](../auth/REQ-AUTH-016-composer-auth-json-credentials.md)),
Bundler's rubygems.org mirror (`BUNDLE_MIRROR__RUBYGEMS__ORG`, and the user's
config: `BUNDLE_USER_CONFIG`, `$BUNDLE_USER_HOME/config` or
`~/.bundle/config`, whose `BUNDLE_<HOST>` credentials are read too,
[REQ-AUTH-018](../auth/REQ-AUTH-018-bundler-credentials.md)), the sources of
`~/.gemrc`, pub's `PUB_HOSTED_URL`, the Hex API of `HEX_API_URL`, R's
`RENV_CONFIG_REPOS_OVERRIDE` and the `options(repos = ...)` of `~/.Rprofile`
and `R_PROFILE_USER`, and the `repository` stanzas of cabal's configuration
(`CABAL_CONFIG`, `$CABAL_DIR/config`, `~/.config/cabal/config`,
`~/.cabal/config`), and the `rocks_servers` of LuaRocks' configuration
(`LUAROCKS_CONFIG`, `~/.luarocks/config-5.x.lua`).

The system **shall** read index configuration from the repository: `.npmrc`,
`.yarnrc.yml`, `pip.conf`, `pip.ini`, `requirements*.txt`, the Poetry and uv
indexes of `pyproject.toml` (with the dependencies pinned to an explicit
one), `NuGet.config` (and its `<clear/>`), the repositories other than Maven
Central of `pom.xml` and of Gradle build and settings scripts (outside
`pluginManagement` and `buildscript`), `.cargo/config.toml`, the `composer`
repositories of `composer.json` (and `"packagist.org": false`; a
repository's `auth.json` is not read,
[REQ-AUTH-017](../auth/REQ-AUTH-017-repository-composer-credentials-discarded.md)),
the `source`
lines of a `Gemfile` (a `source ... do` block serving only the gems inside
it; a repository's `.bundle/config` is not read,
[REQ-AUTH-019](../auth/REQ-AUTH-019-repository-bundler-credentials-discarded.md)),
the GEM remotes of
`Gemfile.lock`, the `hosted:` servers of a `pubspec.yaml` or
`pubspec_overrides.yaml` (each serving its package), the servers other than
pub.dev that `pubspec.lock` resolved packages from, the repositories other than
CRAN of `renv.lock` (each serving the packages recorded from it), the
`options(repos = ...)` of `.Rprofile` and `Rprofile.site`, the `repository`
stanzas other than Hackage of `cabal.project` and `cabal.project.local`, and the
`rocks_servers` other than luarocks.org of a project's
`.luarocks/config-5.x.lua`.

## Rationale

Where a package comes from is written in the configuration of the package
manager, on the machine and in the repository.

## Acceptance criteria

1. A fixture naming an index in each listed repository file yields that index
   for its ecosystem.
2. Cargo's `replace-with` chain is followed to the source that replaces
   crates.io; each `[registries.<name>]` is recorded under its name.
3. When several repository files name an index for one ecosystem, the choice is
   the same on every run.
4. Under `--watch` the repository's sources are re-read on every analysis and
   those no longer named are forgotten.
