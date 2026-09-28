---
id: REQ-SUP-063
title: Additive index sources keep the public index behind them
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** model each unscoped index source as either replacing the
ecosystem's public default or being asked beside it, as the package manager
does: pip's `extra-index-url` (`PIP_EXTRA_INDEX_URL`, `pip.conf`,
requirements files), a supplemental Poetry source, a uv index that is not
the default, a Pipfile source after the first and a PDM source not named
`pypi`, the repositories of a POM, a Gradle script, an sbt build, a
Clojure manifest, an active Maven settings profile and the Gradle, Clojure,
Leiningen and sbt user configuration, a Maven mirror of a repository other
than `central`, `*` and `external:*`,
Composer repositories and NuGet feeds are asked beside it; `index-url`, a
default uv index, a Pipfile's first source, a PDM source named `pypi`, a
Maven mirror of `central`, a
Maven mirror of `*` (which also stands in for every repository asked beside
Central), Cargo's `replace-with` and every other ecosystem's unscoped source
replace it (so does a Maven settings profile or Clojure repository with the
id `central`); `GOPROXY`, `COURSIER_REPOSITORIES`, sbt's repositories file
under `-Dsbt.override.build.repos=true` and Poetry's default and primary
sources are ordered lists that replace it.

For each package the index client **shall** ask the sources beside the public
default first, in the order found, then the public default or the source that
replaces it (every entry of an ordered list, `GOPROXY`'s or Poetry's, in
order), and **shall** move on to the
next index only when one answers that it does not have the package (HTTP 404
or 410, or an answer that does not list it), or after any failure of a
`GOPROXY` entry followed by `|`. Any other failure **shall** end the question.

A scoped source covering the package, the NuGet sources a
`packageSourceMapping` pattern maps the package to
([REQ-SUP-065](REQ-SUP-065-nuget-configuration-layers-and-source-mapping.md)),
a Cargo alternative registry, and the
host an image or Terraform module names **shall** serve the package alone,
with no fallback. A Cargo alternative registry **shall** serve only the crates
that declare it (`registry = "<name>"` in `Cargo.toml`, or its index URL as
`Cargo.lock`'s `source`), and every other crate **shall** be asked of
crates.io or what replaces it.

A source only the repository names and that nobody vouched for **shall** be
skipped rather than asked, and a package that no index the client may ask has
**shall** be attributed to it and marked. A package matching a private
pattern **shall not** be asked of a public index at any position of the list.

Composer's `"packagist.org": false`, a NuGet configuration that leaves
nuget.org out (a `<clear/>` no closer entry for nuget.org follows, or a
disabled nuget.org), and a set `GOPROXY` **shall** switch the public default off;
`GOPROXY` entries after `direct` or `off` are not reached.

## Rationale

Treating an additive source as a replacement shadowed the public index: a
repository's `--extra-index-url` marked every PyPI package as coming from an
index nothing here vouches for, and a machine's extra index was asked about
every public package and answered none. Asking the organization's own index
first is what Maven and Composer do, and it keeps the name of a package that
index has from ever reaching the public one. Falling back only on "not found"
keeps an index that is down from being silently replaced by another index's
package of the same name; a scoped source never falls back because a package
missing from it and present on the public index is what dependency confusion
plants.

## Acceptance criteria

1. With `PIP_EXTRA_INDEX_URL` set, a package the extra index lacks is answered
   by PyPI, and a package it has is answered by it and never named to PyPI.
2. With `--extra-index-url` only in a requirements file, PyPI packages are
   attributed to PyPI, known, and answered by it; a package PyPI lacks is
   reported as coming from the repository's index, which receives no request.
3. A POM's repository is asked before Maven Central when vouched for, and
   skipped when not; Central answers what the repository lacks.
4. A Composer repository is asked before Packagist; with `"packagist.org":
   false` Packagist is not asked.
5. A package on the second of two NuGet feeds is answered by it after the
   first answers 404, and nuget.org is asked last.
6. `GOPROXY=A,B` asks B after A's 404 or 410 and not after A's 500;
   `GOPROXY=A|B` asks B after A's 500; `direct` and `off` end the list; and
   proxy.golang.org is not asked when `GOPROXY` is set.
7. With `[registries.corp]` configured, a crate declaring `registry = "corp"`
   (or its `Cargo.lock` index URL) is asked of corp alone, and every other
   crate of crates.io; a crates.io dependency of a corp crate is crates.io's.
8. An npm package of a scope whose registry lacks it is not asked of the
   public registry.
9. A package matching a private pattern that the extra index lacks is not
   asked of the public index.
10. Of two Poetry primary sources, the second answers what the first lacks,
    and PyPI is not asked.

## Notes

Before anything is asked, a package is attributed to its scoped source, its
registry, the replacing source or the public default; with `--online` the map
then shows the index that answered (Client.Located). Repository sources of
other ecosystems (R `repos`, cabal repositories, LuaRocks `rocks_servers`,
Podfile sources) still replace the public default.
