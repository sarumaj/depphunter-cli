---
id: REQ-CPP-003
title: Definitions extracted
scope: cpp
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C/C++ plugin **shall** extract, with their lines, functions (kind
`func`), member functions (`method`, named `Class.name`, whether declared in
the class or defined outside it as `Class::name`), classes, structs, unions and
enums with a body (nested ones named `Outer.Inner`), `typedef` and `using`
aliases (`type`), namespaces (`namespace`, once per file) and macros
(`macro`). It **shall not** extract a declaration inside a function body, a
header guard, the prototype of a function the same file defines, or a
definition named in capitals only (a macro invocation such as
`TEST(Suite, Case) { }`).

## Rationale

These are what other files use from a C or C++ file. A header guard is on
every header; a prototype and its definition are one function; test macros
parse as functions named `TEST`.

## Acceptance criteria

1. `void Server::start() {}` gives the method `Server.start`; a member
   `int port() const` inside `class Server` gives `Server.port`.
2. `typedef void (*callback_t)(int);` gives the type `callback_t`.
3. `#ifndef UTIL_H` / `#define UTIL_H` gives no symbol; `#define APP_MAX(a, b)`
   gives the macro `APP_MAX`.
4. `std::string s(nullptr);` in a function body gives no symbol, nor does
   `std::atomic<int> g(0);` at file scope.
5. `class API_EXPORT Widget final : public Base<int> {` gives the class
   `Widget`; `explicit operator bool() const` gives no symbol.
