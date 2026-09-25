# depphunter requirements specification

This directory holds the normative requirements of depphunter. Each requirement
is a single file, written in formal language, identified by a human-readable
identifier and a UUID, and traced to the source code that implements it and the
tests that verify it.

## Purpose and scope of the product

depphunter is a command-line tool that analyzes the project in a directory and
presents an interactive, isometric "archipelago" map of its code base in the
default web browser. The product has the following goals:

1. It shall enable a user unfamiliar with a repository to establish, within one
   minute, what the repository contains, how large its parts are, what depends
   on what, and what it draws in from outside.
2. It shall allow the user to determine interactively what is shown, and in how
   much detail, without re-running the tool.
3. It shall support any language ecosystem through a pluggable analysis layer.
4. It shall be distributed as a single, pure-Go, cross-platform binary that
   operates offline.

The following are outside the scope of the product at present:

- precise symbol-level call graphs, other than through the optional language
  server layer ([REQ-LSP-001](lsp/REQ-LSP-001-symbol-references-are-opt-in.md));
- editing or refactoring code, and running builds;
- serving the view to remote users; the server is local-only by design
  ([scope `sec`](sec/)).

## Keywords

The keywords **shall**, **shall not**, **should**, **should not** and **may**
are to be interpreted as described in RFC 2119 and RFC 8174 when, and only
when, they appear in bold.

## Requirement file template

Every requirement is one Markdown file under a directory named after its scope,
named `<ID>-<slug>.md`, and follows [TEMPLATE.md](TEMPLATE.md):

```markdown
---
id: REQ-SRV-004
uuid: 5b0e5c8e-6f0c-4a39-9a2e-0d8e1f1d8c7a
title: Graph document carries an entity tag
scope: srv
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

The server **shall** ...

## Rationale

...

## Acceptance criteria

1. ...

## Notes

...
```

### Front matter fields

| Field           | Meaning                                                                                 |
|-----------------|-----------------------------------------------------------------------------------------|
| `id`            | `REQ-<SCOPE>-<NNN>`. Stable, never reused. Used in code annotations.                    |
| `uuid`          | Random (version 4) UUID. Immutable, survives a renumbering or a move to another scope.  |
| `title`         | A short noun phrase naming the capability or constraint.                                |
| `scope`         | The functional area; one of the scopes below, equal to the directory name.              |
| `type`          | `functional`, `non-functional`, `interface`, `constraint` or `limitation`.              |
| `priority`      | `must`, `should` or `may`, matching the strongest keyword of the statement.             |
| `status`        | `implemented`, `partial`, `not-implemented`, `superseded` or `withdrawn`.               |
| `superseded_by` | For `superseded` requirements, the identifier(s) of the requirement(s) that replace it. |
| `source`        | Optional. The section(s) of the user documentation that also describe the requirement.  |
| `verification`  | The kinds of test that verify the requirement; see below.                               |

### Verification (test) types

| Type          | Meaning                                                                                                                        |
|---------------|--------------------------------------------------------------------------------------------------------------------------------|
| `unit`        | Go unit tests (`go test ./...`) or Node tests of a single module, without I/O beyond fixtures.                                 |
| `integration` | Tests that exercise several components together: the HTTP server, `git`, the file system, a language server, the built binary. |
| `ui`          | Headless Node tests of the browser modules (`web/uitest/*.test.mjs`).                                                          |
| `extension`   | Node tests of the VS Code extension (`extension/test/*.test.js`).                                                              |
| `e2e`         | Manual or scripted end-to-end runs of the command and the map in a real browser.                                               |
| `manual`      | Human judgement of a visual or interactive quality that no automated test captures.                                            |
| `inspection`  | Review of source, configuration, CI workflows or release artifacts.                                                            |

### Scopes

