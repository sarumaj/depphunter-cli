---
id: REQ-PERL-006
uuid: 812ab48d-388e-4152-83be-6dfef6559eed
title: Manifests read
scope: perl
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read requirements from `cpanfile` (`requires`,
`recommends`, `suggests`, `conflicts`, the `test_`/`build_`/`configure_`/
`author_requires` forms, `on 'phase' => sub { }` and `feature` blocks, cpm's
and Carmel's `git =>`/`url =>`/`dist =>` sources), `META.json`/`MYMETA.json`
(version 2 `prereqs`, version 1 keys), `META.yml`/`MYMETA.yml` (versions as
written), `Makefile.PL` (WriteMakefile's literal `PREREQ_PM`,
`BUILD_REQUIRES`, `TEST_REQUIRES`, `CONFIGURE_REQUIRES` hashes and a
META_MERGE's `prereqs`; Module::Install's cpanfile-like statements),
`Build.PL` (`requires`, `recommends`, `build_requires`, `test_requires`,
`configure_requires`) and `dist.ini` (`[Prereqs]` and `[Prereqs / Label]`
sections, the label or `-phase`/`-relationship` giving phase and relation).
Requirements and recommendations of every phase **shall** be imports of the
module's distribution (not `perl`, suggestions or conflicts); the
distribution's name is read from `NAME`/`DISTNAME`, `module_name`/`dist_name`,
Module::Install's `name`, META's and dist.ini's `name`.

## Rationale

A library's manifests say what it needs even where the code loads it
conditionally; applications declare in cpanfile what Carton installs.

## Acceptance criteria

1. The cpanfile's `requires 'LWP::UserAgent', '== 6.72'` is an import of
   libwww-perl 6.72 (pinned); `on 'test'` requirements are imports with spec
   `test requires ...`; `git =>` becomes the package's origin.
2. `dist/Makefile.PL`, `dist/META.json` and `dist/dist.ini` give Moo,
   JSON-PP `>= 4.00`, Test-Fatal and Cpanel-JSON-XS; dist.ini's `; authordep`
   and plugin bundles are not read.
