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
`== x` as a pinned version; a rock the manifest does not list, or lists with no
version the constraint admits, is not on that server. The rocks servers of a
LuaRocks configuration file are an index: the user's configuration
([REQ-SUP-064](REQ-SUP-064-tool-configuration-locations.md)) this machine's, a
project's `.luarocks/config-5.x.lua` the repository's. As `rocks_servers`
replaces LuaRocks' default list and LuaRocks searches every server on it, its
servers **shall** be asked in order, luarocks.org (by any of its addresses)
only when it is on the list; the entries of a group (`{ "url", "mirror" }`)
are one server's mirrors, the next asked after any failure of the one before.
A local server (a directory, `file:`) is not asked.

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
   `https://rocks.corp.test/` asks luarocks.org, then the latter (untrusted);
   a list in `~/.luarocks/config-5.4.lua` or `LUAROCKS_CONFIG` (which
   replaces the home file) is this machine's, and without luarocks.org on it
   luarocks.org is not asked.
3. End to end, a rock the first server lacks, or has in no admitted version,
   is answered by the second.

## Notes

LuaRocks keeps the newest version any server has; depphunter takes the first
server in the list that has an admitted one. Every Lua version's
configuration file is read, the Lua version in use being unknown.
