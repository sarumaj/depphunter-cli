---
id: REQ-AUTH-003
title: Maven server credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read the `<servers>` of Maven's settings files (the
user's `~/.m2/settings.xml`, then `conf/settings.xml` of `MAVEN_HOME`, else
`M2_HOME`) and file each server's username and password under the host of the
`<mirror>` or profile `<repository>` (active or not) whose id it names, or of
the repository of the Clojure CLI's user `deps.edn` (`:mvn/repos`) whose name
it is. The user's file **shall** win where both define a server or a
repository id. A repository only the project declares (a POM's, a project
`deps.edn`'s) **shall not** receive a server's credential.

## Rationale

A Maven server entry names a credential by id; the host is written in the mirror
or repository with the same id.

## Acceptance criteria

1. A server whose id matches a mirror is filed under the mirror's host, and one
   matching a profile repository under that repository's host.
2. A server that no mirror or repository names is not kept.
3. With `MAVEN_HOME` set, a server of the installation's settings finds a
   mirror there, and the user's server of the same id wins over it.
4. A server named like a repository of the user's `deps.edn` (`CLJ_CONFIG`)
   is filed under that repository's host.
5. A stub repository requiring Basic credentials, named by an active profile,
   answers the Maven client with its server's credential.
