---
id: REQ-REGO-007
title: Read without OPA
scope: rego
type: limitation
priority: should
status: implemented
verification:
  - unit
---

## Statement

Rego **shall** be read without OPA: data documents from JSON or YAML
files, bundles and Styra DAS or Conftest pulls are not linked, a
reference through a variable (`data[x]`) is not followed, and rule
bodies are not evaluated.

## Rationale

What `data` holds is decided when OPA loads it.

## Acceptance criteria

1. `data.inventory.hosts` vanishes and `import data.external.inventory` is
   dropped.
