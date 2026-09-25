---
id: REQ-MD-001
uuid: c11f2dc4-cfb5-4948-bc34-86341c394843
title: Markdown links to repository files become edges
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** analyze every non-binary `.md`, `.markdown` and `.mdx`
file, and **shall** turn every link it carries to a file or a directory of the
repository into a dependency edge from the document to that file or directory,
drawn like an import; the link's fragment and query **shall** be ignored for the
edge, a percent-escaped path **shall** be unescaped, a relative path **shall**
be read from the document's directory and a path starting with `/` from the
repository root.

## Rationale

A document that links to a file depends on it and breaks when it moves, and
nothing a package manager reads says so.

## Acceptance criteria

1. `[spec](docs/SPEC.md)` and `[section of it](docs/SPEC.md#how-it-works)` both
   resolve to `docs/SPEC.md`.
2. `[directory](src)` resolves to the directory `src`.
3. `[from the root](/docs/SPEC.md)` resolves to `docs/SPEC.md`.
4. A URL, a `mailto:` address, a fragment of the same document and a path
   climbing out of the repository (`../elsewhere.md`) resolve to nothing.
5. A link to a path that exists but is not on the map (ignored, excluded,
   generated) resolves to nothing, without an error.
