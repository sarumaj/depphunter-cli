---
id: REQ-SUP-078
title: PowerShell repositories and the Gallery's v2 API
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The repositories PowerShell's installers register **shall** be read from
this machine: PSResourceGet's `PSResourceRepository.xml` (`<Repository
Name Url|Uri Priority APIVersion>`), asked by priority (the lower first),
then by name, and PowerShellGet 2's `PSRepositories.xml` (PowerShell's
serialization: each entry's `Name` and `SourceLocation`), in its order; a
local repository and a container registry are not asked. They are asked in
turn, the first that has a module answering, the PowerShell Gallery
(`https://www.powershellgallery.com/api/v2`, the public index) only when one
of them is it or when neither file exists. A module an install names the
repository of (`Install-Module -Repository`, PSDepend's `Repository`)
**shall** be asked of the repository registered under that name alone
(`PSGallery` the Gallery while it is registered), and so **shall** its
dependencies.

With `--online`, a NuGet v3 feed (`.../index.json`) **shall** be read as
NuGet's; any other repository through NuGet's v2 OData API: one version
through `Packages(Id='<name>',Version='<version>')`, anything else through
`FindPackagesById()?id='<name>'`, its pages followed, taking the newest
release the requirement admits (a bare version is a minimum, as
`ModuleVersion` is; a NuGet range as NuGet reads it). The entry's
`Dependencies` (`Name:[range]:|...`, a framework after the second colon)
**shall** be the answer, `[1.2.3]` and a bare version pinning one version.

## Rationale

The Gallery lists each module's `RequiredModules` as the package's
dependencies; nothing in the repository does.

## Acceptance criteria

1. Both files' repositories are asked in PowerShell's order, a local
   repository and a container registry left out; an install's repository is
   asked alone; an unregistered Gallery is not asked; Windows keeps both
   files below `%LOCALAPPDATA%`.
2. A range, a minimum and no version take the newest admitted release
   (pre-releases skipped, every page read); one version is read directly
   (a `Packages()` answer being a single entry); a range nothing admits is
   reported.
3. A NuGet v3 repository is read as NuGet's, and an organization's own
   module is not named to the Gallery.

## Notes

The shapes were taken from PSResourceGet's sources (`RepositorySettings.cs`,
`V2ServerAPICalls.cs`, `PSResourceInfo.cs`) and PowerShellGet 2's
(`PartOne.ps1`, `Save-ModuleSources.ps1`); the Gallery is not reachable from
the sandbox. Not read: the SecretManagement vault entries and Azure Artifacts
credential provider a registration names (the credentials this machine holds
for the repository's host are sent), `Register-PSRepository` calls in the
repository's scripts, and a repository's `ScriptSourceLocation`.
