---
id: REQ-OBJC-002
uuid: e9ef01c2-1ce3-44b3-9f5d-6edc08efa7fa
title: #import, #include and @import read
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read `#import` and `#include` (`"x"` and `<x>`) with the
C/C++ plugin's preprocessor scanner, `#import` included, skipping `#if 0`
blocks, and `@import Module;` or `@import Module.Submodule;` from the source,
each with its line.

## Rationale

`#import` is an include read once; the C/C++ scanner already knows comments,
continuations and `#if 0`. `@import` is a module import in the language itself.

## Acceptance criteria

1. `#import "AppDelegate.h"`, `#import <AFNetworking/AFNetworking.h>`,
   `#include <stdio.h>` and `@import CoreData.NSManagedObject;` are imports;
   an `#import` inside `#if 0` is not.
