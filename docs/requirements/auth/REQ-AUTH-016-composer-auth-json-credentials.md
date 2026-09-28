---
id: REQ-AUTH-016
title: Composer auth.json credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read Composer credentials from this machine: the
`"config"` of `config.json` and `auth.json` in Composer's home
(`COMPOSER_HOME` when set; else on Windows `%APPDATA%\Composer`; else the first
existing of `$XDG_CONFIG_HOME/composer`, `~/.config/composer` and
`~/.composer`), then `COMPOSER_AUTH`, a JSON document of the same shape, each
merged host by host over the one before. It **shall** file `http-basic`
`{username, password}` and a `gitlab-token` `{username, token}` as Basic
credentials, and `bearer`, `gitlab-oauth`, `github-oauth` and a bare
`gitlab-token` as Bearer tokens, under the host (with its port, if any) the key
names; a `github.com` token is filed for `api.github.com` as well. For a host
named under several kinds, the kind Composer loads last (`github-oauth`,
`gitlab-oauth`, `gitlab-token`, `http-basic`, `bearer`) is the one sent.
`bitbucket-oauth` **shall not** be read. A repository reached over plain
`http://` in the home's `config.json` is a host this machine names over http
([REQ-AUTH-011](REQ-AUTH-011-credential-bound-to-host.md)).

## Rationale

Private Packagist, Satis, Repman, GitLab's Composer registry and GitHub are
reached with what `composer config --global --auth` writes, and a pipeline
supplies the same in `COMPOSER_AUTH`. A `bitbucket-oauth` entry is an OAuth
consumer key and secret that Composer exchanges for a token at bitbucket.org
on each run; it is not a credential to send as it stands.

## Acceptance criteria

1. Each kind in `auth.json` makes requests to its host, and only its host,
   carry the credential in the form listed; a `bitbucket-oauth` entry adds
   nothing.
2. A host in `COMPOSER_AUTH` replaces the same host of `auth.json` and leaves
   its other hosts in place; with `COMPOSER_HOME` set, `~/.composer` is not
   read; `$XDG_CONFIG_HOME/composer` comes before `~/.composer`.
3. A malformed `auth.json` or `COMPOSER_AUTH`, or an entry of the wrong shape,
   contributes nothing and leaves the other credentials in place.
4. A redirect to another host does not carry the credential.
