---
id: REQ-OBJC-011
uuid: 24a13801-8cb5-48ba-ad22-e1742592e26d
title: Carthage dependencies
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `Cartfile` and `Cartfile.private` entries
(`github`, `git`, `binary`) as imports of the `carthage` island, named by
their repository URL as the Swift plugin names packages (`github
"owner/repo"` is `github.com/owner/repo`, a URL without scheme and `.git`),
pinned by the `Cartfile.resolved` beside them; without it `== x` pins, `~>` and
`>=` float, a quoted git reference is shown and pins only when it is a commit,
and no requirement floats.

## Rationale

Carthage builds frameworks from repositories; its resolved file is its lock.

## Acceptance criteria

1. `github "Mantle/Mantle" ~> 2.2` resolved at 2.2.0 is pinned with
   `requested` `~> 2.2`; `github "pinterest/PINCache" == 3.0.3` pins; the
   `binary` entry `>= 1.0` floats.
