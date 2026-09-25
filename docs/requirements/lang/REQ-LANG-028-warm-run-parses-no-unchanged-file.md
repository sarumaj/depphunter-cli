---
id: REQ-LANG-028
uuid: 563158df-2880-4b68-ae8c-2261dfffc710
title: Warm run parses no unchanged file
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** persist the extraction cache between runs of the command,
and a run over a project whose files did not change since the previous run
**shall** parse no file.

## Rationale

This is what makes a repeated run, and every re-analysis in watch mode,
near-instant.

## Acceptance criteria

1. A second run over an unchanged project reports 0 parsed and all claimed files
   cached.
2. The graph of the second run equals that of the first apart from
   `generatedAt`.
3. After editing one file, exactly that file is parsed.

## Notes

The cache is stored under the user cache directory; `--no-cache` disables it
(scope `cli`). A cache that cannot be read is treated as empty.
