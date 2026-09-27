---
id: REQ-SUP-053
uuid: bf79616f-714f-4ce4-b9f9-f9971ff90968
title: CPAN distribution dependencies from MetaCPAN
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a CPAN distribution's dependencies from the
MetaCPAN API (`https://fastapi.metacpan.org` unless configured otherwise):
`<api>/v1/release/<distribution>` is its latest release, whose
`dependency` list gives modules by phase and relationship. The run-time
requirements are the answer, without `perl`; each module **shall** become
the distribution `<api>/v1/module/<module>` says provides it (asked once per
module, and cached), a module only perl provides or MetaCPAN does not know
is left out, and a version is a minimum (`>= 1.2`), never pinned.

## Rationale

MetaCPAN is the index CPAN clients and OSV-style tools read; a release names
modules, while the map's packages are distributions. CPAN mirrors
(`PERL_CARTON_MIRROR`, cpanm's `--mirror`) serve the package index and
tarballs, not this API, so they are not read as indexes. The sandbox this
was built in cannot reach MetaCPAN, so the protocol is verified against a
stub server.

## Acceptance criteria

1. Against a stub server, Plack's release gives HTTP-Message `>= 5.814`
   (HTTP::Message and HTTP::Headers asked once each, one distribution) and
   Try-Tiny; perl, Carp (distribution perl), Plack's own module, an unknown
   module, test requirements and suggestions are left out.
2. With nothing configured, the index for `cpan` is
   `https://fastapi.metacpan.org`, known.

## Notes

The dependencies are the latest release's, whatever version the project
pins (REQ-PERL-009).
