---
id: REQ-SUP-051
title: Pod dependencies from the CocoaPods CDN
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a pod's dependencies from the CocoaPods CDN
(`https://cdn.cocoapods.org` unless configured otherwise), which serves the
trunk spec repository as files sharded by the MD5 of the pod's name
(`<a>/<b>/<c>`, its first three hex digits): the podspec of the version asked
for when it is exact (`Specs/<a>/<b>/<c>/<Name>/<version>/<Name>.podspec.json`),
else of the newest release that is not a pre-release and that the requirement
allows (`~>`, `>=`, `<`) in the shard's version list
(`all_pods_versions_<a>_<b>_<c>.txt`), else of the newest release. The answer is
the root spec's and its default subspecs' dependencies (all subspecs' when it
names none), without test specs and the pod's own subspecs, `= x` as a pinned
version. A Podfile's `source` lines and Podfile.lock's `SPEC REPOS` are the
repository's spec repositories: one the lock names serves the pods installed
from it, one only the Podfile names serves every pod. CocoaPods' own repository
(the CDN, `github.com/CocoaPods/Specs`, the lock's `trunk`) is never recorded as
the repository's own.

## Rationale

The CDN is what CocoaPods itself reads since 1.8; a private spec repository is
a git repository no HTTP question can be put to, and a pod named to the public
CDN that lives in one is a disclosure (REQ-SUP-038).

## Acceptance criteria

1. Against a stub CDN, AcmeKit 1.2.0 depends on AFNetworking `~> 4.0` and
   Mantle 2.2.0 (pinned, from its default subspec), not on PromiseKit (another
   subspec) or itself; `~> 1.0` reads 1.10.0, not 2.1.0 or the
   pre-release 2.0.0-beta.1.
2. A pod the lock installed from `https://github.com/acme/Specs.git` is
   looked up there (untrusted, so not asked); AFNetworking from `trunk` is
   the CDN's; a Podfile naming only a private repository besides the CDN makes
   it every pod's index.
