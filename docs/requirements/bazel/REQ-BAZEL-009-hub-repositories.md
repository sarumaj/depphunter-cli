---
id: REQ-BAZEL-009
uuid: edac2691-52ee-452d-8a27-383ff1f01725
title: Hub repositories of other ecosystems
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The hub repositories module extensions and repository rules create
**shall** resolve to the packages of the ecosystem they install, named as
that ecosystem's plugins name them so the nodes meet: rules_jvm_external's
`maven.install`/`maven.artifact`/`maven_install` (`@maven//:group_artifact`
and `artifact("g:a")`) to Maven `group:artifact` at the version its
lock file (`lock_file`, `maven_install_json`) pins, else the declared one
(`lang.PinnedMaven`), with the lock's dependency lists as `lang.Transitive`;
rules_python's `pip.parse`/`pip_parse` hubs (`@pypi//name`,
`@pypi_name//:pkg`, `requirement("name")`) to the PyPI distribution as its
requirements lock spells it, pinned by `==`; Gazelle's `go_deps`
(`from_file` go.mod requirements, `module` tags) and `go_repository` by
Gazelle's repository names (`@com_github_pkg_errors//:errors`) to Go
modules; rules_js' `npm_translate_lock` (`//:node_modules/name`,
`@npm//name`) to npm packages at the version pnpm-lock.yaml resolved; and
rules_rust's crate_universe (`from_cargo` Cargo.lock, `crate.spec`,
`crates_repository`) (`@crates//:name`) to crates. The artifacts, Go module
tags and crate specs a manifest declares **shall** be imports of it.

## Rationale

A Bazel repository's third-party code is mostly installed this way; the
packages are what OSV and the other plugins know.

## Acceptance criteria

1. `@maven//:com_google_guava_guava` is `com.google.guava:guava`
   32.1.2-jre (the lock) requested 32.0.0-jre; `requirement("requests")` is
   requests 2.32.3; `@pypi//pyyaml` is PyYAML; `@org_golang_x_sys//unix` is
   golang.org/x/sys v0.22.0; `:node_modules/lodash` is lodash 4.17.21;
   `@crates//:serde` is serde 1.0.204.
2. Guava's lock entry gives failureaccess as its dependency.
