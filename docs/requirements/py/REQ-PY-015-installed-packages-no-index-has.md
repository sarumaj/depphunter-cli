---
id: REQ-PY-015
uuid: b3b4f942-c3d4-46a7-845f-025cf832ca10
title: Installed packages no index has
scope: py
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

A Python import that no manifest resolves **shall** be resolved against the
distributions installed in a Python environment: the interpreter named with
`--python`, else an activated `VIRTUAL_ENV`, else the project's own `.venv` or
`venv`. The environment's site-packages **shall** be read from disk, including
the base interpreter's when the virtual environment includes system
site-packages, and the interpreter **shall not** be run. A distribution
installed from somewhere other than an index (`direct_url.json`, PEP 610)
**shall** resolve the import under its installed name and version, **shall** be
marked private and record where it was installed from, **shall** be attributed
to that origin rather than to any index, and **shall** take its transitive
dependencies from its `Requires-Dist`; no index and no vulnerability database
**shall** be asked about it. A distribution installed from an index that no
manifest declares **shall** leave the import unresolved, under that
distribution's name. An import that an installed distribution provides **shall**
resolve to that distribution when the project declares it, whatever the import's
name. The per-user site directory is not read. A project configuration file
**shall not** choose the interpreter.

## Rationale

An in-house package installed from a directory, a wheel file or a Git
repository exists on no index, so nothing a repository declares can say what it
is; the environment it runs in can. Running the interpreter would run whatever
the repository's own `.venv` contains.

## Acceptance criteria

1. An import of a module installed from a local directory resolves to that
   distribution, with its installed version and its origin.
2. A declared distribution installed from a local directory keeps its declared
   version and gains its origin; with no declared version it takes the installed
   one.
3. The dependencies of an installed-from-elsewhere distribution are its
   `Requires-Dist`, without those only an extra asks for.
4. An import of a module installed from an index and not declared remains
   unresolved, named after its distribution.
5. Without `--python`, `VIRTUAL_ENV` or a project `.venv`/`venv`, no environment
   is read.
6. The index client asks nothing about a package with an origin, and the map
   marks it private.
7. A package with an origin carries no index and is not marked as coming from an
   index nothing here vouches for, even where the repository names an index for
   its ecosystem; the resolution report counts it under
   "installed from outside any index".
