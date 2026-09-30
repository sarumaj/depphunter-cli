---
id: REQ-CFG-018
title: Project config sets only allow-listed keys
scope: cfg
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** take from the project config file read from the analyzed
directory only the top-level keys on an allow-list of project-settable
settings, and **shall** ignore every other key. Keys **shall** be matched
without regard to case. A setting **shall** be user-only until it is added to
the allow-list; the user config file, `--config`, the environment and flags may
set every setting.

The project-settable keys are `addr`, `open`, `exclude`, `max_file_size`,
`watch`, `cache`, `history`, `history_commits`, `resolve_depth`, `explain`,
`private`, `findings` (confined to paths inside the repository), `vulns`,
`links`, `lsp`, `lsp_timeout` and `ui`. The user-only keys are `editor`
(REQ-CFG-010), `online` (REQ-SUP-020), `python` (REQ-PY-015) and
`trust_indexes` (REQ-SUP-043).

## Rationale

The project file comes with a repository that may be anybody's. With a list of
the keys to drop, a new setting that decides what this machine runs, reads or
reaches would be the repository's to choose unless someone remembered to add
it; with a list of the keys to keep, forgetting leaves it with the user.

Viper reads keys without regard to case, so a rule that compared them exactly
would let `Online: true` through.

## Acceptance criteria

1. Every setting is either on the allow-list or deliberately user-only, and a
   test fails for a setting that is neither.
2. A project file setting `Online`, `EDITOR`, `Python` or `Trust_Indexes` in
   any case changes nothing, and its `Findings` are confined to the repository.
3. A project file's allow-listed keys take effect.
4. The same file named with `--config` sets every key.

## Notes

Before the allow-list, the project file lost `editor`, `online`, `python` and
`trust_indexes` by deletion; the allow-list keeps every other key, so no
setting a repository could make before is refused now.
