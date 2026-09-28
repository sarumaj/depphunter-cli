---
id: REQ-PERL-001
title: Files claimed
scope: perl
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The perl plugin **shall** claim Perl sources - what the scanner labels Perl:
`.pl`, `.pm`, `.t`, `.psgi` and `.PL` files and files without a known
extension (`.cgi`, scripts) whose `#!` line runs perl or perl5.x (not
perl6), and the CPAN manifests `cpanfile`, `Makefile.PL`, `Build.PL`,
`META.json`, `META.yml`, `MYMETA.json`, `MYMETA.yml` and `dist.ini`, except
what Carton installs into a project (`local/lib/perl5`, `local/bin`) and a
build's `blib/`. A `.pl` file without a perl `#!` line or a line starting as
only Perl starts one (`use`, `no`, `require`, `package`, `sub`, `my`,
`our`, `local`, `BEGIN`, POD) that holds a Prolog directive (`:-`) or clause
(`head(X) :- ...`, `a --> ...`) **shall** be labeled Prolog and not
claimed; a `.t` file with neither **shall** have no language and not be
claimed. `cpanfile.snapshot` is labeled Carton and read by the resolver, not
claimed. The manifests' kinds **shall** be part of the cache key.

## Rationale

`.pl` is Prolog's extension too, and `.t` is Perl's only by convention
(templates use it). Perl scripts often have no extension. The markers are
read from the head the scanner already peeks at.

## Acceptance criteria

1. `prolog/family.pl` (`:- module(...)`) is labeled Prolog and not claimed;
   `templates/page.t` is not claimed; `script/shop` (`#!/usr/bin/env perl`)
   and `index.cgi` (`#!/usr/bin/perl5.36.0`) are Perl; a perl6 script is not.
2. `local/lib/perl5/Plack.pm` and `cpanfile.snapshot` are not claimed.
3. `cpanfile`, `Makefile.PL`, `META.json` and `dist.ini` have classes of
   their own; `lib/Foo.pm` and `gen/Foo.pm.PL` have none.
