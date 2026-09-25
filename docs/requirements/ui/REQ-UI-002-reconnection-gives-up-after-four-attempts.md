---
id: REQ-UI-002
uuid: 5a88ad9b-a306-449c-aba1-c5880f5c018f
title: Reconnection gives up after four attempts
scope: ui
type: functional
priority: must
status: implemented
verification:
  - manual
  - e2e
---

## Statement

When the event stream to depphunter fails, the UI **shall** show that it is
reconnecting, and after four failed attempts **shall** stop reconnecting and say
that depphunter has stopped.

## Rationale

Once depphunter has stopped, retrying forever is an endless stream of failed
requests instead of an answer.

## Acceptance criteria

1. Stopping depphunter under an open map shows "reconnecting..." and then
   "depphunter stopped", after which no further requests are made.
