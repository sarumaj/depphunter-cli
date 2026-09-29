---
id: REQ-AUTH-036
title: GitHub tokens for GitHub's REST APIs
scope: auth
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The token the GitHub CLI would send a GitHub host **shall** be sent to that
host's REST API and to no other host: github.com's API (api.github.com)
`GH_TOKEN`, else `GITHUB_TOKEN`, else the `oauth_token` of github.com in gh's
`hosts.yml`; an Enterprise Server instance `GITHUB_API_URL` or `GH_HOST` names
`GH_ENTERPRISE_TOKEN`, else `GITHUB_ENTERPRISE_TOKEN`, else (for
`GITHUB_API_URL`'s, on a runner of that instance) `GITHUB_TOKEN`, which then
**shall not** go to github.com, else its own `hosts.yml` entry; any other host
of `hosts.yml` its own token. An Enterprise Server's token **shall** be
limited to its `/api/v3/` paths, a data-residency host's to `api.<name>.ghe.com`.

## Rationale

GitHub's API answers 60 requests an hour without a token, and a private or
internal action is not served without one.

## Acceptance criteria

1. hosts.yml's tokens go to their hosts' APIs (an Enterprise Server's paths
   only), a host without a token (keyring) gets none, and nothing goes to
   raw.githubusercontent.com.
2. `GH_TOKEN` wins over `GITHUB_TOKEN`, which wins over the file.
3. On a runner of an Enterprise Server, `GITHUB_TOKEN` goes to that instance
   and github.com keeps the file's token; `GH_ENTERPRISE_TOKEN` goes to
   `GH_HOST`'s instance.
4. `GH_CONFIG_DIR` moves `hosts.yml`.

## Notes

The rules follow cli/go-gh's `auth.TokenForHost` and `config.ConfigDir`. The
token gh keeps in the system's keyring (its default) is not read, and `gh
auth token` is not run.
