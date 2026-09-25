---
id: REQ-MD-003
uuid: ac0cd2da-1bd1-41c6-85c5-cff857d35fe0
title: Recognized Markdown link forms
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read as links inline links and images
(`[text](dest "title")`, `![alt](dest)`, destinations in angle brackets),
reference definitions (`[label]: dest`), autolinks (`<https://…>`, `<ftp://…>`),
and the `href` and `src` attributes of raw HTML tags; full and collapsed
reference uses (`[text][label]`, `[label][]`) **shall** be read for the
reference check, and the shortcut form `[label]` **shall not**.

## Rationale

All of these render as links; raw HTML is how a README carries its badges. The
shortcut reference form cannot be told apart from bracketed prose.

## Acceptance criteria

1. `<img src="docs/badge.svg">` and `<a href="docs/SPEC.md">` resolve to their
   files.
2. `[spec]: docs/SPEC.md` resolves to `docs/SPEC.md`.
3. `<https://example.test/page>` is captured as a link.
