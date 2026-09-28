---
id: REQ-AUTH-027
title: pub tokens
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The system **shall** read the tokens `dart pub token add` keeps in
`pub-tokens.json` in Dart's configuration directory (`%APPDATA%\dart` on
Windows, `~/Library/Application Support/dart` on macOS, else
`$XDG_CONFIG_HOME/dart` or `~/.config/dart`): each `hosted` entry's `token`,
or the value of the variable its `env` names. Each **shall** be sent as a
Bearer token to the URLs under the entry's `url` only - the whole host for a
server at its root, the path and below for one under a path - over what netrc
holds for the host. A token pub would refuse (anything but the characters of
RFC 6750's b64token) and an `env` entry whose variable is unset **shall** send
nothing.

## Rationale

A private pub server (Cloudsmith, JFrog, a self-hosted `unpub`) is reached
with the token `dart pub token add` stored; netrc alone left it answering 401.

## Acceptance criteria

1. A token for a server at a host's root reaches every URL of the host; one
   for a server under a path reaches that path and below, not a sibling path.
2. An `env` entry takes its variable; an unset one sends nothing.
3. On macOS the file is read from `~/Library/Application Support/dart`.
4. End to end, `PUB_HOSTED_URL` naming a server under a path answers with the
   token of an `env` entry.
