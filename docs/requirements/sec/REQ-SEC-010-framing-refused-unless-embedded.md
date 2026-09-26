---
id: REQ-SEC-010
uuid: d045c211-de66-45b3-ac26-e520bbfc7a26
title: Framing refused unless an embedding origin is named
scope: sec
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

By default the server **shall** send `X-Frame-Options: DENY` and a
Content-Security-Policy with `frame-ancestors 'none'` on every response. With
one or more `--embed <origin>` flags it **shall** instead name exactly those
origins in `frame-ancestors` and omit `X-Frame-Options`. `--embed` **shall** be
accepted only on the command line, never from a configuration file or the
environment, and each value **shall** be a scheme (`vscode-webview:`) or a
scheme with a host, which may begin with a `*.` wildcard, and an optional port;
any other value **shall** be rejected before the server starts.

## Rationale

A page able to frame the map could overlay it and steer the user's clicks.
Embedding is needed only by a program that launches depphunter itself, such as
the VS Code extension, so only its command line may turn it on; and what is
passed lands in a security header, so nothing in it may end the directive early
or start another.

## Acceptance criteria

1. Without `--embed`, a response carries `X-Frame-Options: DENY` and
   `frame-ancestors 'none'`.
2. With `--embed vscode-webview:`, a response carries
   `frame-ancestors vscode-webview:` and no `X-Frame-Options`.
3. `embed` in a configuration file or the environment enables nothing.
4. A bare CSP keyword such as `'self'`, or a value containing a space, a quote,
   a slash after the host or a semicolon, is refused.

## Notes

The other effects of embed mode - the token kept in the address and the
interface files served without it - are stated in
[REQ-SEC-004](REQ-SEC-004-unauthenticated-requests-refused.md); the origins the
extension passes are in
[REQ-EXT-022](../ext/REQ-EXT-022-fixed-hosting-arguments.md).
