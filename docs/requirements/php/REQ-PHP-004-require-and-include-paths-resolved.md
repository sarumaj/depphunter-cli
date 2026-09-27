---
id: REQ-PHP-004
uuid: e58e7c0a-c20b-4d91-8a44-59b92dba7bd2
title: Require and include paths resolved
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record a `require`, `require_once`, `include` or
`include_once` whose path is made of string literals (without interpolation),
`__DIR__`, `dirname(__FILE__)`, `dirname(__DIR__)`, `dirname(__DIR__, n)` and
`DIRECTORY_SEPARATOR` joined by `.`, and resolve it: a path built on the
file's directory against that directory, any other relative path against the
including file's directory, then the directories of the `composer.json`
files above it, then the repository root. A path it cannot evaluate, an
absolute path, or one that leaves the repository **shall** not become an
edge.

## Rationale

Scripts, templates and legacy code tie files together by path rather than by
class, and PHP looks a relative path up on its include path, which by default
holds the working directory - usually the project root.

## Acceptance criteria

1. `require_once __DIR__ . '/../bootstrap.php'` resolves to `bootstrap.php`
   one directory up.
2. `require dirname(__DIR__, 2) . '/helpers.php'` resolves two directories
   up.
3. `include 'config/app.php'` in `src/Http/Controller/` resolves to the root's
   `config/app.php`.
4. `require $path` and `require __DIR__ . "/$name.php"` are not recorded.
