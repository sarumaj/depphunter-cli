---
id: REQ-SUP-015
uuid: c673880b-26de-4e0b-91eb-2b17d72748a3
title: Index configuration sources
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read index configuration from this machine:
`NPM_CONFIG_REGISTRY`, `PIP_INDEX_URL`, `PIP_EXTRA_INDEX_URL`, `GOPROXY`
(without `direct` and `off`), `~/.npmrc`, `~/.config/pip/pip.conf`,
`~/.pip/pip.conf`, `~/.cargo/config.toml`, the mirrors of `~/.m2/settings.xml`,
`~/.nuget/NuGet/NuGet.Config`, `~/.config/NuGet/NuGet.Config`, and the
`composer` repositories of Composer's `config.json` in `COMPOSER_HOME`,
`~/.config/composer` and `~/.composer`, Bundler's rubygems.org mirror
(`BUNDLE_MIRROR__RUBYGEMS__ORG`, `~/.bundle/config`), the sources of
`~/.gemrc`, pub's `PUB_HOSTED_URL`, the Hex API of `HEX_API_URL`, and R's
`RENV_CONFIG_REPOS_OVERRIDE` and the `options(repos = ...)` of `~/.Rprofile`
and `R_PROFILE_USER`.

The system **shall** read index configuration from the repository: `.npmrc`,
`.yarnrc.yml`, `pip.conf`, `pip.ini`, `requirements*.txt`, the Poetry and uv
indexes of `pyproject.toml`, `NuGet.config`, the repositories of `pom.xml`,
`.cargo/config.toml`, the `composer` repositories of `composer.json`, the
`source` lines of a `Gemfile` (a `source ... do` block serving only the gems
inside it), the GEM remotes of `Gemfile.lock`, the `hosted:` servers of a
`pubspec.yaml` or `pubspec_overrides.yaml` (each serving its package), the
servers other than pub.dev that `pubspec.lock` resolved packages from, the
repositories other than CRAN of `renv.lock` (each serving the packages
recorded from it) and the `options(repos = ...)` of `.Rprofile` and
`Rprofile.site`.

## Rationale

Where a package comes from is written in the configuration of the package
manager, on the machine and in the repository.

## Acceptance criteria

1. A fixture naming an index in each listed repository file yields that index
   for its ecosystem.
2. Cargo's `replace-with` chain is followed to the source that replaces
   crates.io.
3. When several repository files name an index for one ecosystem, the choice is
   the same on every run.
4. Under `--watch` the repository's sources are re-read on every analysis and
   those no longer named are forgotten.
