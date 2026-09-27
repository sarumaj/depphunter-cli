---
id: REQ-BAZEL-005
title: Globs expanded within the package
scope: bazel
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A `glob()` in a label attribute **shall** link the BUILD file to every file
of its package the include patterns match and the `exclude` patterns do
not (`*` within a path segment, `**` across segments), not descending into
subpackages (directories with a BUILD file of their own) or nested
workspaces, and at most 5000 files per glob. A glob matching nothing
**shall** stay one dropped import. Plugins turn one raw import into several
through the optional `lang.Expander` of their resolver.

## Rationale

Most BUILD files name their sources by glob; without expansion a package
would link to none of them.

## Acceptance criteria

1. `glob(["*.cc"], exclude = ["*_test.cc"])` in src/ links shop.cc and not
   shop_test.cc; `glob(["**/*.h"])` links shop.h and detail/impl.h and not
   sub/sub.h (src/sub is a package).
2. Patterns with a dozen `**` segments match in bounded time.
