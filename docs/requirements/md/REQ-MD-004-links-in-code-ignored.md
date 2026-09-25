---
id: REQ-MD-004
uuid: f743ebea-218a-4d56-925f-8875e54f93dd
title: Links in code are not followed
scope: md
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M19
verification:
  - unit
---

## Statement

The system **shall not** treat a link inside fenced code (```` ``` ```` or `~~~`
fences, closed by a fence of the same character at least as long) or inside a
code span as an edge or as a candidate for a broken-link finding, and
**shall not** read headings inside fenced code.

## Rationale

A link inside code is printed rather than followed; checking it would report a
defect in an example.

## Acceptance criteria

1. A link inside a fenced block is neither an edge nor a finding.
2. A link inside a code span is neither an edge nor a finding.
