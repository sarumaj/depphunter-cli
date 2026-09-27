---
id: REQ-PERL-008
title: Distributions resolved, named and pinned
scope: perl
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

CPAN packages **shall** be distributions, named as MetaCPAN names them. A
module no project file provides and perl does not ship **shall** resolve to
the distribution a governing snapshot provides it from, else to the first
of its candidate distributions the governing projects (the manifests in the
file's directory and above, else all) declare: its own - by a curated alias
table (`LWP::*` is libwww-perl, `Mojo::*` Mojolicious, `Moose::*` Moose,
`Test::More` Test-Simple, `Test2::*` Test2-Suite except Test2's core,
`HTTP::Request` HTTP-Message, `DateTime::*` DateTime except its plug-in
namespaces) or the module's name with `::` as `-` - then the namespaces
above it (`Plack::Request` is Plack's), not across a plug-in namespace
(`Plugin`, `Middleware`, `Format`, `Extension`). Otherwise it **shall** be
an unresolved distribution named by the alias table or the module's name.
Pinning: a snapshot pins; `== 1.2` pins (shown bare); a bare version is a
minimum, shown `>= 1.2`, and floats; a range floats as written; no version
or 0 floats.

## Rationale

Requirements are per module, but releases, versions and advisories are per
distribution. Most distributions are named after their main module; a name
guess from the whole module (IO::Socket::SSL is IO-Socket-SSL) beats the
top-level namespace (IO), which would merge unrelated distributions.

## Acceptance criteria

1. `use Mojo::UserAgent` is an unresolved Mojolicious;
   `DateTime::Format::ISO8601` an unresolved DateTime-Format-ISO8601;
   `Some::Unknown::Thing` an unresolved Some-Unknown-Thing.
2. `requires 'Try::Tiny', '>= 0.30, < 1'` with the snapshot is Try-Tiny
   0.31 pinned, requested `>= 0.30, < 1`; `requires 'Path::Tiny', '0.100'`
   without one is `>= 0.100`, floating.
