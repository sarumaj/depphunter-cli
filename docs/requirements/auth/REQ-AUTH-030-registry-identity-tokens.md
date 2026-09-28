---
id: REQ-AUTH-030
title: Container registry identity tokens
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

An `identitytoken` of a container credential file's `auths` entry, and the
secret a credential helper answers with the user name `<token>`, **shall** be
filed as that registry host's identity token, never as a Basic credential. A
higher-priority file's stored credential for the host **shall** replace a lower
file's identity token.

When a registry answers a request with a `Bearer` challenge and this machine
holds an identity token for the registry's host, the pull token **shall** be
asked for with the OAuth2 refresh-token grant of the Docker registry token
specification: a `POST` of `grant_type=refresh_token`, `service`, `scope`,
`client_id` and `refresh_token` to the challenge's realm, the `access_token` (or
`token`) of the answer then sent as the Bearer token of the request. The
identity token **shall** be sent only to a realm over https on the
registry's own host; any other realm **shall** be asked the anonymous `GET` as
without one. No other credential of this machine **shall** be sent with the
`POST`.

## Rationale

`docker login` to Azure Container Registry with an Entra identity (`az acr
login`), and several other registries, stores a refresh token rather than a
password. Declining it left such registries anonymous, so an image's base
image could not be read.

## Acceptance criteria

1. `identitytoken` entries are filed by registry host (Docker Hub's key as
   `registry-1.docker.io`) and are not an `Authorization` header; a stored
   pair in a higher file wins.
2. A helper answering `<token>` files an identity token; one answering another
   user name a Basic credential.
3. End to end against an https stub registry issuing a Bearer challenge: the
   realm receives exactly the form above, the manifest request carries the
   access token, and the base image is read.
4. A realm on another host, or over http on the registry's host, never
   receives the identity token.

## Notes

The refreshed `refresh_token` a token endpoint may return is not written back
to any file. Docker Hub's realm (`auth.docker.io`) is not on the registry's
host, so an identity token for Docker Hub is not used.
