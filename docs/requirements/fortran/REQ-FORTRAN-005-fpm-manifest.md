---
id: REQ-FORTRAN-005
title: fpm.toml
scope: fortran
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `fpm.toml` and make each dependency of
`[dependencies]`, `[dev-dependencies]`, `[features.*.dependencies]` and of
`[[executable]]`, `[[test]]` and `[[example]]` entries an import of the fpm
package named by its dependency key: a git dependency (`git` with `rev`,
`tag` or `branch`), a registry dependency (`namespace` and `v`) and a
metapackage (`stdlib = "*"`, `minpack`). The `openmp` metapackage
**shall** be the OpenMP intrinsic modules, and `mpi`, `hdf5`, `netcdf` and
`blas` the C libraries of REQ-FORTRAN-007. A path dependency **shall**
resolve to its `fpm.toml` (or directory) when inside the repository and be
dropped otherwise. An explicit `[library] source-dir` **shall** be an
import of that directory, each program's `main` file an import of that file,
`[build] external-modules` imports of the modules' providers (a C library's
module, else a Fortran external module) and `[build] link` imports of the C
libraries. Pinning **shall** follow fpm, which has no lock file: a git
`rev` pins; a `tag` is shown, neither pinned nor floating; a `branch` or no
ref floats; a registry `v` pins and none floats; a metapackage's `"*"`
floats. A git repository on a host other than the public forges (GitHub,
GitLab, Bitbucket, Codeberg, sourcehut) **shall** be recorded as the
package's origin. What `build/cache.toml` records fpm fetched **shall** be
shown as the version of a dependency that is not pinned, with the
requirement as requested.

## Rationale

fpm fetches git dependencies at whatever a tag or branch points to when a
build first runs; only a revision names one version. A dependency key is
the name fpm and other manifests know the package by.

## Acceptance criteria

1. `toml-f` (a `rev`) is pinned, `json-fortran` (a `tag`) shows the
   revision cache.toml records with the tag as requested, `fancy` (a
   `branch` on `git.acme.corp`) floats with that repository as origin,
   `plotter` (`v = "1.2.0"`) is pinned and `stdlib = "*"` floats.
2. `widgets = { path = "libs/widgets" }` resolves to
   `libs/widgets/fpm.toml` and `gone = { path = "../gone" }` is dropped.
3. `executable shop: app/main.f90` resolves to `app/main.f90`.
