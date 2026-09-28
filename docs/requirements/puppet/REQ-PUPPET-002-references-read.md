---
id: REQ-PUPPET-002
title: References read
scope: puppet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A manifest's references **shall** be read: `include`, `require` and
`contain` (bare words, strings, lists, parentheses), `class { 'x': }`,
`inherits`, `Class['x']`, declarations, references, defaults and
collectors of defined and custom types (`x::y { 't': }`, `X::Y['t']`,
`Concat { }`, `X::Y <| |>`, `@` and `@@`), namespaced function calls
(`x::y()`) and puppetlabs-stdlib's unnamespaced ones, namespaced data types
(`Stdlib::Port`), `template()`, `epp()`, `file()` and
`puppet:///modules/` sources. Puppet's own resource types, data types and
functions **shall** not be references, and nothing in comments, strings,
heredocs (`@(END)`) or regular expressions **shall** be read. A Free Pascal
`.pp` unit **shall** yield nothing.

## Rationale

A class's dependencies are the classes it includes, the types it declares
and the functions, types and files it uses; Puppet's built-ins are no
dependency.

## Acceptance criteria

1. `profile/manifests/web.pp` yields each form once and nothing from its
   comments, double-quoted string, heredoc, regular expression or core
   types; the lexer test reads only the two real includes.
