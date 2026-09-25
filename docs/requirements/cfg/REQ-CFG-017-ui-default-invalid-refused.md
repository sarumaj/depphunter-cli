---
id: REQ-CFG-017
uuid: 266eccb1-2e28-480d-829a-469d106c4135
title: Unparseable --ui-default refused
scope: cfg
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M30
verification:
  - unit
---

## Statement

The system **shall** fail with an error for a `--ui-default` argument that is
not `key=value`, names a key that cannot be seeded, or has a value that cannot
be parsed for its key.

## Rationale

A seed is set in an editor's settings and never looked at again; dropping it
silently would leave somebody wondering why their view is not what they asked
for.

## Acceptance criteria

1. Each of `theme`, `=dark`, `hide_languages=go`, `show_std=maybe` and
   `expand_depth=deep` is refused with an error.

## Notes

A well-formed seed with a value outside the allowed set (for example
`theme=neon`) fails validation only when it is the effective value; when a
config file overrides it, it is not reported.
