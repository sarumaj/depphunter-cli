---
id: REQ-CFG-003
uuid: 6765ba21-e5eb-4167-8205-22e89b9d51f9
title: Project config file
scope: cfg
type: interface
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md §5
  - docs/REQUIREMENTS.md M9
verification:
  - unit
---

## Statement

The system **shall** read the project config file from `<path>/.depphunter.yaml`
of the analyzed directory, or from the file named by `--config` instead; a
missing project file **shall** be ignored, and a missing `--config` file
**shall** be an error.

## Rationale

The project file travels with the repository; `--config` lets a user keep that
configuration elsewhere.

## Acceptance criteria

1. Keys in `<path>/.depphunter.yaml` take effect.
2. `--config <file>` reads `<file>` and not `<path>/.depphunter.yaml`.
3. `--config` naming a file that does not exist fails with an error.
4. The project file is also where Save writes (REQ-CFG-012).

## Notes

A `--config` file is the user's own choice and is trusted: it may set the keys a
project file may not (REQ-CFG-010).
