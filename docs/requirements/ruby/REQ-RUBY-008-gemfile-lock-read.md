---
id: REQ-RUBY-008
uuid: dd9c8638-dd51-4e68-89be-21de368a9354
title: Gemfile.lock read
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `Gemfile.lock` (`gems.locked` beside `gems.rb`):
the specs of its GEM, GIT and PATH sections with their dependencies, a GIT
section's remote and revision, a PATH section's directory, and its
DEPENDENCIES. A gem locked for several platforms is one gem at one version,
the platform suffix dropped. The gems the lock holds count as declared.

## Rationale

The lock names every gem installed, direct or not, at its exact version.

## Acceptance criteria

1. `nokogiri (1.15.4-x86_64-linux)` and `(1.15.4-arm64-darwin)` are one
   nokogiri 1.15.4; devise from a GIT section carries its remote as origin.
