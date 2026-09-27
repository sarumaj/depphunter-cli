---
id: REQ-OBJC-001
uuid: 6d028053-528e-45b4-9311-8fd845670d07
title: Objective-C files and CocoaPods and Carthage manifests claimed
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The Objective-C plugin **shall** analyze `.mm` files, the `.m` and `.h` files
the scan labels Objective-C, `Podfile`, `*.podspec`, `*.podspec.json`,
`Cartfile` and `Cartfile.private`. The scan **shall** label a `.m` file
Objective-C only when a line of its first 8000 bytes starts with a
preprocessor directive, a `//` comment or an Objective-C keyword
(`@interface`, `@implementation`, `@protocol`, `@class`, `@import`, `@end`),
else Mercury when a line starts with `:-`, else MATLAB; and it **shall** label
a `.h` file Objective-C when a line of its head starts with `#import` or one of
those keywords. The C/C++ plugin **shall** leave `.h` files labelled
Objective-C to this plugin.

## Rationale

`.m` is also MATLAB's and Mercury's extension, and neither writes a line
starting with `#` or `//`, while an Objective-C file without an `#import` in its
head is all but unknown. A `.h` file may be C, C++ or Objective-C; one written in
Objective-C would give the C++ grammar nothing to read, and its `#import` lines
are what the C/C++ plugin skips. The scan already reads the head to tell binary
files apart, so telling these apart costs no extra read.

## Acceptance criteria

1. A `.m` file starting `//  Cart.m` or `#include <stdio.h>` is Objective-C;
   one starting `% SMOOTH` or with MATLAB code only is MATLAB; one with
   `:- module solver.` is Mercury; neither of the last two is analyzed.
2. A `.h` file with `#import <Foundation/Foundation.h>`, `@protocol` or
   `@class` is Objective-C and analyzed by this plugin, not by the C/C++
   plugin; a C header with only `#ifndef` guards stays C and the C/C++
   plugin's.
3. `.mm`, `Podfile`, `.podspec`, `.podspec.json` and `Cartfile` are
   analyzed; `Podfile.lock` and `Cartfile.resolved` are read by the resolver
   but not analyzed. A `Podfile` and a `Cartfile` do not share an extraction
   cache class.
