---
id: REQ-RUBY-009
uuid: 631376a4-e624-40de-b570-aca2b621ae3a
title: RubyGems pinning rule
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A gem **shall** be pinned when the lock records its version (a GIT gem by
a commit revision), or, without a lock entry, when its requirement names one
version: `1.2.3` or `= 1.2.3` (also a pre-release such as `7.1.0.rc1`), the
version then shown bare. `~>`, `>=`, `>`, `<`, `!=` and several requirements
together float, and a gem declared without any version floats. With a lock,
the Gemfile's requirement is shown as requested when it is not the version.

## Rationale

RubyGems reads a bare version as an exact requirement, as Composer does
and npm does not.

## Acceptance criteria

1. `pinned("= 1.2.3")` and `pinned("7.1.0.rc1")` hold; `~> 1.2`,
   `>= 1.0, < 2` and `1.2.x` do not.
2. `gem "rails", "~> 7.1.0"` locked to 7.1.2 is pinned 7.1.2, requested `~> 7.1.0`.
