---
id: REQ-FND-018
uuid: 6486b368-790c-471d-af8b-c6f31af8d575
title: One severity scale from CVSS vectors
scope: fnd
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** map every finding onto the severities critical, high,
medium, low and info, taking the severity from the advisory's CVSS v3 base score
where a Trivy or OSV entry carries a vector and from the severity word of the
report otherwise; npm audit ratings are taken as given.

## Rationale

The word a distribution chose often disagrees with the vector.

## Acceptance criteria

1. A Trivy finding whose CVSS vector disagrees with its severity word takes the
   vector's severity.
2. CVSS v3 base scores are computed as the specification defines them, rounded
   up to one decimal.

## Notes

A finding with neither a vector nor a recognized word is `unknown`, a sixth
value below info.
