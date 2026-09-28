---
id: REQ-SUP-061
title: Alire indexes for crate dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a crate of the
`alire` island depends on from an Alire index, a git repository of release
manifests: `<first two letters>/<crate>/<crate>-<version>.toml` below the
index's root, read from the checkout alr keeps of it on this machine when
there is one, else over HTTP (`<index>/index/...`; the `stable-1.4.0` branch
of alire-project/alire-index on raw.githubusercontent.com is the community
index, the public one). A constraint that is not one version (`^25.0`,
`~1.2`, `>=1 & <2`, `*`) **shall** be answered by the newest release it
admits in Alire's semantic versioning (`^` up to the next major version, `~`
up to the next minor one, a release before a pre-release): listed from the
checkout, or for the community index through GitHub's contents API on its
branch. An HTTP index with neither **shall** not be asked about a range; an
index only git serves, with no checkout, **shall** not be read and passes the
question on with a `no-copy` note; a commit or a branch is reported as having
no version to ask for. The release's `depends-on` crates (every `case(...)`
alternative counted) **shall** be returned with their constraints, an exact
one as its version. A name that is not an Alire crate name is not asked.

The indexes this machine's alr uses **shall** be discovered from its
settings directory (`ALIRE_SETTINGS_DIR`, else `~/.config/alire`): each
`indexes/<name>/index.toml`'s `url`, asked in the order of its `priority`
(lower first), with the checkout beside it (`indexes/<name>/repo`) or, for a
directory index, that directory; the community index's URL is the public
index, switched off when it is not configured and another is. A package is
asked of the indexes in order, the next one when an index does not have it.

## Rationale

The lock file's solution is complete only for the crates the project
locked; the index describes every release. alr keeps every index it uses on
disk, which lists releases without the network and reads private indexes.

## Acceptance criteria

1. aws 25.2.0 asks `index/aw/aws/aws-25.2.0.toml` only and yields
   gnatcoll 25.0.0 (pinned), make, winsock and xmlada with their
   constraints; against an HTTP index that cannot list, `^25.0`, a commit, a
   branch and no version ask nothing.
2. Configured indexes are asked by priority with their checkouts (a git
   index's, a directory index), the community index off when absent;
   `^1.0`, an `|` alternative, `~25.1`, `*` (a release over a pre-release),
   `>25.2` (a pre-release) and an exact version are answered from the
   checkouts with nothing sent, and a commit is not asked about.
3. Without a checkout, the community index's releases are listed through
   GitHub once and the newest admitted manifest read.

## Notes

alr's own solver is not run: the newest admitted release of each crate is
taken alone. GitHub's contents API allows 60 unauthenticated requests an
hour.
