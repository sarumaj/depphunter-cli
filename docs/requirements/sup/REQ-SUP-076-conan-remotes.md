---
id: REQ-SUP-076
title: Conan 2 remotes for Conan packages
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The remotes of Conan 2's `remotes.json` **shall** be read: the one in this
machine's Conan home (`CONAN_HOME`, else `~/.conan2`) as this machine's, and
the one in the home a repository's `.conanrc` names (`conan_home=`, the first
`.conanrc` from a conanfile's directory up to the repository's root, read from
the disk) as the repository's, wherever that home is. Conan asks its remotes
in order, the first that has the recipe answering, and no other: they **shall**
be asked in the file's order, the machine's first, ConanCenter only when it is
one of them or when no `remotes.json` exists (Conan then writes one with
ConanCenter alone). A disabled remote and a `local-recipes-index` **shall**
be left out, and a remote with `allowed_packages` **shall** be asked only for
the references one of its patterns admits, as Conan matches them (`fnmatch`,
`!` or `~` negating, a trailing `@` for no user and channel; a range is
matched as `name/*`). `center2.conan.io` and the older `center.conan.io` are
ConanCenter, the public index, to which a private package is never named. A
remote only the repository names is not asked unless the user vouches for it
or this machine's own `remotes.json` lists the same remote; a credential this
machine holds does not vouch for it (REQ-SUP-043).

With `--online`, the index client **shall** answer as Conan's REST API v2 is
asked: a version range is resolved against the remote's `GET
/v2/conans/search?q=<name>/*` (with `@user/channel` when the reference has
them): the newest version with the same user and channel that the range
admits, as Conan 2 reads ranges (`>=`, `<`, `~`, `^`, `1.2.*`, `*`, `||`,
pre-releases only with `include_prerelease`), a remote that lists none passing
the question on; a recipe revision the reference pins is read as it is, else
`GET /v2/conans/<name>/<version>/<user>/<channel>/latest` (`_` for none) names
the newest; `GET .../revisions/<revision>/files/conanfile.py` is the recipe.
The recipe's `requires` - the attribute's strings and `self.requires()` calls,
read as the cpp plugin reads a conanfile.py - **shall** be the answer, with
their user and channel; tool, build and test requirements **shall not**. A
package the repository's `conan.lock` pins **shall** be asked, and answered,
at the lock's version and recipe revision when the lock's version is the one
required or one the required range admits.

## Rationale

A Conan 2 lock is a flat list and a conanfile names only the direct
requirements: the remote's recipes are where the rest of the graph is.

## Acceptance criteria

1. With `remotes.json` listing an allowed-packages remote, a negated one, a
   disabled one, a local recipes index and ConanCenter, each package is asked
   of the remotes that admit it, in order; `CONAN_HOME` moves the home, and a
   home without `remotes.json` has ConanCenter.
2. A `.conanrc` home's remote is listed after the machine's, unknown until
   vouched for, whatever the machine's credentials; the `.conanrc` nearest a
   conanfile wins, and one without `conan_home` names none.
3. `libcurl/[>=8 <9]` resolves to 8.4.0 (skipping a pre-release, 9.0.0 and
   another user's build), reads its latest revision's conanfile.py and answers
   its requires, the lock's pins (with revision) in place of their ranges; a
   locked package is read at its revision; a user and channel go into the
   path; a range nothing admits and a missing recipe are reported.
4. A remote only the repository names is not asked, and a private package is
   not named to ConanCenter.

## Notes

The conan-io/conan sources were read for the home (`paths.py`), the remote
file (`remotes.py`, `remote.py`), the matching (`refs.py`), the ranges
(`version_range.py`), the resolver (`range_resolver.py`) and the REST routes
(`rest_client_v2.py`, `client_routes.py`, `rest_routes.py`, and the server's
controllers for the answers' shapes); ConanCenter is not reachable from the
sandbox. Not done: a `local-recipes-index` remote's folder is not read,
`verify_ssl: false` is not honored (certificates are always verified), the
`.conanrc` beyond the repository's root is not looked for, and while a
`.conanrc` makes Conan use its home alone, the machine's remotes are still
asked first here. The recipe is not run (REQ-CPP-013): a requirement built at
run time is not seen, and a conditional one counts whatever the condition.
