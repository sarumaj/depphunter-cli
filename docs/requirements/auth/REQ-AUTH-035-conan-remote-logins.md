---
id: REQ-AUTH-035
title: Conan remote logins exchanged for a token
scope: auth
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

The login Conan would use for each remote of this machine's `remotes.json`
(in `CONAN_HOME`, else `~/.conan2`; ConanCenter as `conancenter` when there is
none) **shall** be read as Conan finds it: the home's `credentials.json` entry
for the remote's name (`{"credentials": [{"remote", "user", "password"}]}`,
the environment variables its Jinja2 template names as `os.getenv("NAME")` or
`os.environ["NAME"]` substituted, an entry still holding template syntax left
out), else `CONAN_LOGIN_USERNAME_<NAME>` and `CONAN_PASSWORD_<NAME>` (the name
upper case, `-` as `_`), else `CONAN_LOGIN_USERNAME` and `CONAN_PASSWORD`. A
password without a user is no login.

A remote **shall** be asked without the login first, as Conan asks it; when it
answers 401, the login **shall** be sent as Basic credentials to its `GET
/v2/users/authenticate` alone, once per remote, and the token that answers
**shall** be sent as a Bearer token from then on. The login **shall** go only
to a remote reached over https or on this machine, and is never an
Authorization header of any other request. A repository's Conan home **shall
not** supply a login.

## Rationale

Conan remotes behind Artifactory or a conan_server refuse anonymous access;
their API takes the token the authenticate endpoint hands out, not the
password.

## Acceptance criteria

1. `credentials.json` (the last entry for a remote, a template's environment
   variable substituted), the remote's own variables and the generic ones give
   each remote its login; a template Conan alone can render gives none;
   `CONAN_HOME` moves the home; the login is no Authorization header.
2. A remote answering 401 gets the login at its authenticate endpoint, then the
   token on every request, the endpoint asked once.
3. Without a login, the 401 is the answer, and a note says Conan's
   `auth_remote.py` plugin is not run when the home has one; a remote reached
   over plain http elsewhere than this machine gets no login.

## Notes

Not read: the `auth_remote.py` plugin (a program, which Conan asks first) and
the `auth_source.py` plugin and `sources.conandata` credentials for source
downloads, which do not concern a remote; the tokens `conan remote login`
keeps in the home's `.conan.db` (an SQLite database, not read); a remote's
`force_auth`, which only saves Conan the anonymous request.
