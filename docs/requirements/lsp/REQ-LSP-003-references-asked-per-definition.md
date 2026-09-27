---
id: REQ-LSP-003
uuid: 82029ad4-bfbc-4066-a08f-9823eaf2dcbb
title: References asked per definition
scope: lsp
type: functional
priority: must
status: implemented
verification:
  - integration
  - unit
---

## Statement

The system **shall** send `textDocument/references` (without the declaration)
for every symbol definition of the graph in a file the server covers, at the
position of the symbol name on its definition line.

## Rationale

The graph already knows every definition; asking about each one gives the
complete usage graph.

## Acceptance criteria

1. References to a method, a type in a composite literal, a function across
   packages and a function in a variable declaration are all found
   (TestGoplsReferences).
2. A symbol is looked up by the word it is written as: `Type.method` as
   `method`, `init@12` as `init`, an Elixir `Shop.Cart.total/2` as `total`,
   an Objective-C `Cart.initWithItems:total:` as `initWithItems`, a
   category `NSString(Shop)` as `NSString`, a Clojure defmethod
   `area :circle` as `area` and a Bazel target `//src:shop` as `shop`.
