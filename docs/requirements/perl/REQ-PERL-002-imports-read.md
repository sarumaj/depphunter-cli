---
id: REQ-PERL-002
title: Imports read
scope: perl
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read as imports: `use Module` and `no Module` (not a
version, `use v5.36`, and not the pragmas that only switch the compiler:
strict, warnings, utf8, feature, lib, vars, integer, bytes and the like),
`require Module` anywhere (not `require 5.006` or a computed argument),
`require "file"` and `do "file"` with a literal or FindBin path, the classes
`use parent` and `use base` name (`-norequire` ones as classes that are not
loaded), Mojo::Base's parent class, `use if`'s and `use aliased`'s module,
Moose's, Moo's and Role::Tiny's `with` and `extends` (in a file that uses
one of them; not role parameters in braces), Corinna's and Object::Pad's
`:isa(...)` and `:does(...)`, Test::More's `use_ok`/`require_ok` and
Module::Runtime's and Class::Load's `use_module`/`load_class` of a literal
name, and the `use`/`require` inside a string `eval "use X; 1"`. Nothing
in POD, here-documents, strings, comments, formats or after
`__END__`/`__DATA__` is read.

## Rationale

Optional dependencies are loaded by `require` inside `eval {}` or a string
eval; roles and parent classes are loaded as surely as `use` loads them.

## Acceptance criteria

1. `use Moose`, `with 'Shop::Role::Priced'`, `extends 'Shop::Base'`,
   `require Try::Tiny` in `eval {}` and `use JSON::XS` in `eval "..."` are
   imports; `use Fake::Heredoc` in a here-document, `use Fake::Pod` in POD and
   `use Fake::End` after `__END__` are not.
2. `with 'NotARole'` before `use Moo` is not an import; after it, `with
   'Role::A', 'Role::B' => { -excludes => 'x' }` imports both roles.
