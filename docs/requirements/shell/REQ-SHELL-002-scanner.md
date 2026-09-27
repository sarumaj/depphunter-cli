---
id: REQ-SHELL-002
title: Shell scripts read as the shell reads them
scope: shell
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read a script as a sequence of simple commands the way the
shell splits them: words with single, double and ANSI-C quoting and backslash
escapes, parameter expansions, command substitutions (`$(...)` and backquotes,
whose commands are read as well), arithmetic, process substitutions, arrays
and glob groups; commands separated by newlines and control operators;
redirections and their targets left out of the words; here-document bodies
skipped; `#` comments only at the start of a word; `case` patterns, `[[ ]]`,
`(( ))` and `for` headers not taken for commands. It **shall** know which
commands run inside a function body or a substitution, and recover at the next
command from text it does not understand.

## Rationale

Every dependency of a script is a command: `source x`, `./x.sh`,
`pip install x`. Text that only looks like one - a here-document written to a
file, an `echo "source x"`, a comment - must not become an edge, and the
unmatched `)` of a `case` pattern inside `$(...)` must not end the
substitution early.

## Acceptance criteria

1. In a script with a `"$"` before `)`, a `case` inside `$(...)`, an array
   whose strings and comments hold `)`, `${#arr[@]}`, `a#b`, nested
   backquotes, a quoted `<<'EOF'` and a tab-stripped `<<-END` here-document
   holding `source` and `./x.sh` lines, and an empty array `()`, only the
   `source` in a function inside `$(...)` and the final `source after.sh`
   (on its own line number) are imports.
