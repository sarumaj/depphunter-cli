---
id: REQ-SUP-054
title: opam package dependencies from opam repositories
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read an opam package's dependencies from an opam
repository: `packages/<name>/<name>.<version>/opam`, from the copy opam keeps
of the repository on this machine when there is one, else over HTTP
(`https://raw.githubusercontent.com/ocaml/opam-repository/master` for
opam-repository). A version that is not exact (a range, or none) **shall** be
answered by the newest version the repository lists that it admits, in opam's
version order (Debian's: `~` before everything, `1.10` after `1.9`): listed
from the copy, or for opam-repository through GitHub's contents API
(`/repos/ocaml/opam-repository/contents/packages/<name>`, once per package).
An HTTP repository with neither **shall** not be asked about a range (no
version to ask for); a repository only git serves, with no copy, **shall** not
be read at all and passes the question on, with a `no-copy` note. The answer
is the description's `depends` without the compiler's packages and what only
tests, documentation or development need (`with-test`, `with-doc`,
`with-dev-setup`, `dev`); a dependency written `{= "1.2"}` is that version
(pinned), else its constraint as written.

The repositories **shall** be discovered. This machine's: the URL of each in
the opam root's `repo/repos-config` (`OPAMROOT`, else `~/.opam`), in the order
of priority the current switch gives (`OPAMSWITCH`, else the root config's
`switch`: its `switch-config`'s `repositories`, else the root config's), each
with the copy in `repo/<name>/` or `repo/<name>.tar.gz`; opam.ocaml.org (or
the opam-repository git URL) is opam-repository, switched off when the switch
does not use it. The repository's: a dune-workspace's `repository` stanzas, in
its first `lock_dir`'s `repositories` order (`:standard` being overlay and
upstream), opam-repository switched off when that list leaves it out; beside
it when no `lock_dir` lists any. A package is asked of the repositories in
order, the next one when a repository does not have it.

## Rationale

opam.ocaml.org serves the repository as an archive and its package pages
as HTML; the git repository serves each description as a file, which is
one request per package and version. opam keeps every repository it uses on
disk, which lists versions without the network and reads private
repositories, whatever host serves them.

## Acceptance criteria

1. Against a stub server, lwt 5.9.1 gives cppo `>= 1.1.0`, dune `>= 3.8`
   and ocplib-endian 1.2 (pinned); ocaml, base-threads, with-test and
   with-doc dependencies and a repeated package are left out.
2. fmt `>= 0.9` is not asked of an HTTP repository that cannot list; with
   nothing configured, the index for `opam` is the opam-repository files URL,
   known.
3. repos-config, the root config, a switch-config and `OPAMSWITCH` give the
   repositories in the switch's order with their copies (a directory, an
   archive), opam-repository off when unused; `%LOCALAPPDATA%\opam` on
   Windows.
4. A dune-workspace's repositories are the repository's own, in its
   lock_dir's order, and switch opam-repository off when left out.
5. Ranges are answered from a directory copy and an archive copy by the
   newest version admitted, the first repository with the package deciding,
   with nothing sent; without a copy, opam-repository is listed once through
   GitHub, and a git-only repository gives a `no-copy` note.

## Notes

GitHub's contents API allows 60 unauthenticated requests an hour; a
credential for api.github.com in the netrc is sent with them. opam's own
solver is not run: the newest admitted version of each package is taken
alone, and `avoid-version` flags and a switch's installed versions are not
considered.
