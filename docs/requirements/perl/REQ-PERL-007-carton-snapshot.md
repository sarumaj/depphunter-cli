---
id: REQ-PERL-007
uuid: 26de562d-f24a-4665-b241-6e598b6b7427
title: Carton snapshot read
scope: perl
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The resolver **shall** read `cpanfile.snapshot` (Carton's, beside the
manifests, from disk): each distribution's name and version (split at the
last `-` before a version), the modules it provides and those it requires. A
module a governing snapshot provides **shall** resolve to that distribution,
pinned to its version, with the requirement as Requested when it differs.
`--resolve-depth` **shall** follow the snapshot's requirements: each
required module becomes the distribution the snapshot says provides it
(pinned), a core module nothing installed is left out, and any other module
the distribution its name gives (a minimum version floats).

## Rationale

The snapshot is the lock file of Perl applications and maps modules to
distributions exactly, the one thing no name heuristic can.

## Acceptance criteria

1. `use Plack::Request` resolves to Plack 1.0050 pinned, requested `>=
   1.0047`.
2. Plack's dependencies are HTTP-Message 6.45 and Try-Tiny 0.31 (pinned) and
   Hash-MultiValue `>= 0.05` (floating); Carp and perl are left out.
