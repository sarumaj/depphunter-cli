---
id: REQ-LANG-025
uuid: 1c51bc1e-0034-47a8-83b4-0d7a4afa7a82
title: Extract and resolve split
scope: lang
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M3
verification:
  - unit
  - inspection
---

## Statement

A plugin **shall** separate extraction, which depends only on a file's content
and extension (or on a declared file class), from resolution, which a per-run
resolver performs from the whole file list and the project's manifests and
layout.

## Rationale

Extraction is the expensive part and can be cached by content; resolution
depends on other files and must be redone on every run.

## Acceptance criteria

1. The plugin interface has an extraction method taking one file and its
   content, and a resolver constructor taking the project's files.
2. A plugin whose extraction depends on more than the extension declares a class
   that becomes part of the cache key.
