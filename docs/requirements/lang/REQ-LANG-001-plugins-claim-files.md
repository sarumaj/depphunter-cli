---
id: REQ-LANG-001
uuid: 408ceb7d-79f9-4429-ab98-66dd7411380a
title: Plugins claim files
scope: lang
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §4
verification:
  - unit
  - inspection
---

## Statement

Each language ecosystem **shall** be implemented as a plugin that decides, by
the file's extension or name, which scanned files it analyzes (claims), and the
analysis **shall** pass to a plugin only the files it claims.

## Rationale

Claiming by name keeps the plugins independent of one another and lets any
ecosystem be added without changes to the pipeline.

## Acceptance criteria

1. Every plugin exposes a claim decision taking a scanned file.
2. A plugin's extraction is called only for files it claims.
3. A file no plugin claims is not parsed.
