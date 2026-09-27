---
id: REQ-PHP-005
uuid: fd3530e4-c5ef-43a5-beb5-6764ba41c2d7
title: Names resolved to project files
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A class, function or constant name **shall** resolve to the project file that
declares it: first by the declarations the project's PHP files make (their
`namespace` and the classes, interfaces, traits and enums at any
indentation; functions and constants at the first column), then by the PSR-4
and PSR-0 rules of every `composer.json` in the project (`autoload` and
`autoload-dev`) and of the path-repository packages its `composer.lock`
records inside the repository. A name under one of the project's namespaces
that no file declares **shall** resolve to that namespace's directory, or to
the directory its autoload rule names.

## Rationale

Composer finds a class by its autoload rules, but a classmap, a `files`
entry, PSR-0 and a project without Composer are only known from what the files
declare; reading the declarations covers them all, and the rules place a class
whose declaration the text scan misses.

## Acceptance criteria

1. `use App\Models\User` with `App\` mapped to `src/` resolves to
   `src/Models/User.php`.
2. `use Legacy_Mailer` with the PSR-0 rule `Legacy_` resolves to
   `lib/Legacy/Mailer.php`; a classmap class resolves to its file.
3. `use function App\Support\format_money` resolves to the `files` entry
   declaring it.
4. `use App\Models` resolves to `src/Models`.
5. A path-repository package's class resolves to its file under
   `packages/`.
