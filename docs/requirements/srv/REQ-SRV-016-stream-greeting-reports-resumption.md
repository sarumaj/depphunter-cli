---
id: REQ-SRV-016
title: Stream greeting reports resumption
scope: srv
type: functional
priority: must
status: implemented
verification:
  - integration
---

## Statement

On every connection the server **shall** first send a `hello` event carrying the
current graph version, entity tag and announcement sequence, and whether the
`Last-Event-ID` sent by the client equals the current sequence (`resumed`).

## Rationale

Nothing is announced twice, so a client has to be told whether it slept through
an announcement.

## Acceptance criteria

1. A first connection is greeted with `resumed: false`.
2. A client whose `Last-Event-ID` is the current sequence is greeted with
   `resumed: true`.
3. A client behind the sequence, or with an unreadable id, is greeted with
   `resumed: false`.
