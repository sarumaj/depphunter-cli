---
id: REQ-SUP-052
title: Rock dependencies from a rocks server
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The index client **shall** read a rock's dependencies from a rocks server
(`https://luarocks.org` unless configured otherwise) as the luarocks client
does: a version with its revision (a locked `1.14.0-3`) is fetched as
`<server>/<rock>-<version>.rockspec` directly; otherwise the server's
manifest (`manifest-5.1.zip`, else `manifest-5.1`, read once per server)
lists the versions, and the newest release the constraint allows is taken
(an exact version meaning `==` it, `~>` LuaRocks' partial match, `scm`
and `dev` builds only when nothing else matches). The answer is the
rockspec's run-time `dependencies`, every platform's, without `lua`,
`== x` as a pinned version. The rocks servers of a LuaRocks configuration
file are an index: `LUAROCKS_CONFIG` and `~/.luarocks/config-5.x.lua` this
machine's, a project's `.luarocks/config-5.x.lua` the repository's;
luarocks.org itself is never recorded as one of them.

## Rationale

luarocks.org has no per-rock JSON API; the manifest and the rockspecs are
what every rocks server, public or private, serves. The sandbox this was
built in cannot reach luarocks.org, so the protocol is verified against a
stub server.

## Acceptance criteria

1. Against a stub server, `acme-http ~> 1.10` reads the manifest (zipped, or
   plain when there is no zip) and 1.10.0-2's rockspec: luaposix, luasocket
   3.1.0 (pinned) and penlight `~> 1.5`, not lua or the test dependency;
   `1.2.0-1` pinned is fetched without the manifest again.
2. A project's `.luarocks/config-5.1.lua` naming luarocks.org and
   `https://rocks.corp.test/` makes the latter the (untrusted) index; the same
   in `~/.luarocks/config-5.4.lua` or `LUAROCKS_CONFIG` is this machine's.
