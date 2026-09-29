---
id: REQ-SUP-074
title: CocoaPods spec repositories read from their clones
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The spec repositories in CocoaPods' repos directory (`CP_REPOS_DIR`, else
`repos` below `CP_HOME_DIR`, else `~/.cocoapods/repos`) **shall** be read as
copies of the indexes a project names: a git clone (`pod repo add`) stands
for the URL of its `origin` remote, compared as CocoaPods compares them (case,
`.git`, a trailing slash and a credential in an https URL aside), and a clone
of the trunk repository (`github.com/CocoaPods/Specs`) for the CocoaPods CDN.
With `--online`, a pod asked of such an index **shall** be answered from the
clone, with nothing sent over the network: the specifications below `Specs/`
when the clone has it, else below its root, sharded by the MD5 of the pod's
name as the clone's `CocoaPods-version.yml` says (`prefix_lengths`), else not;
the version chosen as the CDN's is (REQ-SUP-051) among the clone's version
directories; `<Name>.podspec.json`, else the Ruby `<Name>.podspec` read without
running Ruby (its `dependency` calls, a platform's included, by root spec and
subspec, its default subspecs; test and app specs left out). A pod the clone
does not have **shall** pass the question to the next repository. A file the
directory of a CDN source (`.url`) holds, the trunk's downloaded version lists
and podspecs among them, **shall** be read from there before the CDN is asked.

A clone is no index of its own: the pods of a project are asked of the
repositories its Podfile lists, in order, as CocoaPods asks them. A
repository the project names (a Podfile `source`, a `Podfile.lock` spec
repository) that this machine has a clone of **shall** be known, since this
machine's own configuration names it; one without a clone is not
(REQ-SUP-043), and a git repository with no clone, even one the user vouches
for, **shall** not be read and **shall** pass the question on with a
`no-copy` note.

## Rationale

A company's private spec repository is a git repository no HTTP question can
be put to, but every machine that installs its pods has a clone of it. Read
from the disk, it answers without the network and without naming a pod to
anybody.

## Acceptance criteria

1. A Podfile listing a company repository cloned here (by a differently
   spelled URL) and the CDN answers the company's pod from the clone, a JSON
   podspec for a range, a Ruby podspec for an exact version, and a trunk pod
   from a sharded trunk clone, with no request made.
2. Without Podfile sources only the CDN is a candidate; a Podfile naming only
   the company repository switches the CDN off; a `Podfile.lock` repository
   with a clone is known, one without is not.
3. A vouched git repository without a clone gives a `no-copy` note and the
   CDN is asked instead.
4. A pod whose podspec the trunk source's directory holds is read from there;
   another is fetched from the CDN.
5. A pod name or version that is not one directory name reads nothing.

## Notes

Clones are read as they are: `pod repo update` is not run, so a clone is as
current as its last update. A git repository is recognized by its address (an
ssh, git or file URL, one ending in `.git`); a private CDN source is asked over
HTTP like the trunk's.
