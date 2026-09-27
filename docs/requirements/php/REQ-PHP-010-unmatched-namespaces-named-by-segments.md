---
id: REQ-PHP-010
title: Unmatched namespaces named by their segments
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A namespaced name that is not a project file, a built-in or under a locked
package's prefix **shall** resolve to a package the project declares whose
vendor and name are the first two segments (case and punctuation aside), whose
name is the first segment (`PHPUnit` is `phpunit/phpunit`), the only package
of a vendor named like the first segment, or the vendor's package named by the
following segments together (`Psr\Http\Message` is `psr/http-message`);
otherwise to the package the first two segments name (the first lower-cased,
the second hyphenated at its word boundaries; Symfony's `Component\X`,
`Bundle\X` and `Bridge\X` as `symfony/x`, `symfony/x` and
`symfony/x-bridge`), marked unresolved unless the project requires it.

## Rationale

A library commits no lock, and some packages autoload by classmap only, so
the lock's prefixes cannot always answer; package names usually follow the
namespace.

## Acceptance criteria

1. `use PHPUnit\Framework\TestCase` with phpunit autoloaded by classmap
   resolves to `phpunit/phpunit`.
2. `use Symfony\Component\Console\Command\Command`, neither installed nor
   required, is `symfony/console`, unresolved.
3. `use Carbon\Carbon` undeclared is `carbon/carbon`, unresolved.
