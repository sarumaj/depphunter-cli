---
id: REQ-SUP-044
title: Composer dependencies from repository metadata
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a Composer package's requirements from its
repository's Composer 2 metadata: for Packagist
`https://repo.packagist.org/p2/<vendor>/<name>.json`, for any other repository
the `metadata-url` its `packages.json` names (asked once per run). It
**shall** expand the minified version list, take the version asked for (with
or without a leading `v`) or else the newest, and return its `require`
without the platform requirements, each with its constraint.

## Rationale

A library commits no lock, so what its packages depend on is only on the
index; the metadata document serves every version's requirements in one
request.

## Acceptance criteria

1. Against a stub repository, a version that inherits its `require` from the
   version above it returns those requirements, without `php` and `ext-*`.
2. A range returns the newest version's requirements; `packages.json` is asked
   once for several lookups.
3. With Packagist as the index the client asks `/p2/<name>.json` for the
   lower-case name directly.
