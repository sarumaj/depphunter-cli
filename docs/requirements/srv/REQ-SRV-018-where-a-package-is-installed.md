---
id: REQ-SRV-018
title: Where a package is installed
scope: srv
type: functional
priority: should
status: implemented
verification:
  - unit
  - integration
---

## Statement

`GET /api/locate?id=<package id>` **shall** answer `{"folder": "<absolute
path>"}` with the directory the package is installed in on the machine the
server runs on, and 404 when it finds none. It **shall** look in the directory
a package was installed from (`origin`); in the `node_modules`, `vendor` or
`deps` directory of the project directories from the files that use the
package up to the root; in the project's virtual environment (`.venv`, `venv`,
`env`) or the activated one; and in the shared caches of Go, Cargo, Maven,
Gradle, NuGet, RubyGems and pub, by the package's version.

## Rationale

The code a dependency runs is on the disk already, and reading it there is
quicker than finding it on the web - provided somebody says where it is, which
every package manager answers differently.

## Acceptance criteria

1. An npm package imported from `web/src` is found in `web/node_modules`
   before the root's `node_modules`.
2. A Go module is found in `vendor/` and else in the module cache, its path
   and version escaped as the go command escapes them.
3. A Python distribution is found by its `.dist-info`, as the package its
   `top_level.txt` names.
4. A name that would leave the directory it is joined to (`../..`) is found
   nowhere.
5. A package installed nowhere looked in, and an id that is not a package's,
   answer 404.

## Notes

Nothing is run to ask a package manager, and nothing is fetched.
