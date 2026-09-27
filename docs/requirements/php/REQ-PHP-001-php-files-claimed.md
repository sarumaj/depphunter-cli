---
id: REQ-PHP-001
title: PHP files claimed
scope: php
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The PHP plugin **shall** analyze files ending in `.php`, `.phtml` and `.inc`
(case-insensitively) that are not binary, and label `.php` and `.phtml` files
as PHP.

## Rationale

`.phtml` is PHP embedded in markup and is read the same way. `.inc` is the
traditional extension of PHP include files; other tools use it too, but a file
without a `<?php` tag parses as inline text and yields nothing.

## Acceptance criteria

1. `a.php`, `b.PHTML` and `c.inc` are claimed; `d.phps` and `e.js` are not.
2. A binary `x.php` is not claimed.
