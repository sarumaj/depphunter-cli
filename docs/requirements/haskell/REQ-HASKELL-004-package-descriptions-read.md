---
id: REQ-HASKELL-004
title: Package descriptions read
scope: haskell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read every `.cabal` file of the repository - name,
library, executable, test-suite, benchmark and foreign-library components with
their `hs-source-dirs` (the package directory when none), modules and
`build-depends`, conditional blocks included, common stanzas merged into the
components that `import:` them, and `custom-setup`'s `setup-depends` - and
hpack's `package.yaml` where no `.cabal` file sits beside it (top-level and
per-component `source-dirs` and `dependencies` as a list or a map, `when:`
conditionals, `custom-setup`).

## Rationale

Which packages a module may import, and where a component's modules are, is
said per component; stack projects commit the `.cabal` file hpack generates,
which is the same package.

## Acceptance criteria

1. `shop-core`'s library gets `containers` from its common stanza; its test
   suite depends on `hspec` and `QuickCheck == 2.14.3`.
2. `stackproj/package.yaml`'s executable depends on `filepath >= 1.4` and on
   the top-level `text`.
