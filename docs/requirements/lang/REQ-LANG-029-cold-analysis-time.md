---
id: REQ-LANG-029
uuid: 6f087598-abb9-4a34-bdb4-3fc7476887c6
title: Cold analysis time
scope: lang
type: non-functional
priority: must
status: implemented
verification:
  - integration
  - e2e
---

## Statement

A cold analysis (empty cache) of the reference project **shall** complete in
less than 5 s on the reference machine. The reference project is the 10 000-file
project `BenchmarkColdAnalysis` (`internal/analyze/bench_test.go`) generates. The
reference machine has 8 cores of the per-core speed of a 2.8 GHz Intel Xeon
(x86-64).

So that the target can be checked on any machine, the analysis **shall**
additionally:

1. spend no more than 36 CPU-seconds on the reference project at that per-core
   speed; and
2. run on N cores at least 0.85 × N times as fast as on one, for N up to 4.

## Rationale

The tool is meant to answer questions about an unfamiliar repository within a
minute, and the map is not shown before analysis ends. Nearly all of a cold run is
tree-sitter parsing (REQ-LANG-006), whose speed the runtime sets: about 1 MB/s
per core for TypeScript and 1.3 MB/s for Python. The time therefore depends on
the core count, and the requirement is stated against a machine. The two
additional criteria are what that machine's time is made of: CPU work, and how
evenly it is spread over the cores.

## Acceptance criteria

1. `go test ./internal/analyze -run '^$' -bench ColdAnalysis -benchtime 3x`
   reports less than 5 s per operation on the reference machine.
2. The same benchmark reports no more than 36 `cpu-s/op` at the reference
   per-core speed.
3. With `-cpu 1,4`, the 4-core time is at most 1 / 3.4 of the 1-core time.

## Notes

Measured on 2026-09-25 on a 4-core 2.8 GHz Intel Xeon container:

| Cores | Wall time | CPU time |
|------:|----------:|---------:|
| 1 | 32.8 s | 32.8 cpu-s |
| 2 | 17.2 s | 34.1 cpu-s |
| 4 | 8.9 s | 34.3 cpu-s |

Criteria 2 and 3 are met: the 4-core speed-up is 3.67. Criterion 1 is
extrapolated rather than measured, since no 8-core machine was available:
34.3 cpu-s over 8 cores at the measured 92 % efficiency gives about 4.7 s.
In a CPU profile, tree-sitter parsing and its queries account for 80-90 % of the
CPU time. The budget of 36 CPU-seconds is what 8 cores at 92 % efficiency get
through in 5 s.

On 4 cores the target cannot be met: the parsing alone takes more than 5 s of
wall time there. Meeting it on any machine would mean replacing tree-sitter
for TypeScript and Python with hand-written scanners, as C# and PowerShell
already are (REQ-CS-005, REQ-PS-009). That was judged not worth the loss of
parsing accuracy. A single worker pool shared by all plugins was also tried; it
made no measurable difference, since each plugin's pool already keeps every
core busy.
