---
id: REQ-AUTH-004
title: NuGet package source credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read `<packageSourceCredentials>` of this machine's NuGet
configuration (the `nuget.config` files of the directories above the analyzed
one outside its checkout,
[REQ-SUP-079](../sup/REQ-SUP-079-configuration-files-above-the-analyzed-directory.md);
`%APPDATA%\NuGet\NuGet.Config` on Windows, elsewhere
`~/.nuget/NuGet/NuGet.Config` and `~/.config/NuGet/NuGet.Config`; the
additional user files; then the machine-wide files of
[REQ-SUP-065](../sup/REQ-SUP-065-nuget-configuration-layers-and-source-mapping.md)),
merged as NuGet merges them, and file each `Username` and `ClearTextPassword`
under the host of the enabled `<packageSources>` entry it names, a character a
tag name may not hold being written `_xHHHH_` in the key (a space
`_x0020_`).

## Rationale

Azure Artifacts, Nexus and ProGet feeds are kept behind these credentials.

## Acceptance criteria

1. The credential of "Company Feed" (element `Company_x0020_Feed`) is filed
   under the host of that source, on `pkgs.dev.azure.com` for its
   organization's path only
   ([REQ-AUTH-022](REQ-AUTH-022-nuget-environment-credentials.md)).
2. A user file's credential serves a source a machine-wide file defines; a
   disabled source gets none.
3. A `nuget.config` above the analyzed checkout gives its source's
   credential; one between the analyzed directory and the checkout's top
   gives none.
