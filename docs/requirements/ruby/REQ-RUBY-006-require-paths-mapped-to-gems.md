---
id: REQ-RUBY-006
uuid: 025c2ddc-f90c-4d31-b24b-397715983445
title: Require paths mapped to gems
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A `require` that is neither a project file nor Ruby's own **shall**
resolve to a gem: an alias for paths not named after their gem
(`active_support` is `activesupport`, `action_controller` is `actionpack`,
`rails` is `railties`, `concurrent` is `concurrent-ruby`, `zip` is `rubyzip`),
else the declared or locked gem named by the path's leading segments joined
with dashes or run together, longest first (`rspec/core` is `rspec-core`), case,
dashes and underscores ignored, or a gem named like its first segment with
`ruby` before or after it (`yajl` is `yajl-ruby`). A path nothing declares
**shall** be shown as an unresolved gem named after its first segment (the
first two under `net/` and `dry/`). A gem the repository builds itself (a
gemspec in it) **shall** resolve to that gemspec.

## Rationale

A gem may install any number of require paths; the lock names gems, not
paths, so the name has to be derived.

## Acceptance criteria

1. `require "rails/all"` is railties and `require "action_controller/railtie"`
   actionpack at their locked versions; `require "rspec/core"` is
   `rspec-core`; `require "bootsnap/setup"` with bootsnap undeclared is an
   unresolved `bootsnap`.
