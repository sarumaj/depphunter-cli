---
id: REQ-HIST-016
title: Git never fetches or asks for credentials
scope: hist
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every git command depphunter runs **shall** read only the repository on disk: no
transport **shall** be used, no lazy fetch made, no credential helper consulted
and nothing prompted. In a partial clone, where the history with lines counted
cannot be read without the contents left out, the history **shall** be read from
the commits and trees alone, without renames followed, and **shall** say that it
counted no lines (`noLines`); the map **shall** then offer no "Lines changed".

## Rationale

A partial clone fetches what it is missing whenever a command needs it, and the
fetch runs the user's credential helper: under `--watch` that asked for a
password on every analysis, for figures nobody had asked to go online for.

## Acceptance criteria

1. In a blobless clone with a credential helper configured, the history is read,
   with commits, authors and files, the helper is never run and no blob is
   fetched.
2. The history of a partial clone carries `noLines: true`; a whole clone's does
   not.
3. With `noLines`, "Lines changed" is disabled in the color menu and left out of
   the tooltip and the details panel.
