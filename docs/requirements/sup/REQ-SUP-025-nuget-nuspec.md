---
id: REQ-SUP-025
uuid: 1e4d5ece-788b-4ffd-99a3-1a375a2babed
title: NuGet dependencies from the nuspec
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a NuGet package's dependencies from its nuspec,
located through the feed's service index; without a version it **shall** use the
newest release on the feed.

## Rationale

A NuGet feed is a service index naming the resources it offers; the answer is
the same for every package on the feed and is kept for the run.

## Acceptance criteria

1. Against a stub feed, flat and framework-grouped dependencies are returned
   once each.
2. Without a version, the newest release, not a pre-release, is read.
