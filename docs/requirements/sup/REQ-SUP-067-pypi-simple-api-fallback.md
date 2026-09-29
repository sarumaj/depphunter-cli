---
id: REQ-SUP-067
title: PyPI Simple API fallback and flat indexes with PEP 658 metadata
scope: sup
type: functional
priority: must
status: implemented
verification:
  - unit
  - integration
---

## Statement

When a PyPI index other than PyPI itself answers the JSON API request
([REQ-SUP-023](REQ-SUP-023-pypi-requires-dist.md)) with 404 or 410, or with
something that is not JSON, the index client **shall** ask the index's Simple
API instead: the project page `<index>/<normalized name>/` (PEP 503 name
normalization, the index URL as configured, so `/simple` stays), sent with
`Accept: application/vnd.pypi.simple.v1+json,
application/vnd.pypi.simple.v1+html;q=0.2, text/html;q=0.01`, read as PEP 691
JSON when the answer's media type is JSON and as PEP 503 HTML otherwise, with
each file's PEP 658 / PEP 714 metadata marker (`core-metadata`,
`dist-info-metadata`, `data-core-metadata`, `data-dist-info-metadata`) and
PEP 592 yanked marker.

It **shall** take the release from the wheel and source archive filenames of
the distribution asked about: the pinned version when there is one, yanked or
not; otherwise the newest release by PEP 440 order that is neither yanked nor
a pre-release or development release, and the newest non-yanked pre-release
only when there is no other. It **shall** read the dependencies from the
`Requires-Dist` headers of `<file URL>.metadata` of a file of that release that
advertises one, a wheel's before a source archive's, leaving out requirements
guarded by an `extra ==` marker as the JSON API path does, and **shall** refuse
metadata whose SHA-256 differs from the one the page gives.

File URLs **shall** be resolved against the URL the page was served from, after
redirects (and its `<base href>`). Every request **shall** carry only the
credential the credential store holds for its own URL
([REQ-SUP-033](REQ-SUP-033-index-requests-carry-host-credentials.md)); a
Python index's credential serves the index URL without `/simple` (devpi's
`/+simple`), which covers files and metadata below that path.

A flat index ([REQ-SUP-066](REQ-SUP-066-python-tool-indexes.md)) **shall** be
read the same way from the one page it is, as configured, read once per run
and holding every project's files: its filenames give the releases of the
distribution asked about, and a file's metadata is its advertised
`<file URL>.metadata`. A flat index that is a directory this machine's own
configuration names **shall** be read from disk: its top-level files, a
file's metadata being the `<file>.metadata` beside it or, for a wheel, the
`METADATA` of the wheel's own `.dist-info` directory. A flat index without the
distribution **shall** pass the question on; a release it has without
readable metadata (a local release with only source archives) is no answer
with a reason, as above.

## Rationale

GitLab, AWS CodeArtifact, Azure Artifacts, Google Artifact Registry, devpi and
plain Nexus or Artifactory repositories serve pip's Simple API and nothing at
the Warehouse JSON API, so `--online` asked them and got nothing. PEP 658 puts
a release's metadata beside its files so that a client does not have to
download an archive to learn its requirements.

## Acceptance criteria

1. Against a stub whose JSON API answers 404, a PEP 691 page with
   `core-metadata` hashes gives the `Requires-Dist` of the newest final,
   non-yanked wheel, with extras left out; the page is asked with the PEP 691
   `Accept` header, relative file URLs resolve against the redirected page, and
   the answer is cached.
2. A PEP 503 HTML page with `data-dist-info-metadata`, `data-core-metadata` and
   `data-yanked` gives the pinned release's dependencies even when that release
   is yanked, and the newest one's when unpinned.
3. A release with no metadata advertised, or advertised and missing, is no
   answer with a reason in the report, no archive is downloaded and the next
   index is not asked; a page that is missing, or lacks the pinned release, moves
   on to the next index.
4. A path-scoped credential goes with the page and the metadata below the
   index's path; a metadata file on another host is sent without it.
5. PyPI itself is asked only its JSON API.
6. A flat page this machine names is requested once for several questions:
   the newest final release's advertised metadata answers, a release without
   metadata gives the reason and stops, and a package the page does not list
   is asked of the next index.
7. A local flat directory gives a wheel's `METADATA` (not one deeper in the
   archive) and a source archive's `.metadata` file; a release with only a
   source archive is no answer with a reason.

## Notes

`requires-python` is not used to pick a release: the interpreter the map is
drawn for is not known. A release whose files advertise no metadata gets no
answer (the report says so): depphunter does not download wheels or source
archives to read their metadata, nor unpack a local source archive (a local
wheel's `METADATA` is read in place).
