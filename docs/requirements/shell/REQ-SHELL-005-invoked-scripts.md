---
id: REQ-SHELL-005
title: Scripts a script runs
scope: shell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A command whose name evaluates (as in REQ-SHELL-004) to a path with a directory
part (`./build.sh`, `scripts/x`, `"$DIR/deploy"`) **shall** be an import of that
project file, and so **shall** the script an interpreter is given: the first
argument that is not an option (options that take a value skipped) of `sh`,
`bash`, `zsh`, `dash`, `ksh`, `mksh`, `ash`, `bats`, `python`, `python2`,
`python3`, `python3.x`, `node`, `ruby`, `perl`, `php`, `Rscript` and `pwsh`
(`-File`), by name or absolute path, unless an option makes it read code from
elsewhere (`-c`, `-e`, `-m`, `-s`). `exec`, `command`, `builtin`, `env` (with its
assignments), `sudo`, `doas`, `nohup`, `time`, `nice`, `timeout`, `stdbuf` and,
in bats files, `run` **shall** be looked through to the command they run;
`command -v x` runs nothing.

## Rationale

CI and build pipelines are chains of scripts; the script a job starts runs
others (`./hack/verify.sh`, `python3 tools/gen.py`), and those edges are what
shows which tools the pipeline depends on. A bare command name is looked up on
`PATH`, never in the project.

## Acceptance criteria

1. `"$SCRIPT_DIR/deploy" --dry-run`, `bash scripts/lib/log.bash`,
   `python3 -u "$ROOT_DIR/scripts/gen.py"`, `./scripts/deploy --clean`,
   `exec ./scripts/deploy "$@"` and a bats `run "$BATS_TEST_DIRNAME/../scripts/build.sh"`
   are imports of those files.
