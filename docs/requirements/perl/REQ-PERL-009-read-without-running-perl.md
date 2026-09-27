---
id: REQ-PERL-009
title: Perl read without running perl
scope: perl
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Perl is read without running perl, cpanm or Carton: `@INC` changed at run
time (`unshift @INC`, `PERL5LIB`), `use lib` of other variables, modules
loaded by computed names, and code in `BEGIN` blocks or source filters are
not followed; a module's distribution without a snapshot is named by a
curated table and its name, so a module shipped in a differently named
distribution that the project does not declare gets a wrong name; `--online`
reads the latest release's dependencies whatever version is pinned, and no
vulnerability database covers CPAN.

## Rationale

Only perl can parse Perl; the map reads what is written. OSV's ecosystem
list has no CPAN entry.

## Acceptance criteria

1. `use lib $dir` adds no directory; a `require $class` is not an import.
