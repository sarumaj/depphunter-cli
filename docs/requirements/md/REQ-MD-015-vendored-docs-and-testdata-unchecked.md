---
id: REQ-MD-015
uuid: b436c774-50f2-4c93-a884-c4c3d81a1e80
title: Vendored docs and testdata not checked
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall not** check the links of Markdown files located, at any
depth, under a directory named `vendor`, `node_modules`, `third_party`,
`thirdparty`, `site-packages`, `.venv`, `venv` or `testdata`.

## Rationale

Vendored documentation links to parts of its own repository that vendoring does
not copy, and fixtures under `testdata` are wrong on purpose; a finding against
either would be a defect nobody is expected to correct.

## Acceptance criteria

1. A run over a repository that vendors its dependencies reports nothing from
   their READMEs.
2. A broken link in `internal/x/testdata/README.md` is not reported.

## Notes

`TestVendoredDocsAndTestdataAreNotLinkChecked` covers this selection
(`documents` and `fixed` in `cmd/depphunter/main.go`).
