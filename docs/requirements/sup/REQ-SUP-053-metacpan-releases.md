---
id: REQ-SUP-053
title: CPAN distribution dependencies from MetaCPAN and CPAN mirrors
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a CPAN distribution's dependencies from the
MetaCPAN API (`https://fastapi.metacpan.org`). A distribution at one version
(`1.0050`, `v2.1.3`) **shall** be answered by that release:
`<api>/v1/release/<AUTHOR>/<name>` when the repository's `cpanfile.snapshot`
records the release's `pathname` (`A/AU/AUTHOR/Dist-1.23.tar.gz`), else the
release `<api>/v1/release/_search` finds by distribution and version; when
MetaCPAN has no such release, the latest release's dependencies **shall** be
given with a `no-release` note. Any other requirement (a range, a minimum,
none) is answered by `<api>/v1/release/<distribution>`, the latest release.
The run-time requirements of the release's `dependency` list are the answer,
without `perl`; each module **shall** become the distribution
`<api>/v1/module/<module>` says provides it (asked once per module, and
cached), a module only perl provides or MetaCPAN does not know is left out,
and a version is a minimum (`>= 1.2`), never pinned.

The CPAN mirrors this machine installs from **shall** be indexes: the
`--mirror` URLs of `PERL_CPANM_OPT` under `--mirror-only` (or `--from`),
asked in order, with MetaCPAN switched off unless one of them is a public CPAN
mirror (which stands for MetaCPAN); and `PERL_CARTON_MIRROR`, asked before
MetaCPAN. A mirror without `--mirror-only` only serves archives and is not an
index. A mirror (http, https, `file:` or a directory) **shall** be read from its
`modules/02packages.details.txt.gz` (else `.txt`), once: a distribution it does
not list is not found there, and the next index is asked. One it lists
**shall** be answered with MetaCPAN's same release - the author the mirror's
path names, the version asked for or else the mirror's - each module named by
the distribution the mirror lists for it. A distribution a private pattern
covers **shall not** be named to MetaCPAN, and one MetaCPAN does not describe
is answered without dependencies and with a `no-release` note naming the
mirror's archive.

## Rationale

MetaCPAN is the index CPAN clients and OSV-style tools read; a release names
modules, while the map's packages are distributions. A pinned version's
dependencies are that release's, not the latest's. A DarkPAN (Pinto,
OrePAN2, a minicpan) is what a company's cpanm resolves from, and a package
it lacks is not the company's; its dependencies are only in the archive,
which is not downloaded. The sandbox this was built in cannot reach MetaCPAN,
so the protocol is verified against stub servers.

## Acceptance criteria

1. Against a stub server, Plack's latest release gives HTTP-Message
   `>= 5.814` (HTTP::Message and HTTP::Headers asked once each, one
   distribution) and Try-Tiny; perl, Carp (distribution perl), Plack's own
   module, an unknown module, test requirements and suggestions are left
   out.
2. With nothing configured, the index for `cpan` is
   `https://fastapi.metacpan.org`, known.
3. Plack 1.0050, which the snapshot records as MIYAGAWA's, asks
   `/v1/release/MIYAGAWA/Plack-1.0050`; Plack 1.0047 is searched for; Plack
   0.9, which the search does not find, gives the latest release's
   dependencies and a note; `>= 1.0` asks for the latest release.
4. `PERL_CPANM_OPT` with `--mirror-only`, `--from` or a public mirror in the
   list, without `--mirror-only`, with a directory, and `PERL_CARTON_MIRROR`
   give the expected indexes in order.
5. Against a stub mirror, 02packages is read once; Plack is answered with the
   release by its mirror's author, a pinned Plack with that version's, a
   distribution MetaCPAN lacks with no dependencies and a note, and neither a
   private nor an unlisted distribution is named to MetaCPAN; a directory
   mirror is read from the disk, and a distribution it lacks is asked of
   MetaCPAN after it.

## Notes

A DarkPAN distribution that is not on CPAN has no dependencies on the map:
only its archive's `META.json` holds them. A distribution a private pattern
covers is answered by the mirror alone; one it does not cover is named to
MetaCPAN when the mirror has it, so a DarkPAN's own distributions should be
covered by private patterns. cpm's resolvers, `cpanfile` mirrors and cpanm's
`--mirror-index` are not read.
