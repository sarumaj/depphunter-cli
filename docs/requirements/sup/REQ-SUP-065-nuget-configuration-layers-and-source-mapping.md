---
id: REQ-SUP-065
title: NuGet configuration layers, disabled sources and package source mapping
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

Index discovery **shall** merge NuGet's configuration files as NuGet does,
from the farthest to the closest: the machine-wide files (every `*.config` in
NuGet's machine-wide `Config` directory), the additional user files of the
`config` directory beside the user's `NuGet.Config`, the user's
`NuGet.Config`, the `nuget.config` files of the directories above the
analyzed one (those outside its checkout this machine's, those between it and
the checkout's top the repository's;
[REQ-SUP-079](REQ-SUP-079-configuration-files-above-the-analyzed-directory.md)),
a closer directory's closer, then the repository's `nuget.config` files the
scan finds, a deeper one closer than a shallower one.
In each of `<packageSources>`, `<disabledPackageSources>`,
`<packageSourceMapping>` and `<packageSourceCredentials>`, an entry of a closer
file **shall** replace the same key of a farther one, and a `<clear/>` **shall**
drop everything the section held before it. Keys **shall** be compared without
regard to case.

A source whose key `<disabledPackageSources>` sets to `true` **shall not** be
asked. nuget.org **shall** be switched off when no enabled source names it and
a `<clear/>` of `<packageSources>` took effect, a source naming it is
disabled, or the key `nuget.org` is disabled. A source keeps the trust of the
file that gave it its URL: one whose URL comes from the repository is the
repository's, whatever key it has.

A package that a `<packageSourceMapping>` pattern covers **shall** be asked
only of the enabled sources mapped to its most specific pattern (an exact id
over any prefix, a longer `Prefix.*` over a shorter one, `*` last; ids and
patterns compared without regard to case), and never of another index, the
public one included. A package mapped only to sources that are disabled or
undefined **shall** be asked of no index. A package no pattern covers is asked
as without a mapping, and **shall** be noted in the resolution report, since
NuGet itself would not restore it
([REQ-TRC-017](../trc/REQ-TRC-017-resolver-notes.md)).

## Rationale

`packageSourceMapping` is how an organization makes sure `Contoso.*` comes
from its own feed and is never looked up on nuget.org: asking nuget.org about
it would be the disclosure the mapping exists to prevent. `<clear/>` and
`<disabledPackageSources>` are how a repository or a machine says which feeds
NuGet may use at all; asking a feed they turned off asks a question NuGet
never would.

## Acceptance criteria

1. With `Contoso.*` mapped to a private feed, `Contoso.Billing` is answered by
   it and a `Contoso.*` package the feed lacks is not named to nuget.org; the
   longer `Contoso.Public.*` mapped to nuget.org wins over `Contoso.*`; an
   exact id wins over a prefix; a package no pattern covers is asked of every
   feed and then nuget.org.
2. A package mapped only to a disabled source is asked of nothing.
3. A disabled source receives no request; a disabled `nuget.org` key switches
   nuget.org off; a closer file's `value="false"` enables a source again.
4. A repository `nuget.config` with `<clear/>` drops this machine's feeds and
   nuget.org, and they return once the file is gone; a deeper `nuget.config`
   that names a machine key again makes it the repository's source; the
   machine-wide file's sources come first and the user file's URL wins their
   shared key.
5. Sources come out farthest first: an additional user file's, the user
   file's, a trusted one of a `nuget.config` above the checkout, an untrusted
   one of a `nuget.config` between the analyzed directory and the checkout's
   top, the project's.

## Notes

A package no pattern covers is asked as if there were no mapping (the feeds,
then nuget.org), where NuGet itself would refuse to restore it: nothing is
learned by refusing to look, and the public index is still never asked about a
package a pattern covers. Paket's sources are not NuGet sources and are not
mapped; a mapped package is not asked of them. The repository's
`nuget.config` files are merged into one configuration for the whole
repository (files in sibling directories are merged in path order), where
NuGet reads for each project only those of its own directory and above.
