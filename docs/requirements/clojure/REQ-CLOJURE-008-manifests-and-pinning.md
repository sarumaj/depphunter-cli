---
id: REQ-CLOJURE-008
title: Manifests read, artifacts named and pinned
scope: clojure
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read the dependencies of `deps.edn` and `bb.edn`
(`:deps`, aliases' `:extra-deps`, `:replace-deps`, `:deps`,
`:override-deps`, `:default-deps`), `project.clj` (`:dependencies`,
`:plugins`, profiles' `:dependencies` and `:plugins`, versions from
`:managed-dependencies`; computed `~` values unknown), `shadow-cljs.edn`
(`:dependencies`) and `build.boot` (`set-env!` `:dependencies`) as imports,
each lib once. A lib `g/a` is the Maven package `g:a` and a lib without a
group `a:a`; a Maven version pins it by Maven's rule (a plain version;
`RELEASE`, `LATEST`, ranges and snapshots float); a git dependency is
`g:a` with its `:git/url` (or the repository its name implies:
`io.github.o/r` is `https://github.com/o/r.git`) as origin, pinned by a
full `:git/sha` (the tag as the requested ref), shown with its tag alone
but neither pinned nor floating, floating with neither; a `:local/root`
dependency, and an artifact a project of the repository is named after
(`defproject`), **shall** be that project's manifest.

## Rationale

Clojure's libraries are Maven artifacts (Clojars is a Maven repository), so
they share the Maven island and its pin rule; the full name group:artifact is
what a POM, OSV and Trivy address.

## Acceptance criteria

1. The fixture's `deps.edn` imports `io.github.acme/widgets` pinned by its
   sha with `v1.2.0` requested, `io.github.acme/gadgets` at `v0.3` neither
   pinned nor floating, `com.taoensso/timbre` `RELEASE` floating and
   `acme/shared` as `libs/shared/deps.edn`.
2. `project.clj`'s `[hiccup ~hiccup-version]` takes `1.0.5` from
   `:managed-dependencies`, and `compojure` is `compojure:compojure`.
