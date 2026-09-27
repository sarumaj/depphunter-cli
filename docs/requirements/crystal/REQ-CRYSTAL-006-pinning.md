---
id: REQ-CRYSTAL-006
title: Pinning
scope: crystal
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

A shard **shall** be named by its name in `shard.yml` (the name `require`
and `lib/` use). A `shard.lock` (version 1.0 or 2.0, read from disk when not
scanned) entry **shall** pin the shard at its version (`1.2.3`, or
`0.3.1+git.commit.<sha>`), with the requirement as the requested version
when it differs. Without a lock entry: a `commit:` **shall** pin; a `tag:`
or an exact `version:` **shall** be shown, neither pinned nor floating,
since a tag can be moved; a `branch:`, a range and no requirement
**shall** float. A shard from a `git:` URL on a host other than GitHub,
GitLab, Bitbucket, Codeberg or sourcehut **shall** carry the URL as its
origin.

## Rationale

Shards are git repositories resolved by URL and tags; only the lock (or a
commit) fixes what is installed.

## Acceptance criteria

1. kemal (`~> 1.4`, locked 1.4.0), db (locked 0.13.1), markd (commit) and
   radix (override commit) are pinned; crinja (tag v0.8.1) is neither;
   internal (`>= 1.0` on git.acme.internal) floats with its URL as origin.
