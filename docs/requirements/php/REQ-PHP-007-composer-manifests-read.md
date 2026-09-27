---
id: REQ-PHP-007
uuid: 4d56cbe0-41d9-4f27-9cd7-5c4503ce003d
title: Composer manifests read
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read every `composer.json` in the project for its
`require` and `require-dev` packages and its autoload rules, leaving out the
platform requirements (`php`, `ext-*`, `lib-*`, `composer-plugin-api` and any
other name without a `/`). A file **shall** take its packages from the
`composer.json` files in its directory and above it, nearest first, or from
every project when none is above it.

## Rationale

composer.json says which packages a project asks for and with which
constraints; the platform requirements are not packages anybody downloads.

## Acceptance criteria

1. `php` and `ext-json` in `require` do not become packages.
2. A nested `composer.json` without a lock takes locked packages from the
   project above it.
