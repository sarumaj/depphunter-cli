---
id: REQ-TERRAFORM-006
title: Files read by functions
scope: terraform
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** resolve a `file`, `templatefile`, `filebase64` or
`file*sha*`/`filemd5` call whose path is a literal - relative, or under
`${path.module}`, `${path.root}` or `${path.cwd}` - to that file of the
repository, relative to the calling file's directory, dropping it when the
file does not exist or the path is computed.

## Rationale

Templates and policy documents a module reads are part of it.

## Acceptance criteria

1. `templatefile("${path.module}/templates/init.sh.tpl", {...})` in the
   fixture's `main.tf` resolves to `templates/init.sh.tpl`, and
   `file("policy.json")` in `locals.tf` to `policy.json`.

## Notes

`path.root` and `path.cwd` are the root module's directory, which is the
calling file's own only in a root module; taking them as the module's
directory is a best effort.
