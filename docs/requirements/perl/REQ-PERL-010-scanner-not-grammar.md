---
id: REQ-PERL-010
title: Read by a scanner, not the grammar
scope: perl
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Perl sources and manifests **shall** be read by a token scanner, not a
tree-sitter grammar: POD blocks, here-documents (`<<"X"`, `<<'X'`, `<<~X`,
`<<X`, several on one line), `__END__`/`__DATA__`, formats, comments,
strings, `q`/`qq`/`qw`/`qx`/`m`/`qr`/`s`/`tr`/`y` with any delimiters
(nesting brackets, `s{...}{...}` with its second part apart), patterns told
from division by what precedes them, punctuation variables (`$"`, `$/`,
`$#array`, `$;`), hash keys and file tests named like quote operators, and
prototypes (`($;$)`) are tokens. The scanner, the manifest readers and the
snapshot reader **shall** return a result for any input.

## Rationale

The vendored Perl grammar was measured on mojo, Plack and metacpan-web: 5
to 23 ms per file (6.3 s for mojo's 274 files, 465 ms for one test file) and
ERROR nodes in 3 to 6% of files. The scanner reads mojo in about 0.2 s of
CPU. A panic in Extract would end the analysis, so it was fuzzed.

## Acceptance criteria

1. Every prefix of every fixture file, and inputs cut inside each construct,
   extract without an error or a panic; 100000 nested brackets do not exhaust
   the stack.
2. An import after each trap (division then pattern, `$"`, `$h{s}`, `-s
   $file`, `s{..}\n{..}`, `tr///`, `q<..<..>..>`, here-documents, POD,
   formats, prototypes, `<$fh>`) is read.
