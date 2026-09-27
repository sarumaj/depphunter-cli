---
id: REQ-SUP-045
title: RubyGems dependencies from the compact index
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a gem's runtime dependencies from the compact
index of its gem server (`<index>/info/<name>`, `https://rubygems.org` unless
configured otherwise): the line of the version asked for, the
platform-independent one first, or else the newest release that is neither a
pre-release nor built for one platform, each dependency with its requirements
joined by a comma and an exact `= 1.2.3` given as `1.2.3`.

## Rationale

A gem library commits no lock, so what its gems depend on is only on the
index; the compact index is what Bundler itself reads, and every gem server
Bundler is pointed at serves it.

## Acceptance criteria

1. Against a stub server, `sinatra` 3.0.0 depends on `rack` `~> 2.2, >= 2.2.4`
   and `tilt` `~> 2.0`.
2. A requirement that names no version returns the newest release's
   dependencies, not a pre-release's or a `-java` build's, and `= 4.0.0` is
   pinned 4.0.0.
