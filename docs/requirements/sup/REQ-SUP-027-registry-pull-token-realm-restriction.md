---
id: REQ-SUP-027
uuid: 55e2fb92-3870-4b15-a873-81e093786706
title: Pull-token challenge realm
scope: sup
type: constraint
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md M12
verification:
  - unit
---

## Statement

The index client **shall** answer a registry's Bearer pull-token challenge only
where the realm is an HTTPS URL or on the registry's own host, and **shall** use
the token for that request only.

## Rationale

A challenge is a header naming wherever it likes; a plain-HTTP realm on another
host would redirect the request, and its credentials, elsewhere.

## Acceptance criteria

1. A challenge whose realm is on the registry's own host is answered and the
   manifest is read.
2. A challenge whose realm is plain HTTP on another host is not followed.
3. A registry that answers 401 with no usable challenge is recorded as
   unauthorized.

## Notes

No automated test covers the refusal of a foreign plain-HTTP realm.
