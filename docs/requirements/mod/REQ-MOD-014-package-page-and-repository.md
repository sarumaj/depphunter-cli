---
id: REQ-MOD-014
title: Package page and repository
scope: mod
type: interface
priority: should
status: implemented
verification:
  - unit
---

## Statement

A package node **shall** carry `page`, the package's page on its ecosystem's
public index, when it resolves from that index or from none, is not `private`
and does not carry `indexUnknown`. It **shall** carry `repository`, the web
page of the repository its source lives in, when the commit it was built from
(`git`), where it was installed from (`origin`) or its name says where that is.
Both are worked out from the document's own members, without a request.

## Rationale

A dependency is looked into where it is published and where its source is, and
both are one click from the map once the document says where they are. A
package served by an index of the organization's, or by one only the
repository names, is not the public package of that name: a link to the public
site would name it there and lead to something else.

## Acceptance criteria

1. An npm package resolving from the public registry carries
   `page: "https://www.npmjs.com/package/<name>"`.
2. A package served by another index, a `private` one and one with
   `indexUnknown` carry no `page`.
3. A package with `git: "https://github.com/o/r#<commit>"` carries
   `repository: "https://github.com/o/r/tree/<commit>"`.
4. A package installed from `git+https://github.com/o/r.git` or
   `git@github.com:o/r.git`, and a Go module `github.com/o/r/v2`, carry
   `repository: "https://github.com/o/r"`.
5. A package installed from a local directory or a plain archive carries no
   `repository`.

## Notes

Nothing confirms that the page exists: a package the index has since removed
still links to where it would be.
