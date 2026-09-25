---
id: REQ-CFG-002
uuid: 482fbc25-1282-4cd8-99e0-44a46ca1ace7
title: User config file location
scope: cfg
type: interface
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The system **shall** read the user config file from `<user config
dir>/depphunter/config.yaml`, where `<user config dir>` is the platform's user
configuration directory (`$XDG_CONFIG_HOME`, or `~/.config` when unset, on Linux
and other Unix systems); a missing user config file **shall** be ignored.

## Rationale

Standing preferences belong in the user's own configuration directory, outside
any repository.

## Acceptance criteria

1. With `XDG_CONFIG_HOME=/x` on Linux, `/x/depphunter/config.yaml` is read.
2. Without that file the run proceeds with the other sources.

## Notes

The directory is Go's `os.UserConfigDir`: `$XDG_CONFIG_HOME` on Linux,
`~/Library/Application Support` on macOS and `%AppData%` on Windows.
