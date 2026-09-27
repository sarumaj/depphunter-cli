---
id: REQ-BAZEL-012
title: Read by a scanner, not the grammar
scope: bazel
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Starlark **shall** be read by a scanner of its own (`internal/lang/starlark`,
shared with the index): `#` comments, strings with escapes, raw and bytes
prefixes and triple quotes, line continuations, logical lines ending only
outside brackets, calls with positional, keyword and `*`/`**` arguments,
lists, tuples, dicts, comprehensions, conditionals, lambdas and binary
operators, assignments and `def` bodies told by indentation. Brackets are
matched once; the reader **shall** return a result for any input in time
linear in its size, with nesting bounded.

## Rationale

The vendored Starlark grammar was measured on bazelbuild/examples,
abseil-cpp, rules_go and envoy: 2.2 to 7.2 ms per file (4.6 s for envoy's
1981 files), accurate (one ERROR file). The scanner extracts envoy's files
in about 0.13 s. A panic in Extract would end the analysis, so it was
fuzzed.

## Acceptance criteria

1. Every prefix of every fixture file and runs of brackets, calls, quotes,
   operators, conditionals, lambdas and definitions extract without a
   panic.
2. Strings holding quotes, `#` or line breaks do not end early or hide the
   statements after them.
