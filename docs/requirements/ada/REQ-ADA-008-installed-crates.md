---
id: REQ-ADA-008
title: Installed crates and transitive dependencies
scope: ada
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** index the units and project files of the crates
Alire fetched beside a manifest (`alire/cache/dependencies/<crate>_<version>_<hash>/`
and `alire/cache/pins/`), and of the crates the manifest or lock file
names in Alire 2's shared releases cache (`$ALIRE_SETTINGS_DIR/cache`,
else `$XDG_CACHE_HOME/alire`, else `~/.cache/alire`) at the locked or
exactly required version, else the newest there; nothing else in the
shared cache **shall** be read. `--resolve-depth` **shall** follow the
lock file's solution (each release's own `depends-on`, at the versions
the solution chose), else a fetched crate's `alire.toml`, and say which
answers came from what is installed.

## Rationale

What Alire fetched says exactly which units a crate provides; the lock
file records the whole solution.

## Acceptance criteria

1. gnatcoll depends on libgpr 25.0.1 (`^25` requested) by the lock file;
   simple_components, fetched but not locked, on a floating strings_edit
   and tables 1.14.0 from its manifest; utilada is read from the shared
   cache at 2.6.0, the newest of two releases, and its parent unit
   `Util.Log` attributes `Util.Log.Loggers`.
