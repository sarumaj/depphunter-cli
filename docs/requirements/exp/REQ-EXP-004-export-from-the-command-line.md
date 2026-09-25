---
id: REQ-EXP-004
uuid: 943e4d82-619b-4c61-91b3-96bfb1e77f96
title: Export from the command line
scope: exp
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

With `--export json|graphml|dot|html` the system **shall** write the export to
the file named by `-o`/`--output`, or to standard output when none is given, and
exit without serving; an unknown format **shall** be rejected.

## Rationale

Exports are used in scripts and CI where no browser is available.

## Acceptance criteria

1. `depphunter --export json -o g.json <dir>` writes a graph whose edges include
   the Go standard-library import.
2. `--export json` without `-o` writes only the document to standard output.
3. An unknown format fails with an error naming the valid formats.
