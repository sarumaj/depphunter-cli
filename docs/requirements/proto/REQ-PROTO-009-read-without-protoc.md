---
id: REQ-PROTO-009
title: Protocol Buffers read without protoc or buf
scope: proto
type: limitation
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall not** run protoc or buf. `.proto` files are read by a
scanner of the plugin's own; the `-I` flags a Makefile or script passes to
protoc are not read (the conventional roots of REQ-PROTO-004 stand in for
them); a type used from another file is not linked separately, as protobuf
requires an import of the declaring file, which is already the edge; the Buf
Schema Registry is not asked about a module's own dependencies (its API is
Connect RPC, not a plain index), so `--online` adds nothing for the `buf`
island, and neither OSV nor Trivy has a Buf ecosystem, so no vulnerability is
reported for a module.

## Rationale

The vendored tree-sitter proto grammar was measured on shallow clones of
grpc-ecosystem/grpc-gateway, bufbuild/protovalidate, envoyproxy/envoy's `api`
directory and two googleapis directories (1322 files): 2.4 to 3 ms per file,
and 9 files with ERROR nodes: the six using `edition = "2023"`, a proto2
group and two extension option paths (`.float.(ext) = 1.0`). The language's
statements are regular (declarations, blocks and `;`-terminated statements
with bracketed options and aggregate values), so a scanner reads them: the
plugin extracts envoy's 712 files in 37 ms and all
7341 files of googleapis in about 0.5 s of whole analysis.

## Acceptance criteria

1. A file with an option aggregate value, an rpc with an option block, a
   string containing `}` and an unterminated message keeps the declarations
   before and after them.
