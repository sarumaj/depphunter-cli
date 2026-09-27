---
id: REQ-OBJC-004
uuid: 61810dcd-3599-4fd5-ad38-889469029620
title: Includes resolved as C includes
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

An `#import` or `#include` that is not an Apple framework's **shall** resolve
as the C/C++ plugin resolves an include (REQ-CPP-004, REQ-CPP-005): to a
project file (the includer's directory for a quoted one, a compilation
database's include path, the conventional directories, a unique file whose
path ends in it), to the C and C++ standard libraries and system headers, and
otherwise to a pod (REQ-OBJC-006) before a vcpkg or Conan package or a
`c-external` library. A quoted bare header the project does not have and no
pod is named after is dropped.

## Rationale

Objective-C includes C headers the way C does; one resolver keeps the two
plugins' edges consistent and reuses the compilation database.

## Acceptance criteria

1. `#import "Models/Cart.h"` resolves to `App/Models/Cart.h`, `#include
   <stdio.h>` to `c-std`, `#include <vector>` in a `.mm` file to `cpp-std`, and
   `#import "Generated.h"` is dropped.
