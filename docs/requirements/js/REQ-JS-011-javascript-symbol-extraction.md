---
id: REQ-JS-011
uuid: 1fb5241c-211b-43d6-a7b6-301b8baa7f40
title: JavaScript and TypeScript symbols
scope: js
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M2
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
