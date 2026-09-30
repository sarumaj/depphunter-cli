---
id: REQ-LANG-032
title: YAML anchors read as what they name, with bounded work
scope: lang
type: constraint
priority: must
status: implemented
verification:
  - unit
---

## Statement

Every plugin that reads a YAML manifest through node trees **shall** read an
alias (`*name`) as the value its anchor (`&name`) names, wherever the value
itself would be read. Reading a manifest **shall** end, and take time
bounded by a polynomial in its size, whatever its anchors do: an alias to
the mapping or sequence that contains it, a merge key (`<<`) that names its
own mapping directly or through other anchors, or aliases nested so that
their full expansion grows exponentially ("billion laughs"). An alias or a
merge that would lead back to where the reading started **shall** read as
absent.

## Rationale

A repository is untrusted input. yaml.v3 limits alias expansion only when it
decodes into Go values; into a `yaml.Node` it keeps aliases as pointers, so
the walks over the tree are what can loop or multiply. Before this, a
Compose file with `a: &x {<<: *x}` made the docker plugin recurse until the
stack overflowed, which ended the whole run. Anchors are also an ordinary way
to share a version or a source between entries; a plugin that ignored them
reported those entries without the shared value.

## Acceptance criteria

1. An anchored value is read in a buf.yaml (deps), a pubspec.yaml
   (a dependency's constraint), a spago.yaml (a dependency's range), a
   shard.yml (a dependency's version), a stack.yaml (an extra-dep's commit),
   a .fixtures.yml (a repository's URL) and a META.yml (a requirements
   section), as the same value written in place is.
2. A Compose file and an hpack package.yaml with a self-referencing merge
   key, an alias to its own container, and an alias cycle through two
   anchors are read without a crash, with every entry that does not depend on
   the cycle read as usual.
3. A document of nested aliases whose full expansion has more than a billion
   nodes, and one of nested merge keys of the same growth, are read within a
   second.

## Notes

`internal/lang/yamlnode` holds the rule: Resolve stops after a few hops,
Merged merges each mapping at most once per call and reads one it is still
merging as empty, and no other helper descends more than one level. A plugin
that recurses through values (hpack's `when:`) remembers the nodes it has
visited.
