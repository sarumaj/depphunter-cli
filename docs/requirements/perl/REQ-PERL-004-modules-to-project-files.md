---
id: REQ-PERL-004
title: Modules to project files
scope: perl
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A module **shall** resolve to the project file `Foo/Bar.pm` for `Foo::Bar`
under, in order: the file's `use lib` directories - literal
(relative to the distribution's root, the repository's and the file's
directory), or computed from `$FindBin::Bin`/`$FindBin::RealBin`,
`__FILE__`, `dirname()`, File::Spec's `catdir`/`catfile`, Mojo::File's
`curfile->dirname->sibling(...)` and Path::Tiny's `path(__FILE__)->parent->child(...)`
chains, and `lib::relative` - then the nearest distribution's `lib/`,
root and `t/lib`, the `lib/` of every directory above the file and the
repository's `lib/` and root. After core and declared modules, it
**shall** resolve to the only project `.pm` file ending in the module's path
(two segments or more), else the only file declaring the package. A class
named without loading it (`use parent -norequire`) resolves to the file
declaring the package first; one the file declares itself, or a module of
the project's own distribution missing from the checkout, is dropped.
`require`/`do` of a path resolves relative to the file when computed from
it, else under the same directories.

## Rationale

Perl finds modules on `@INC`, which tests and scripts extend with `use
lib`; the conventional roots cover the rest without running perl.

## Acceptance criteria

1. `use lib "$FindBin::Bin/testlib"` finds `t/testlib/TestHelper.pm` and
   `use lib 'inc'` finds `inc/Shop/Inc.pm`, though another file of each name
   exists elsewhere.
2. `use lib curfile->dirname->sibling('modules')` and `use lib
   File::Spec->catdir(dirname(__FILE__), '..', 'modules')` find
   `examples/tool/modules/Tool.pm`.
3. `use parent -norequire, 'Shop::Base'` resolves to `lib/Shop/Base.pm`;
   `use Acme::Widget::Util` in the Acme-Widget distribution is dropped.
