---
id: REQ-PHP-006
title: PHP built-ins island
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A global class, function or constant that PHP or one of its extensions
defines, and a class of an extension's namespace (`Random\`, `Dom\`,
`PgSql\`, `MongoDB\Driver\` ...), **shall** resolve to the `php-std`
island (a standard library, hidden by default) under the extension's name in
lower case (`core`, `standard`, `spl`, `date`, `json`, `pdo`, `mbstring`
...). A global name that neither PHP nor the project defines **shall** be
dropped.

## Rationale

The built-ins are the platform, not dependencies anybody installs; grouping
them by extension says which extensions a file needs, as `ext-*` requirements
do. A global function nobody here defines is most likely a framework helper
loaded by a `files` autoload, whose package cannot be told from its name.

## Acceptance criteria

1. `use Exception` resolves to `core`, `new \DateTimeImmutable` to `date`,
   `catch \RuntimeException` to `spl`, `implements \JsonSerializable` to
   `json`, `\strlen()` to `core`.
2. `\collect()` is dropped; `MongoDB\Client` (a Composer package) is not a
   built-in.

## Notes

The tables (internal/lang/php/builtin.go) cover the engine, the bundled
extensions and common PECL ones (redis, memcached, imagick, amqp, swoole,
ds).
