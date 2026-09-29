---
id: REQ-SUP-073
title: CUE registries for module dependencies
scope: sup
type: functional
priority: should
status: implemented
verification:
  - unit
---

## Statement

With `--online`, the index client **shall** answer what a module of the `cue`
island depends on from a CUE registry, an OCI registry laid out as cue lays it
out: the manifest of `<registry>/v2/<repository prefix>/<module path>/manifests/<version>`
names the module's `module.cue` as the layer of media type
`application/vnd.cue.modulefile.v1`, and that blob alone (not the module's
archive) **shall** be read; its `deps` are returned, pinned by their versions
(REQ-CUE-012). The requests **shall** go through the OCI client (pull-token
challenges, credentials by host). A module without a version **shall not** be
asked.

registry.cue.works is the public index. `CUE_REGISTRY` **shall** be read as
cue reads it: comma-separated `<registry>` (for every module; `none` switches
the public registry off) and `<module prefix>=<registry>` entries (for the
modules under that prefix, whole path elements, the longest prefix first),
a registry being `<host>[/<repository prefix>]`, over https unless it ends in
`+insecure` or is this machine; `simple:` is the same list. These are this
machine's sources.

## Rationale

cue's module cache holds only what `cue` fetched here (REQ-CUE-011); the
registry holds every version's module.cue beside its archive.

## Acceptance criteria

1. example.com/lib v0.2.0 routed by `example.com=<host>/modules+insecure`
   yields github.com/acme/schemas v1.2.0 and cue.dev/x/k8s.io v0.4.0 from the
   manifest and the module file blob, with the `cue login` token for the host.
2. An unversioned module makes no request; the catch-all of CUE_REGISTRY
   serves every other module.
3. Prefixes match whole path elements; `none` leaves no registry; `file:`
   leaves the public one.

## Notes

A `file:` or `inline:` CUE_REGISTRY (a configuration written in CUE) is not
read, and a `<prefix>=none` entry is ignored. The sandbox proxy blocks
registry.cue.works; the layout was taken from cue's source (mod/modregistry,
internal/mod/modresolve) and verified with a stub server only.
