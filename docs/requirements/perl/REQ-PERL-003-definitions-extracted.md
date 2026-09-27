---
id: REQ-PERL-003
uuid: 476f7f11-1ae8-42ca-9eb6-484ce59c1b6f
title: Definitions extracted
scope: perl
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as symbols: packages (`package X;`,
`package X { }`) and Corinna/Object::Pad classes and roles (kind class),
named subs (`Package.name`; kind method when their signature or first
lines take `$self`, `$class` or the like, else function; forward
declarations `sub name;` are not definitions), Corinna's `method` (method),
`use constant`'s names (const) and the attributes `has` declares in a file
using Moose, Moo, Mouse or Mojo::Base (property; a qw list or an array of
names, `'+name'`).

## Rationale

Packages and subs are Perl's units of reuse; Moose attributes and constants
are the rest of a class's interface.

## Acceptance criteria

1. `lib/Shop.pm` gives `Shop` (class), `Shop.TAX`, `Shop.CURRENCY`,
   `Shop.LIMIT` (const), `Shop.name`, `Shop.price`, `Shop.qty`, `Shop.id`
   (property), `Shop.total` (method) and `Shop._helper` (function).
2. `sub mymax(\\@;$);` is not a symbol; `package Shop::Cart::Item { sub price
   }` gives `Shop::Cart::Item.price`, and the sub after the block is
   `Shop::Cart`'s again.
