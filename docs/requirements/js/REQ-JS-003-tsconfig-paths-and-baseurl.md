---
id: REQ-JS-003
uuid: 2f330a41-2551-436c-84bf-cc3deb123e75
title: tsconfig and jsconfig paths
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve non-relative specifiers through the
`compilerOptions.paths` (longest matching prefix, one `*` wildcard, targets
tried in order) and `compilerOptions.baseUrl` of the nearest `tsconfig.json` or,
where none is in the same directory, `jsconfig.json`, including settings
inherited through relative `extends`, and **shall** accept comments and trailing
commas in these files.

## Rationale

Path aliases are how most TypeScript projects import their own modules; without
them those imports would appear as npm packages.

## Acceptance criteria

1. `@app/lib/math` with `paths: {"@app/*": ["src/*"]}` resolves to
   `src/lib/math.ts`.
2. A `baseUrl` inherited from a relatively extended config applies.
3. A config containing comments is read.

## Notes

Configurations extended from packages (non-relative `extends`) are ignored.
`jsconfig.json` has no dedicated test.
