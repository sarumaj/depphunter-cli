---
id: REQ-RUBY-004
uuid: 760a347a-bf44-49e3-ad46-212f07b5e881
title: Ruby standard library island
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A `require` that no project file on the load path answers and that names
Ruby's standard library, a default gem or a bundled gem (for example `json`,
`set`, `net/http`, `yaml`, `digest/sha2`, `minitest/autorun`) **shall**
resolve to the hidden `ruby-std` island, grouped by library (`net/http` is
`net-http`, `yaml` is `psych`, `digest/sha2` is `digest`), unless a project
declares or locks that library as a gem, in which case it **shall** resolve to
the gem.

## Rationale

What ships with Ruby is not a dependency anybody installs; but a default
gem the Gemfile names, or the lock holds, is loaded at the locked version.

## Acceptance criteria

1. `require "set"` is `ruby-std` `set`; `require "digest/sha2"` is `digest`.
2. With psych in Gemfile.lock, `require "yaml"` is the gem psych at its
   locked version.
