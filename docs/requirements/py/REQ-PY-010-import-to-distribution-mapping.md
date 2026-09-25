---
id: REQ-PY-010
uuid: 744e0fd1-98a7-4c04-a716-d37bedaca916
title: Import to distribution mapping
scope: py
type: functional
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M2
verification:
  - unit
---

## Statement

The plugin **shall** map a top-level import name to a declared distribution by
trying, in order, the well-known alias of the import (for example `yaml` to
`PyYAML`, `bs4` to `beautifulsoup4`, `sklearn` to `scikit-learn`, also for
two-segment names such as `google.protobuf`), the name itself, `python-<name>`,
`py<name>` and `<name>-python`, comparing names after PEP 503 normalization, and
**shall** mark an import matching no declared distribution as unresolved.

## Rationale

Import names and distribution names differ for many popular packages.

## Acceptance criteria

1. `import yaml` resolves to the distribution `PyYAML`.
2. `from bs4 import BeautifulSoup` resolves to `beautifulsoup4`.
3. `import notdeclared.sub` resolves to `notdeclared`, unresolved.
