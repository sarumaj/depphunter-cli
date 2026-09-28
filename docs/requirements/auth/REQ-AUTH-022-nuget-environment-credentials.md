---
id: REQ-AUTH-022
title: NuGet credentials from the environment
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** take a NuGet source's credential from the
`NuGetPackageSourceCredentials_<source>` variable (`Username=...;Password=...`,
perhaps with `ValidAuthenticationTypes=...`) of a source that this machine's
NuGet configuration names, the variable's prefix and the source's name being
matched without regard to case; the variable **shall** win over the source's
`<packageSourceCredentials>`. It **shall** take each `endpoint`'s `username`
and `password` from `VSS_NUGET_EXTERNAL_FEED_ENDPOINTS`, the Azure Artifacts
credential provider's JSON, the user name being optional.

A credential whose `ValidAuthenticationTypes` does not allow `basic`, a variable
for a source this machine's configuration does not name, and a disabled source
**shall** give nothing. A NuGet credential **shall** be filed under the
source's host and port; on `pkgs.dev.azure.com`, which every Azure DevOps
organization shares, it **shall** serve only the organization's path
(`/<organization>/`).

## Rationale

A pipeline does not write a password into a `NuGet.Config`: it sets one of
these variables, which is what `dotnet restore` reads on a build agent.
Filing an Azure Artifacts token for all of `pkgs.dev.azure.com` would send one
organization's token with a request for another organization's feed.

## Acceptance criteria

1. The variable of "Corp Feed" (spelled in another case) serves that source and
   not another source of the machine's configuration, end to end against a
   stub feed requiring it; it wins over the file's credential.
2. A variable for a source no file names, a disabled source's, and one limited
   to `negotiate` give nothing.
3. `VSS_NUGET_EXTERNAL_FEED_ENDPOINTS` serves its endpoints: an Azure DevOps
   endpoint for its organization's path only, another host for all of it; an
   entry without a password and malformed JSON give nothing.

## Notes

A source defined only by the repository's `nuget.config` gets its variable
only under [REQ-AUTH-023](REQ-AUTH-023-repository-feed-credentials-from-the-environment.md).
The credential provider plugins themselves (`VSS_NUGET_ACCESSTOKEN`, device
flow, `ARTIFACTS_CREDENTIALPROVIDER_*`) are not run.
