---
id: REQ-MD-016
uuid: 8d3f7718-a010-43b4-98b1-9dd131801cb1
title: Link check can be turned off
scope: md
type: functional
priority: must
status: implemented
verification:
  - unit
  - extension
---

## Statement

The system **shall** turn the link check off when given `--no-links`, the
environment variable `DEPPHUNTER_LINKS=false`, or `links: false` in the project
configuration, and the VS Code extension **shall** pass `--no-links` when its
`depphunter.links` setting is off.

## Rationale

The check runs by default, so a user who does not want its findings needs a way
to switch it off from every configuration source.

## Acceptance criteria

1. `--no-links`, `DEPPHUNTER_LINKS=false` and `links: false` each disable the
   link check.
2. With `--no-links` and `--no-vulns`, no findings are collected at all.
3. The extension turns an off `depphunter.links` setting into `--no-links`.

## Notes

Configuration precedence belongs to scope `cfg`.
