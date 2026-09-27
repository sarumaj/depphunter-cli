---
id: REQ-JS-011
title: JavaScript and TypeScript symbols
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** extract top-level functions and generator functions
(`func`), classes and abstract classes (`class`), variables (`var`, or `func`
when initialized with a function or arrow function), TypeScript interfaces
(`interface`), type aliases (`type`) and enums (`enum`), each also when
exported, and class methods (`method`, named `<Class>.<method>`).

## Rationale

These are the declarations a reader navigates by in JavaScript and TypeScript.

## Acceptance criteria

1. `src/index.ts` yields `main` (func), `App` (class), `App.render` (method),
   `Props` (interface), `T` (type), `E` (enum), `handler` (func) and `X` (var).
2. `legacy/app.js` yields `Legacy.start` (method).
