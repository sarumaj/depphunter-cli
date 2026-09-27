---
id: REQ-RUBY-001
title: Ruby files claimed
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Ruby plugin **shall** analyze files that are not binary and end in
`.rb`, `.rake`, `.gemspec` or `.ru` (case-insensitively), or are named
`Gemfile`, `Rakefile`, `Guardfile` or `Capfile`, and label them as Ruby.

## Rationale

Rake tasks, gem specifications and Rack configurations are Ruby, and so are
the files Bundler, Rake, Guard and Capistrano find by name; a Gemfile is where
an application's gems are declared.

## Acceptance criteria

1. `a.rb`, `lib/tasks/db.rake`, `a.gemspec`, `config.ru`, `Gemfile`,
   `Rakefile`, `sub/Guardfile`, `deploy/Capfile` and `B.RB` are claimed;
   `Gemfile.lock`, `gems.locked`, `a.erb`, `gemfile` and `a.rbs` are not.
2. A binary `a.rb` is not claimed.
