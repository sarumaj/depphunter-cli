---
id: REQ-PROTO-002
uuid: 1ccb5b80-e1b7-434c-9f2a-2f35b18e07ef
title: Protocol Buffers imports
scope: proto
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** record every `import "path";`, `import public "path";`
and `import weak "path";` statement of a `.proto` file as an import, shown as
written (`import public "a/b.proto"`), with adjacent string literals joined as
protoc joins them, and **shall not** read imports from comments or strings.

## Rationale

An import is the only way a `.proto` file uses a type another file declares; a
public import is also re-exported to the importer's importers, and a weak import
is still a build dependency.

## Acceptance criteria

1. `import weak "acme/billing/v1/legacy.proto"` and
   `import public "common/v1/types.proto"` are imports with those specs.
2. `import "a/" "b.proto";` is the import `a/b.proto`; an import inside a
   `//` or `/* */` comment is not recorded.
