---
id: REQ-RUBY-007
uuid: 53b44c82-d1ca-4d5c-8cf4-b02ae9a408df
title: Gemfile and gemspec declarations read
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read every `Gemfile` (or `gems.rb`) and `*.gemspec`
as text: `gem` lines with their requirements (a trailing comma continues a
line), `path:`, `git:`, `github:` and `ref:` options and `path`, `git` and
`github` blocks, `gemspec` directives (with `path:`), `eval_gemfile`, and a
gemspec's `name`, `require_paths` and `add_dependency`,
`add_runtime_dependency` and `add_development_dependency`. A Gemfile's `gem`
lines, its `gemspec` directive and a gemspec's dependency calls **shall** be
imports of what they declare: the gem, the local gemspec (a `path:` gem's
gemspec or directory), so a Rails application's gems, which
`Bundler.require` loads without a `require`, are on the map.

## Rationale

Rails applications seldom require their gems; the Gemfile is the only
place they are named.

## Acceptance criteria

1. The fixture's Gemfile imports rails, pg, puma, sidekiq, devise,
   rspec-rails and acme-auth at their locked versions and
   `gems/billing/billing.gemspec` for `gem "billing", path:`.
2. `gem "rails", "~> 7.1", ">= 7.1.2"` declares the requirement
   `~> 7.1, >= 7.1.2`; gems in a `path "engines" do` block come from
   `path:engines`.
