---
id: REQ-SUP-068
title: Container registry mirrors
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The index client **shall** ask for an image where this machine's container
tools pull it from:

- registries.conf(5), version 2: `CONTAINERS_REGISTRIES_CONF`, else the user's
  `$XDG_CONFIG_HOME/containers/registries.conf`
  (`~/.config/containers/registries.conf`) when it exists, else
  `/etc/containers/registries.conf`; then the `*.conf` files of
  `/etc/containers/registries.conf.d` and of the user's
  `containers/registries.conf.d`, each directory by name (only the user's when
  the user's registries.conf is the main file), a later file's `[[registry]]`
  replacing the one with the same `prefix` (the `location` when no prefix is
  written);
- the `[[registry]]` whose `prefix` matches the most of the image's full name
  (`docker.io/library/nginx` for `nginx`) at a `/` boundary, or a `*.domain`
  prefix matching the image's host, rewrites it: each `[[registry.mirror]]`
  that serves the reference (`pull-from-mirror` / `mirror-by-digest-only`:
  digests alone, tags alone, or both) is asked first, in order, then the
  `location`, the matched prefix replaced by each one's location; a
  `blocked = true` registry is never asked, and the report says so;
- without a matching `[[registry]]`, a Docker Hub image is asked of the
  Docker daemon's `registry-mirrors` first, in order - the rootless daemon's
  `$XDG_CONFIG_HOME/docker/daemon.json` (`~/.config/docker/daemon.json`) when
  it exists, else `/etc/docker/daemon.json`; Docker Desktop's
  `~/.docker/daemon.json` on macOS and Windows, else the Windows engine's
  `%ProgramData%\docker\config\daemon.json` - and then of Docker Hub.

A mirror **shall** be passed over on any failure, as containers/image and
dockerd move on from one. Mirrors and a configured location are this machine's
configuration and are asked like it; the map attributes the image to its
registry (the location), and the report to the endpoint that answered.

## Rationale

A build farm that pulls through a mirror or a pull-through cache often cannot
reach Docker Hub or quay.io at all, and rate limits make asking Docker Hub
directly the slow path everywhere else.

## Acceptance criteria

1. A registries.conf mirror is asked first under its rewritten path; the
   location is asked when the mirror answers 404 or 500, not when it answers.
2. A digest-only mirror is not asked for a tag, a tag-only one not for a
   digest.
3. A blocked registry receives no request; a longer unblocked prefix on the
   same host still does.
4. Drop-ins merge in order (the system's directory, then the user's), by
   prefix; `CONTAINERS_REGISTRIES_CONF` names the main file; a user
   registries.conf leaves the system's files unread.
5. Prefixes match whole segments, the longest wins, and a wildcard matches
   subdomains only.
6. The daemon's `registry-mirrors` serve Docker Hub images only, before Docker
   Hub; `/etc/docker/daemon.json` is read when the rootless one is absent;
   entries dockerd refuses (no scheme, a path) are ignored.

## Notes

`unqualified-search-registries`, `short-name-mode` and short-name `[aliases]`
are not applied: the oci plugin names Docker Hub images the short way
whichever way they were written
([REQ-DOCKER-004](../docker/REQ-DOCKER-004-docker-hub-names-canonical.md)),
so a short name cannot be told from `docker.io/library/...` written in full,
and Docker,
which builds most Dockerfiles, searches nothing. `insecure = true` does not
switch a location to plain http (it is asked over https, and a mirror that
fails is passed over). Version 1 files (`[registries.search]`,
`[registries.block]`) are not read. Credentials are per host, as for any
registry ([REQ-AUTH-005](../auth/REQ-AUTH-005-container-registry-stored-credentials.md)).
