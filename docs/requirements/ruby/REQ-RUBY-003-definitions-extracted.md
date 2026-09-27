---
id: REQ-RUBY-003
title: Ruby definitions extracted
scope: ruby
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract modules and classes named with the modules
and classes around them (`Shop::Catalog::Item`, also when written
`class A::B`), methods as `Owner.name` (instance methods, `def self.x`,
methods in `class << self` and in blocks such as a Concern's
`class_methods`), constants as `Owner::NAME`, the attributes of `attr_*` calls
as `Owner.name` of kind `attr`, and top-level methods as functions. A method or
constant inside a method body is not extracted.

## Rationale

The symbols are what the map draws inside a file and what `--lsp` asks
references for.

## Acceptance criteria

1. `lib/shop/catalog.rb` yields `Shop::Catalog::Item` (class),
   `Shop::Catalog::Item.build` and `.import` (methods), `Shop::Catalog::Item::LIMIT`
   (const), `Shop::Catalog::Item.price` (attr), `Shop::Catalog::Price` and
   `helper` (func), and nothing for a local variable.
