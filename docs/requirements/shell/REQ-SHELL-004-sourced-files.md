---
id: REQ-SHELL-004
uuid: 9e9bdd26-5fd9-4290-8abf-d4a93dd6d152
title: Sourced files
scope: shell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

`source PATH` and `. PATH` **shall** be imports of the project file PATH names,
with PATH evaluated from literals, variables assigned earlier in the file
(also by `local`, `export`, `declare` and `readonly`) and the idioms for the
script's own directory - `$(dirname "$0")`, `$(dirname "${BASH_SOURCE[0]}")`,
`${BASH_SOURCE%/*}`, `$(cd "$(dirname ...)" && pwd)` with any options and
redirections, `realpath`/`readlink -f` of those, zsh's `${0:A:h}`, `${0:h}`,
`$0:A:h` and `${${(%):-%x}:a:h}`, bats' `$BATS_TEST_DIRNAME` - which resolve
relative to the script's directory; `$(git rev-parse --show-toplevel)`, which
resolves relative to the repository root; and `$PWD`, `$(pwd)` and a bare
relative path, which resolve relative to the working directory: first the
script's directory, then the repository root. `${VAR:-default}` evaluates the
default when VAR is unknown. A path under `~` or `$HOME`, an absolute path, a
glob, or one with an unknown part after its start **shall not** be recorded; a
path that names no project file, or the importer itself, **shall** be dropped.

## Rationale

Real scripts almost never source a literal path: they compute their own
directory first, so that they work from any working directory. The working
directory of a script is unknowable from the file; the script's own directory
and the repository root are where scripts are run from in practice.

## Acceptance criteria

1. `source "$SCRIPT_DIR/lib/common.sh"` after
   `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`,
   `. "$(dirname "$0")/lib/log.bash"`, `source "${BASH_SOURCE%/*}/lib/common.sh"`,
   `source "$DIR/lib/common.sh"` after `DIR=$(dirname "$(readlink -f "$0")")`
   and zsh's `source ${0:A:h}/../scripts/lib/common.sh` resolve to the files
   beside or above the script.
2. `$(cd "$(dirname "$0")/.." && pwd)` is the script's parent directory,
   `$(git rev-parse --show-toplevel)/ci/a.sh` is `ci/a.sh` at the root, and
   `source lib/a.sh` is tried beside the script, then at the root.
3. `source ~/.bashrc`, `source "$HOME/x/a.sh"`, `source /etc/profile`,
   `"$(dirname "$0")/lib/$1.sh"` and a glob record nothing.
