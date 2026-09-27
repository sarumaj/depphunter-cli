---
id: REQ-PHP-012
title: PHP read without running Composer
scope: php
type: limitation
priority: must
status: implemented
verification:
  - manual
---

## Statement

The plugin **shall not** run Composer or PHP: autoloaders, `files` entries
and include paths configured at run time are not evaluated, a global function
from a package's `files` autoload is not attributed to it, a package that
autoloads only by classmap is matched by name (REQ-PHP-010), a relative
`require` is resolved against the file's directory and the project roots
rather than the working directory the script will run in, and dynamic paths
and class names (`new $class`) are not followed.

## Rationale

depphunter reads repositories statically and never executes their code.

## Acceptance criteria

1. `\collect()` (a Laravel helper) is dropped, and `require $path` is not
   recorded.
