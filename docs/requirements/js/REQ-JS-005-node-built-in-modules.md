---
id: REQ-JS-005
uuid: af83e9aa-3ec3-44ce-8750-b5cabd3eaf16
title: Node.js built-in modules
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve a `node:`-prefixed specifier, and a bare specifier
naming a Node.js built-in module, to a package of the ecosystem `node` ("Node.js
built-ins"), declared as a standard library.

## Rationale

Built-ins are the JavaScript standard library and are not installed from npm.

## Acceptance criteria

1. `node:fs` and `fs` both resolve to the package `fs` in the ecosystem `node`.
2. `path` resolves to `node` package `path`.