| Scope   | Area                                                                                           |
|---------|------------------------------------------------------------------------------------------------|
| `cli`   | The command, its flags, arguments, help, version, logging and exit behavior.                   |
| `cfg`   | Configuration sources, precedence, environment variables and saved view settings.              |
| `sec`   | Security of the local server and of executed commands.                                         |
| `dist`  | Distribution, licensing, release engineering, CI and dependency maintenance.                   |
| `mod`   | The graph data model exchanged between analysis, UI, exports and extension.                    |
| `lang`  | The language plugin contract, file scanning, extraction cache and analysis pipeline.           |
| `go`    | The Go plugin.                                                                                 |
| `js`    | The JavaScript and TypeScript plugin.                                                          |
| `py`    | The Python plugin.                                                                             |
| `rs`    | The Rust plugin.                                                                               |
| `java`  | The Java plugin.                                                                               |
| `cs`    | The C# plugin.                                                                                 |
| `ps`    | The PowerShell plugin.                                                                         |
| `ci`    | The continuous-integration plugin (GitHub Actions, GitLab CI, container images).               |
| `md`    | Markdown documents as dependencies, and broken-link findings.                                  |
| `sup`   | Supply chain: pinning, transitive resolution, package indexes, private packages.               |
| `auth`  | Registry credentials.                                                                          |
| `trc`   | The resolution report.                                                                         |
| `fnd`   | Findings from scanner reports and the vulnerability database.                                  |
| `hist`  | The git history overlay.                                                                       |
| `lsp`   | Symbol references through language servers.                                                    |
| `srv`   | The local HTTP API and the event stream.                                                       |
| `watch` | Watch mode and incremental re-analysis.                                                        |
| `exp`   | Exports: JSON, DOT, GraphML, HTML, PNG and the backpack.                                       |
| `map`   | The isometric map: layout, navigation, selection, side panel, search, filters, colors, styles. |
| `ui`    | Page-level behavior: introductions, help, reconnection, menus.                                 |
| `a11y`  | Accessibility.                                                                                 |
| `perf`  | Performance.                                                                                   |
| `walk`  | Walk mode: movement, camera, pointer capture, planet, water, health and wind.                  |
| `city`  | The procedural city shared by both views: streets, ramps, bridges, vegetation, facades.        |
| `tool`  | Walk-mode tools, hands, gestures, projectiles, the tool row and the wheel.                     |
| `hunt`  | The dependency hunt: tagging, bugs, catches, beacons, tracker, backpack, photographs, HUD.     |
| `ext`   | The VS Code extension.                                                                         |

## Traceability

Code that implements a requirement carries an annotation naming it, in the
comment syntax of its language, directly above the declaration or block it
concerns:

```go
// Implements: REQ-SEC-001, REQ-SEC-002
func (s *Server) ListenAndServe() error {
```

A test that verifies a requirement carries a `Verifies:` annotation instead:

```js
// Verifies: REQ-WALK-012
it('ignores a short drop and kills on a long one', () => {
```

The same form is used in YAML and shell (`# Implements: ...`), in CSS
(`/* Implements: ... */`) and in HTML (`<!-- Implements: ... -->`). Vendored
code, generated files and test data are never annotated.

A step of a CI workflow (`.github/workflows/`) may carry `# Verifies: ...`
where the job is the automated check, such as building and running every
release target.

`node scripts/reqtrace.mjs` reads the requirement files and the annotations and
writes [TRACEABILITY.md](TRACEABILITY.md): for each requirement, where it is
implemented and where it is verified. `node scripts/reqtrace.mjs --check` fails
when an annotation names an unknown requirement, when a requirement file is
malformed or duplicates an identifier or a UUID, or when TRACEABILITY.md is out
of date.

## Known limitations

The following limitations are part of the specification and are recorded as
requirements of type `limitation` in their scopes:

- Java imports name packages rather than artifacts, so Maven dependencies are
  matched heuristically and an index cannot be asked what a Maven package
  depends on.
- GitHub Actions references from github.com and from GitHub Enterprise are one
  ecosystem.
- pip's keyring is not consulted for credentials.
- Language servers that index slowly may return fewer references within the
  time budget.
