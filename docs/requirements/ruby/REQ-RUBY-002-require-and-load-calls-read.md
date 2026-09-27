---
id: REQ-RUBY-002
uuid: bfb04cad-c435-40ff-bd88-aac7131f0812
title: Require, load and autoload calls read
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record `require`, `require_dependency`,
`require_relative`, `load` and `autoload` (with or without a receiver) whose
path the file spells out: string literals without interpolation,
`__dir__`, `File.dirname(__FILE__)` (also interpolated at the start of a
string), `File.expand_path(path, base)`, `File.join` and `+`. A path relative
to the file resolves against its directory; `require_relative` and `autoload`
resolve to the project file, `.rb` added.

## Rationale

Ruby code names what it loads by path, and much of it builds the path from
the file's own location; a `require` of a variable cannot be known without
running the code.

## Acceptance criteria

1. `require File.expand_path("../config/environment", __dir__)` in
   `spec/rails_helper.rb` resolves to `config/environment.rb`;
   `autoload :Tax, "billing/tax"` to the file on the gem's `lib`.
2. `require some_variable` is not recorded; a `require_relative` of a
   missing file is dropped.
