---
id: REQ-PHP-011
uuid: 53220415-d70f-4b12-9660-913901d8530e
title: Lock graph for the transitive walk
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** answer `--resolve-depth` from `composer.lock` (or
installed.json): a package depends on the packages its lock entry requires,
without the platform, at their locked versions and pinned, or at the
constraint when the lock lacks them; an answer from installed.json **shall**
be reported as installed.

## Rationale

The lock already records every installed package's requirements.

## Acceptance criteria

1. `guzzlehttp/guzzle` depends on `psr/log` 3.0.0, pinned, and not on `php`
   or `ext-json`.
2. A requirement the lock lacks is returned with its constraint, floating.
