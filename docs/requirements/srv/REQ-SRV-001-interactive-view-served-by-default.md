---
id: REQ-SRV-001
uuid: af61b069-d569-40f8-9fbf-05f1bfa55f21
title: Interactive view served by default
scope: srv
type: functional
priority: must
status: implemented
verification:
  - integration
  - e2e
---

## Statement

When `--export` is not given, the system **shall** analyze the project and serve
the interactive map over its local HTTP server, printing the URL to open,
instead of writing an output file.

## Rationale

The interactive view is the primary output of the product; the file exports are
the exception that has to be asked for.

## Acceptance criteria

1. Running `depphunter` in a repository without `--export` starts the HTTP
   server and logs `serving at <url>`.
2. The same run writes no export file and does not exit until interrupted.

## Notes

Opening the browser, the token exchange and the host check are covered by scopes
`cli` and `sec`.
