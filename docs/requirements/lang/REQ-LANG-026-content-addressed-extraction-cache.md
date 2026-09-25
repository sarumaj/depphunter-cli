---
id: REQ-LANG-026
uuid: 2dab280c-7a85-42ee-8a6c-89b2ecfddc91
title: Content-addressed extraction cache
scope: lang
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M3
verification:
  - unit
---

## Statement

The system **shall** cache each extraction under a key made of the plugin name,
the plugin version, the file's extension (with the plugin's class where
declared) and the SHA-256 of the file's content, and **shall** reuse a cached
extraction only for an identical key.

## Rationale

A content hash recognizes an unchanged file regardless of its timestamp;
including the plugin version invalidates entries when the extraction logic
changes.

## Acceptance criteria

1. Changing a file's content causes it to be parsed again.
2. Increasing a plugin's version causes all its files to be parsed again.
3. An unchanged file is not parsed on the next run.
