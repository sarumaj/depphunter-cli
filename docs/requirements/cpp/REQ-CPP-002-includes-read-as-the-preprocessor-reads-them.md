---
id: REQ-CPP-002
title: Includes read as the preprocessor reads them
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** read `#include` and `#include_next` directives
line by line, removing comments and joining lines continued with a backslash,
wherever they occur in the file, and **shall** record each as `#include "x"`
or `#include <x>`. An include through a macro (`#include CONFIG_H`) and
`#import` **shall not** be recorded.

## Rationale

Directives are line-based and may sit inside any block; reading them apart
from the parser also sees includes the grammar would misread around unusual
macros. An include through a macro names nothing until the macro is expanded.

## Acceptance criteria

1. `#  include   <map>  // comment` is recorded as `#include <map>`.
2. An include inside a block comment is not recorded; a `/*` inside a string
   literal does not open a comment.
3. `#include CONFIG_HEADER` and `#import <Foundation/Foundation.h>` are not
   recorded.
