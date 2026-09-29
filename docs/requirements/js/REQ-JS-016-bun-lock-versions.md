---
id: REQ-JS-016
title: bun.lock versions
scope: js
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `bun.lock`, Bun's text lock file, tolerating trailing
commas and comments. It **shall** apply the hoisted packages to the
`package.json` files in its directory and below, and each workspace's declared
dependencies to the workspace's directory, where a package installed under the
workspace's name (`ui/react`) wins over the hoisted one. A registry version, and
a git or GitHub resolution to a commit (full or abbreviated as Bun writes it),
**shall** pin the package, a full commit being its version and the repository
its origin (REQ-JS-019); workspace, link, folder and tarball entries **shall**
pin nothing. Beside another lock file of the same directory, `bun.lock`
**shall** answer only for the packages that lock file does not, the order being
`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lock`.

## Rationale

Bun 1.2 writes `bun.lock` by default; without it a Bun project's packages float.
Putting it last means reading it never changes a version another lock gave.

## Acceptance criteria

1. `react` declared `^18.2.0` at the root resolves to `18.3.1`, pinned; in the
   `ui` workspace, declared `^17.0.0`, to the `ui/react` entry's `17.0.2`.
2. `forge-std` declared `github:foundry-rs/forge-std#v1.9.4` resolves to
   `github:foundry-rs/forge-std#1eea5ba`, pinned, from
   `https://github.com/foundry-rs/forge-std`.
3. An npm alias resolves to the aliased package's version; a tarball URL keeps
   the declared URL and floats.
4. Beside a `package-lock.json` locking `react` at `18.2.0`, `react` resolves to
   `18.2.0` while `chalk`, only in `bun.lock`, is pinned by it.
5. Every prefix of the fixture's `bun.lock` is read without a panic.
