---
id: REQ-AUTH-014
uuid: acb97b20-36f3-4433-ab47-6d14bf9d3f13
title: pip keyring is not consulted
scope: auth
type: limitation
priority: must
status: implemented
source:
  - docs/REQUIREMENTS.md Known limits
verification:
  - inspection
---

## Statement

The system **shall not** consult pip's keyring for credentials; a PyPI mirror
whose credential is held only in a keyring **shall** be asked without one.

## Rationale

The keyring is platform-specific and not readable in pure Go. A credential in
the index URL or in netrc, which is how such a mirror is otherwise configured,
is read.

## Acceptance criteria

1. Inspection of internal/auth shows no keyring access.
2. A mirror whose credential is only in a keyring answers 401 and the report
   records that status.
