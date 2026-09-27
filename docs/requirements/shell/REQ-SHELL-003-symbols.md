---
id: REQ-SHELL-003
uuid: 83419de4-d84f-46ac-b90f-d98985cad971
title: Shell functions, aliases and variables as symbols
scope: shell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record as symbols every function definition (`name() {`,
`name() (`, `function name {`, `function name() {`, at any depth; kind
`function`), every bats test (`@test "name" {`; kind `test`), every alias an
`alias` command defines outside a substitution (kind `alias`), and, at the top
level of the script only (outside function bodies and substitutions),
variables it exports (`export`, `declare -x`, `typeset -x`; kind `var`),
read-only variables (`readonly`, `declare -r`; kind `const`) and variables with
an upper-case name it assigns (kind `var`, `IFS` excepted). A name is recorded
once, at its first definition.

## Rationale

Lower-case assignments in a script are mostly loop and scratch variables;
upper-case and exported ones are the configuration a script shares with the
scripts that source it. Local variables are never visible to other files.

## Acceptance criteria

1. The fixture's build script has `SCRIPT_DIR`, `ROOT_DIR` and `BUILD_MODE`
   (var), `VERSION` (const, readonly), `build` (line 19) and `clean` (line 32,
   `function clean {`) and not `tmp_dir` or the function's `local out`; the
   bats file has `setup` and the test `build prints usage`; the zsh plugin has
   the aliases `G` and `gs` from `alias -g G='| grep' gs='git status'` and
   `EDITOR` from `typeset -gx`.
