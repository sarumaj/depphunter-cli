---
id: REQ-PERL-005
uuid: 3bdb2581-d8b6-42d4-bfa2-008660bc3310
title: Core modules
scope: perl
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module perl ships (Module::CoreList for perl 5.38.0, without the VMS, OS2,
Amiga, Haiku and Win32 ones) **shall** resolve to the hidden `perl-std`
island, named by the module, unless the project installs it from CPAN: a
governing cpanfile.snapshot provides it, or a governing manifest requires it
or another module of its distribution with a version other than 0 (a
dual-life module: `List::Util 1.45` makes `Scalar::Util` the
Scalar-List-Utils distribution). A manifest's requirement of a core module
without a version is `perl-std` too.

## Rationale

Like Ruby's default gems: a dual-life module required with a version is
installed from CPAN when perl's copy is older, and it is then the CPAN
release that matters.

## Acceptance criteria

1. `use Data::Dumper` and `use POSIX` are perl-std; `use Scalar::Util` with
   the snapshot providing it is Scalar-List-Utils 1.63 (pinned).
2. `requires 'Exporter';` is perl-std Exporter; `requires 'Test::More',
   '0.98'` is the Test-Simple distribution.
