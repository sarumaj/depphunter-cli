---
id: REQ-SHELL-010
title: Shell scripts read without running them
scope: shell
type: limitation
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall not** run a script or a shell. Scripts are read by a scanner
of the plugin's own; only paths the file itself determines are followed (not
loops over globs such as `for f in "$DIR"/lib/*.sh; do source "$f"`, `eval`,
`bash -c` strings, arrays, or variables assigned in another file or inside a
function called later), the working directory is guessed (REQ-SHELL-004), the
functions a sourced file defines are not linked to their calls, and system
package managers (`apt-get`, `apt`, `apk`, `brew`, `dnf`, `yum`, `zypper`,
`pacman`) are not read, as no ecosystem of the map holds their packages. zsh's
`{ cmd }` without a separator before the brace is not recognized as closing
the group.

## Rationale

The vendored tree-sitter bash grammar was measured on shallow clones of
nvm-sh/nvm, bats-core/bats-core, dokku/dokku and ohmyzsh/ohmyzsh: 2.7 to 3.6 ms
per file, and ERROR nodes in 269 of ohmyzsh's 414 zsh files (65%), 10 of
bats-core's 304 (its `@test` syntax) and 5 of dokku's 765. Everything the
plugin needs is at the level of words and simple commands, which a scanner
reads in 0.07 to 0.15 ms per file (nvm's 352 scripts in 41 ms, ohmyzsh's 556 in
60 ms), zsh included.

## Acceptance criteria

1. `apt-get install -y jq` and `brew install shellcheck` record nothing; a
   `source` inside a here-document or a string is not an import; a path under
   `~` or `$HOME` or computed from an argument is not recorded.
