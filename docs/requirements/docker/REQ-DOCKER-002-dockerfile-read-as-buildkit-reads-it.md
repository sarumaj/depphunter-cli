---
id: REQ-DOCKER-002
title: Dockerfile read as BuildKit reads it
scope: docker
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The docker plugin **shall** split a Dockerfile into instructions following
BuildKit's rules: parser directives at the top of the file (`escape` choosing
the continuation character), comment lines dropped even inside a continued
instruction, line continuations joined, instruction keywords matched
case-insensitively, the bodies of heredocs (`<<EOF`, `<<-EOF`, quoted
markers) skipped, and an `ONBUILD` instruction read as the instruction it
carries. Each instruction **shall** be reported on the line it starts on.

## Rationale

A hand-written line parser is used rather than a grammar: the syntax is
line-oriented, and what decides which image an instruction names (build
arguments, earlier stages) has to be worked out in source order in any case.
Skipping heredocs keeps a script's text from being read as an instruction.

## Acceptance criteria

1. With the `escape` directive set to a backtick, a line ending in a backtick
   continues and one ending in a backslash does not; CRLF line ends are
   accepted.
2. `from alpine:3` continued onto a line `as base` is one `FROM` instruction.
3. A `FROM` inside a heredoc body is not an instruction; `<<` inside a
   shell expression (`$((1<<2))`) and a marker whose terminator never comes
   are not heredocs.
4. A comment line inside a continued `RUN` does not end it.
