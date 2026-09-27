---
id: REQ-RUBY-011
uuid: b091591a-a892-42b9-9f73-eb045dba62ef
title: Gemfile.lock graph for the transitive walk
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Ruby resolver **shall** answer `--resolve-depth` from Gemfile.lock: a
locked gem's dependencies, each at its locked version and pinned, its
requirement kept as requested when it is not the version; a dependency the
lock does not hold is its requirement.

## Rationale

The lock already records the whole graph; reading it needs no network.

## Acceptance criteria

1. railties 7.1.2 depends on actionpack 7.1.2, activesupport 7.1.2 and psych
   5.1.1; nokogiri on racc 1.7.3 requested `~> 1.4`.
