---
id: REQ-AUTH-004
uuid: 4dedcc9b-a4f5-4ee6-ab98-3ec205ba28d0
title: NuGet package source credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read `<packageSourceCredentials>` of the user's NuGet
configuration (`~/.nuget/NuGet/NuGet.Config` and `~/.config/NuGet/NuGet.Config`)
and file each `Username` and `ClearTextPassword` under the host of the
`<packageSources>` entry it names, a space in the key being written `_x0020_`.

## Rationale

Azure Artifacts, Nexus and ProGet feeds are kept behind these credentials.

## Acceptance criteria

1. The credential of "Company Feed" (element `Company_x0020_Feed`) is filed
   under the host of that source.
