---
id: REQ-OBJC-011
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
and no requirement floats. A dependency Carthage checked out into
`Carthage/Checkouts/<name>/` beside the `Cartfile` **shall** depend, for
`--resolve-depth`, on the entries of its own `Cartfile` (not its
`Cartfile.private`; its `Cartfile.resolved` when it has no `Cartfile`), each
pinned by the project's `Cartfile.resolved` when that lists it, else by the
checkout's, and be reported as installed; a missing or unreadable checkout
says nothing.

## Rationale

Carthage builds frameworks from repositories; its resolved file is its lock,
a flat list, while each checkout's `Cartfile` says what it depends on.

## Acceptance criteria

1. `github "Mantle/Mantle" ~> 2.2` resolved at 2.2.0 is pinned with
   `requested` `~> 2.2`; `github "pinterest/PINCache" == 3.0.3` pins; the
   `binary` entry `>= 1.0` floats.
2. The checked-out AlamofireImage depends on Alamofire at the project's
   resolved 5.8.1 and Kingfisher at its own resolved 7.10.0 (requested
   `~> 7.0`), not on Nimble from its `Cartfile.private`; a checkout whose
   `Cartfile` is not one, and a dependency without a checkout, have none.
