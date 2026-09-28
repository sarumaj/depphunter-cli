---
id: REQ-FND-005
title: Trivy vulnerabilities, misconfigurations and secrets
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read Trivy JSON reports, including vulnerabilities,
misconfigurations and secret hits; a secret finding **shall not** carry the
secret value. A vulnerability **shall** be placed on the island of the
ecosystem Trivy's own vulnerability driver asks for its package type: npm for
`npm`, `yarn`, `pnpm`, `bun`, `node-pkg` and `javascript`; PyPI for `pip`,
`pipenv`, `poetry`, `uv`, `pylock` and `python-pkg`; Maven for `pom`,
`gradle`, `sbt` and `jar`; NuGet for `nuget`, `dotnet-core` and
`packages-props`; crates.io for `cargo` and `rustbinary`; Go for `gomod` and
`gobinary`; and Composer, RubyGems, CocoaPods, Swift packages, pub, Hex,
Conan and Julia for their types. A type with no island here (conda, Bitnami,
an operating system's packages) leaves the finding on the scanned file.

## Rationale

Trivy covers packages, infrastructure files and leaked secrets in one report.

## Acceptance criteria

1. A Trivy report with the three result kinds yields vulnerability findings on
   packages and lint findings on the target files with their lines.
2. The detail of a secret finding does not contain the matched value.
3. Each of Trivy's language package types lands on its ecosystem's island;
   `conda-pkg`, `bitnami` and an OS type land on none.

## Notes

The type names are Trivy's `LangType` constants (pkg/fanal/types/const.go) and
the mapping follows its `pkg/detector/library/driver.go`.
