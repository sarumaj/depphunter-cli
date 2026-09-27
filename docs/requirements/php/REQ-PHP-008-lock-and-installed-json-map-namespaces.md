---
id: REQ-PHP-008
uuid: e699259a-abfc-42a1-b4ab-35e937f8a7d7
title: Namespaces mapped to packages by the lock
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `composer.lock` beside each `composer.json`
(`packages` and `packages-dev`) and, where there is no lock,
`vendor/composer/installed.json` from disk in Composer 2 or Composer 1 form,
and resolve a name to the locked package whose PSR-4 or PSR-0 prefix it falls
under (the longest prefix first, case-insensitively), a function or constant
by its namespace. Packages the lock holds but `composer.json` does not require
**shall** count as declared.

## Rationale

Each package's own autoload rules are in the lock, so the namespace a class
is written in says exactly which package ships it, with no name heuristics.
The vendor directory is not scanned, so installed.json is read directly.

## Acceptance criteria

1. `use Symfony\Component\HttpFoundation\Request` resolves to
   `symfony/http-foundation` by its locked prefix.
2. `use Psr\Log\LoggerInterface` resolves to `psr/log`, which only the
   lock holds.
3. `use Mockery` resolves to `mockery/mockery` by its PSR-0 prefix.
4. Without a lock, `vendor/composer/installed.json` gives the same answers
   with its versions.
