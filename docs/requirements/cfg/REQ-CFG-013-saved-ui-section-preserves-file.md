---
id: REQ-CFG-013
uuid: c9b125cf-5327-4bf8-8ff5-78475a438854
title: Save preserves the rest of the config file
scope: cfg
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

When saving, the system **shall** replace only the `ui:` section of the project
config file, including the filter keys `hide_languages`, `hide_islands` and
`path_filter`, editing the YAML node tree so that the other keys, their order
and their comments are preserved; it **shall** create the file or the section
when absent, and **shall** remove a saved filter the current view no longer has.

## Rationale

The project file is written by people as well as by the map; a save must not
lose what they wrote.

## Acceptance criteria

1. A file with a leading comment, an `exclude` key with a line comment and a
   `ui:` section keeps the comment, the key and its comment after a save.
2. The saved `ui:` section holds the new values, and a previously saved
   `path_filter` absent from the view is removed.
3. Saving into a directory without a project file creates it.
