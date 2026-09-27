---
id: REQ-RUBY-012
title: Ruby read without running Bundler
scope: ruby
type: limitation
priority: must
status: implemented
verification:
  - manual
---

## Statement

The plugin **shall not** run Ruby or Bundler: `$LOAD_PATH` changes made at
run time, requires of computed paths (`Dir.glob(...).each { require }`), a
Gemfile's conditionals and platforms and Zeitwerk inflections and custom
autoload paths are not evaluated; constants are resolved only in Rails
applications (a gem's own Zeitwerk loader is not read), a constant is not
attributed to a gem, and a require path of an undeclared gem is named by
heuristics.

## Rationale

depphunter reads repositories statically and never executes their code.

## Acceptance criteria

1. `require some_variable` is not recorded, and `ActiveRecord::Base` is not
   an edge to activerecord.
