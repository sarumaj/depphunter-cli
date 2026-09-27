---
id: REQ-RUBY-010
uuid: e04dbb5c-9fa4-4f22-9a8c-0e3ee5993f79
title: Rails constants resolved by Zeitwerk naming
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

In a Rails application (a directory with `config/application.rb`) the
plugin **shall** resolve the constants code names - receivers,
superclasses, arguments (`include Trackable`), rescued exceptions and
assigned values - to the file Zeitwerk would autoload them from: every
directory under `app/` and `app/*/concerns` is a root (and `lib/` when the
application calls `autoload_lib` or names `lib` in `autoload_paths`), a file's
path under its root camelized is its constant, compared without case and
underscores, and a directory without a file is a namespace. The modules
around the reference are tried innermost first. A constant nothing autoloads
**shall** be dropped.

## Rationale

A Rails application loads its own classes by name, not by `require`;
without this its models, controllers and concerns would stand unconnected.

## Acceptance criteria

1. In `app/controllers/admin/users_controller.rb`, `Role` inside
   `module Admin` resolves to `app/models/admin/role.rb`; `User` to
   `app/models/user.rb`; `ActiveRecord::RecordNotFound` is dropped.
2. `Shop::MoneyFormat` resolves to `lib/shop/money_format.rb` under `autoload_lib`.
