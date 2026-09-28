---
id: REQ-OBJC-015
title: Xcode header search paths
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The include resolution of the Objective-C and C/C++ plugins **shall** search,
after the includer's directory and a compilation database's include path, the
`HEADER_SEARCH_PATHS` and `USER_HEADER_SEARCH_PATHS` of every Xcode project
whose directory holds the includer, innermost project first, read from the
build settings of its `project.pbxproj` (every configuration and condition)
and from `.xcconfig` files. An `.xcconfig` file **shall** count for the
projects whose file references name it, else for the project in its directory
or the nearest one above, else for the repository's root; one that another
includes **shall** count only through that one. `$(SRCROOT)`, `$(PROJECT_DIR)`
and `$(SOURCE_ROOT)` (also written `${...}`) **shall** stand for the project
directory, a relative entry **shall** be taken from it, `$(inherited)` **shall**
add nothing, other variables **shall** be expanded from the settings of the same
project file or `.xcconfig` chain, quoted entries **shall** keep their spaces,
and an entry ending in `/**` **shall** also search the directories below it,
the shallowest match first. An entry **shall** be dropped when it names an
absolute path, a directory outside the repository or one the repository lacks,
or a variable that cannot be expanded.

## Rationale

Xcode projects rarely have a compilation database; their include path is in
the build settings, and a header two directories hold is otherwise either
guessed by name or not found.

## Acceptance criteria

1. With `"$(SRCROOT)/Vendor/**"` in the project file, `<Lib/Lib.h>` resolves
   to `Vendor/x/Lib/Lib.h` although `Other/Lib/Lib.h` exists; without it, to
   the directory `Other/Lib`.
2. `"\"$(PROJECT_DIR)/Lib Headers\""`, `"$(THIRD)/api"` with `THIRD` set beside
   it, a conditional `"HEADER_SEARCH_PATHS[sdk=iphoneos*]"`, a relative
   `USER_HEADER_SEARCH_PATHS` and an `.xcconfig` entry whose setting comes from
   an included file are all searched.
3. `/usr/include`, `$(SRCROOT)/../outside`, `$(BUILT_PRODUCTS_DIR)/include`,
   `$(SRCROOT:dir)/x`, a missing directory and a path after `//` in an
   `.xcconfig` are not.
4. A file below a nested project searches that project's paths first, then
   the root project's; a file of the root project does not search the nested
   one's, nor those of an `.xcconfig` only the nested project names.
5. Project files and `.xcconfig` files cut at any point, include cycles and
   self-referencing settings give no failure.
