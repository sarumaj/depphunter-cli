---
id: REQ-PUPPET-004
title: Module resolution
scope: puppet
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The first segment of a reference **shall** name the module. A module of the
repository (any `<dir>/<name>` holding `manifests/`, `functions/`,
`types/`, `plans/` or `lib/puppet/`, or a directory whose `metadata.json`
names it) **shall** win, the one containing the file first, else the one
sharing the longest path with it. The reference **shall** link to the file
Puppet's autoloader reads: `x::y::z` to `manifests/y/z.pp`, `x` to
`manifests/init.pp`, a function to `functions/y/z.pp` or
`lib/puppet/functions/x/y/z.rb`, a type alias to `types/y/z.pp`, a
template to `templates/`, a file to `files/`; a missing file to the
module's `init.pp`, its `metadata.json` or its directory. An unqualified
custom type **shall** link to a module's `lib/puppet/type/<name>.rb`, a
well-known module's type (`file_line` is stdlib's) or the module its
prefix names (`mysql_user`); otherwise it **shall** be dropped.

## Rationale

Puppet names classes and files by module and path; control repositories
keep their own modules in `site-modules/`, `site/` and `dist/`.

## Acceptance criteria

1. The fixture's role, profile and widget references link to their files,
   `role::base` (no file) to the module directory, `profile_thing` to its
   Ruby type and `nagios_host` is dropped.
