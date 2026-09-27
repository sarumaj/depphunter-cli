---
id: REQ-LANG-015
uuid: 938c55d0-f066-4280-87ab-f1e080beaef0
title: Language detection by name
scope: lang
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** determine a file's language from its extension,
case-insensitively, or from well-known file names (for example `Dockerfile`,
`Makefile`, `go.mod`, `Gemfile`, `Rakefile`, and the Dockerfile names of
REQ-DOCKER-001), and
**shall** leave the language empty when neither is known.

## Rationale

Detecting by name needs no reading of the file and is enough for the language
colors and filters.

## Acceptance criteria

1. `a.go` is Go, `x.py` is Python, `Dockerfile`, `Containerfile`,
   `Dockerfile.dev` and `api.Dockerfile` are Docker; `Gemfile`,
   `lib/tasks/db.rake` and `app.gemspec` are Ruby.
2. A file with an unknown extension has no `lang`.
