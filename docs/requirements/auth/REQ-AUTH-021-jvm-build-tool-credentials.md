---
id: REQ-AUTH-021
title: sbt, Coursier and Gradle credentials
scope: auth
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The system **shall** read the credentials the JVM build tools other than Maven
keep on this machine, each filed under the host it names:

- sbt: the `host=`, `user=` and `password=` of `SBT_CREDENTIALS`,
  `.credentials` in sbt's global directory (`-Dsbt.global.base`, else
  `~/.sbt`) and `~/.ivy2/.credentials`, whatever `realm=` they give.
- Coursier: `COURSIER_CREDENTIALS`, holding `host(realm) user:password` lines
  itself or naming a properties file (an absolute path or a `file:` URL); when
  it is unset, `credentials.properties` in Coursier's configuration directory
  (`COURSIER_CONFIG_DIR`; `%APPDATA%\Coursier\config` on Windows;
  `~/Library/Application Support/Coursier` on macOS; else
  `$XDG_CONFIG_HOME/coursier` or `~/.config/coursier`), whose
  `<name>.host`, `<name>.username` and `<name>.password` make one credential.
- Gradle: for a `maven { name = "<n>"; url = ...;
  credentials(PasswordCredentials) }` of an init script in Gradle's user home
  (`GRADLE_USER_HOME`, else `~/.gradle`: `init.gradle(.kts)` and
  `init.d/*.gradle(.kts)`), `<n>Username` and `<n>Password` from
  `gradle.properties` there, else from `ORG_GRADLE_PROJECT_<n>Username` and
  `ORG_GRADLE_PROJECT_<n>Password`.

Each is sent as Basic credentials. A host given with a scheme, a path or user
information **shall not** be taken.

## Rationale

sbt builds conventionally add `Credentials(Path.userHome / ".sbt" /
".credentials")`, Coursier-based tools (Mill, Scala CLI, sbt's own resolver)
read Coursier's file or variable, and Gradle's `PasswordCredentials`
convention reads `<name>Username`/`<name>Password` properties: that is where a
developer's or a pipeline's Nexus or Artifactory credential is for these tools.

## Acceptance criteria

1. The three sbt files each give a credential for their `host`; a `host`
   written as a URL gives none.
2. Coursier's default file, `COURSIER_CREDENTIALS` inline and naming a file
   (path and `file://` URL), and the macOS directory each give their
   credentials; an entry without a user name gives none.
3. A Gradle init script's repository asking for `PasswordCredentials` gets
   `gradle.properties`' properties before the `ORG_GRADLE_PROJECT_` ones; a
   repository that does not ask for them gets none.

## Notes

A project's own build script is not matched to Gradle's properties, nor a
project's `build.sbt` credentials: the repository would then be choosing where
the password is sent ([REQ-AUTH-012](REQ-AUTH-012-url-credential-from-machine-only.md)).
A `credentials { username = ...; password = ... }` block written out in an init
script is not read, nor `-Dgradle.user.home` in `GRADLE_OPTS`. Leiningen's
`~/.lein/credentials.clj.gpg` is not decrypted (it needs the user's GPG key);
Leiningen's `:repositories` credentials written into `profiles.clj` are not
read either. Maven's `settings-security.xml` master password is not used
([REQ-AUTH-010](REQ-AUTH-010-encrypted-password-left-alone.md)).
