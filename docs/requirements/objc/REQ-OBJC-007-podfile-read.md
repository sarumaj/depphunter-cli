---
id: REQ-OBJC-007
title: Podfile pods read as imports
scope: objc
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The plugin **shall** read a Podfile's `pod` statements wherever they are
(targets, nested targets, methods), across continued lines, with both hash
syntaxes and comments stripped, as imports of the pod they name (a subspec
`Firebase/Analytics` is the pod `Firebase`): the version requirements, and the
`:git` with `:tag`, `:branch` or `:commit`, `:path` and `:podspec` options. A
name built by interpolation is not read.

## Rationale

The Podfile is where an app says what it uses; like a Gemfile it is read as
text, because running it needs Ruby and CocoaPods.

## Acceptance criteria

1. In the fixture's Podfile every `pod` line is an import: in a `def`, in
   nested targets and one spread over three lines; `pod 'LocalKit', :path =>
   'LocalKit'` resolves to `LocalKit/LocalKit.podspec`.
