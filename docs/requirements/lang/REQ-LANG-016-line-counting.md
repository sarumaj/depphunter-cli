---
id: REQ-LANG-016
uuid: e0ad451d-a537-4f03-8251-ae9a0989c845
title: Line counting
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** count the lines of every text file it reads during
scanning, counting a final line without a trailing newline as a line.

## Rationale

Lines of code are the building height and the stated size of a file.

## Acceptance criteria

1. A file `package a\n\nfunc A() {}\n` has 3 lines.
2. A file `x = 1\ny = 2` without a trailing newline has 2 lines.
