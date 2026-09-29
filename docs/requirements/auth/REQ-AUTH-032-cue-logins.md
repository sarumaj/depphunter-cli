---
id: REQ-AUTH-032
title: cue login tokens
scope: auth
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The tokens `cue login` keeps in `logins.json` (in `CUE_CONFIG_DIR`, else `cue`
in the platform's configuration directory) **shall** each be sent as a Bearer
token to the registry host they were stored for, ahead of a Docker credential
for the same host; a token whose type is not Bearer **shall not** be sent. An
expired token is sent as it is (cue would refresh it).

## Rationale

A private CUE registry answers only to the token cue stored for it.

## Acceptance criteria

1. registry.cue.works gets the stored token rather than Docker's Basic pair;
   a token of another type is not sent; CUE_CONFIG_DIR moves the file.
