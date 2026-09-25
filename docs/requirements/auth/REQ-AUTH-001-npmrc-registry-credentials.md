---
id: REQ-AUTH-001
uuid: c8b95e62-4896-46d5-8fef-395100e06034
title: npm per-registry credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read from the user's `~/.npmrc` the four per-registry
credential forms `//<host>/...:_authToken`, `:_auth` (base64 "user:password"),
`:username` and `:_password` (base64), and **shall** file each under the host it
names.

## Rationale

npm tokens are where a developer's registry credential lives; without them a
private registry answers 401.

## Acceptance criteria

1. A token is sent as a Bearer credential, and `_auth` and the username/password
   pair as Basic credentials, to their host.
2. A token written as `${NPM_TOKEN}` is resolved from the environment.
