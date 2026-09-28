---
id: REQ-AUTH-019
title: Repository Bundler credentials discarded
scope: auth
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall not** take a credential from a repository's Bundler
configuration: the application config (`.bundle/config` beside a `Gemfile`, or
the `config` in the directory `BUNDLE_APP_CONFIG` names) and a credential in a
`Gemfile` source URL contribute nothing, while the source itself is still an
index ([REQ-SUP-015](../sup/REQ-SUP-015-index-configuration-sources.md)).

## Rationale

As for [REQ-AUTH-012](REQ-AUTH-012-url-credential-from-machine-only.md): a
repository that could supply a credential could also choose where it is sent.
`BUNDLE_APP_CONFIG` is not read even when it points outside the checkout: it
is the per-application layer, and the machine's credentials belong in the
user's config or the environment.

## Acceptance criteria

1. A checkout with a `Gemfile` and a `.bundle/config` naming credentials for
   its gem server, with `BUNDLE_APP_CONFIG` pointing at that or another
   directory of the checkout, makes no request carry a credential.
2. The `Gemfile`'s sources are recorded without the credential.
