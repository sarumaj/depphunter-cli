---
id: REQ-AUTH-018
title: Bundler credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read Bundler's per-server credentials from this machine:
the `BUNDLE_<HOST>` keys of the user's Bundler config (`BUNDLE_USER_CONFIG`
when set, else `config` in `BUNDLE_USER_HOME` when set, else
`~/.bundle/config`), then the `BUNDLE_<HOST>` environment variables, each
replacing the same key of the file. `<HOST>` is the host upper-cased with `.`
written `__` and `-` written `___`; a key may also be a source URL
(`BUNDLE_HTTPS://RUBYGEMS__PKG__GITHUB__COM/ACME/`), which is filed under that
URL's host and port. A host key **shall** be preferred to a URL key on the
same host. A value `user:password` **shall** be sent as Basic credentials, each
half URL-unescaped as Bundler does; a value without a colon is a token sent as
the user name with an empty password. Bundler's own dotted settings
(`build.*`, `local.*`, the `gem.*` defaults of `bundle gem`, `github.https`)
and any value containing `://` (a mirror) **shall not** be taken for a
credential. A source URL key with `http://`, and a `BUNDLE_MIRROR__*` mirror
with an `http://` address, name a host this machine reaches over http
([REQ-AUTH-011](REQ-AUTH-011-credential-bound-to-host.md)). A credential written
into a machine-configured source URL replaces Bundler's for that host, and
Bundler's replaces a netrc entry.

## Rationale

Gemfury, Artifactory, GitHub Packages and commercial gem servers (Sidekiq Pro,
Kiba Pro) are reached with what `bundle config set --global <host> <user:pass>`
writes, and a pipeline sets the same `BUNDLE_<HOST>` variable. Bundler puts
the value into the source URL's user information and sends it as HTTP Basic
credentials, so a bare token is the user name. Bundler keeps a URL's own user
information over the configured one, hence the order after the source URL.

## Acceptance criteria

1. `BUNDLE_GEMS__CONTRIBSYS__COM` in the environment replaces the same key of
   `~/.bundle/config`; each key makes requests to its host, and only its host
   (not a subdomain, not another port of a URL key), carry the credential.
2. With `BUNDLE_USER_CONFIG` or `BUNDLE_USER_HOME` set, `~/.bundle/config` is
   not read.
3. `BUNDLE_BUILD__NOKOGIRI`, `BUNDLE_GEM__TEST`, `BUNDLE_LOCAL__RACK`,
   `BUNDLE_WITHOUT` and a mirror file nothing; `gem.fury.io` and `github.com`
   are hosts.
4. A stub gem server that refuses requests without Basic credentials answers
   the compact index client when `BUNDLE_<HOST>` names it, in the environment or
   in `~/.bundle/config` beside a mirror of rubygems.org pointing at it.
5. Over plain http the credential is sent only to a host named with an
   `http://` URL key or mirror.
