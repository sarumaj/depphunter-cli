---
id: REQ-RUBY-005
uuid: d48aecda-5e1e-450e-9611-aec269bdc374
title: Requires resolved on a guessed load path
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve a `require` path to a project file on a
guessed `$LOAD_PATH`: the `lib`, `test` and `spec` directories of the file's
directory and of every directory above it, the require paths of every
gemspec in the repository (`lib` unless `require_paths` says otherwise), the
`lib` of path gems (`gem "x", path:` and the lock's PATH sections), and in a
Rails application its `app/*` directories. A `load` **shall** be looked up
relative to the file, the project's directory, the load path and the
repository root, and never reach a gem.

## Rationale

Bundler, Rake and RSpec put these directories on the load path, and a
repository that builds several gems requires them by name.

## Acceptance criteria

1. `require "billing/invoice"` in `gems/billing/lib/billing.rb` resolves to
   `gems/billing/lib/billing/invoice.rb`; `require "test_helper"` in
   `gems/billing/test/` to its `test_helper.rb`; `load "lib/tasks/seed.rake"`
   in the Rakefile to that file.
