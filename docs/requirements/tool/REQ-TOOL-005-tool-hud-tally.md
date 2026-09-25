---
id: REQ-TOOL-005
uuid: f86bbd9e-7ebd-4610-bdc2-97d6ec88c8fa
title: Each tool names its act and its tally
scope: tool
type: functional
priority: must
status: implemented
verification:
  - e2e
---

## Statement

Each tool **shall** define a verb for its use and a noun for its tally, and the
HUD messages about a tag **shall** use the noun of the tool that made it.

## Rationale

A module tagged by a dart is "tagged"; one reached by a rod is "landed".

## Acceptance criteria

1. Tagging with the dart reports the module as tagged; tagging with the rod
   reports it as landed.
