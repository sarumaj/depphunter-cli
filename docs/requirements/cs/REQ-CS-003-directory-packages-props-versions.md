---
id: REQ-CS-003
uuid: 8462a63c-6727-48e8-9a38-be91a140bb99
title: Central versions from Directory.Packages.props
scope: cs
type: functional
priority: must
status: implemented
verification:
  - unit
---

## Statement

The C# plugin **shall** take the version of a `<PackageReference>` that declares
none from the `<PackageVersion>` entry of the same package id in
`Directory.Packages.props` (central package management).

## Rationale

With central package management the project files carry no versions at all.

## Acceptance criteria

1. `<PackageReference Include="Serilog" />` with
   `<PackageVersion Include="Serilog" Version="3.1.1" />` resolves to `Serilog`
   `3.1.1`.

## Notes

The central version is looked up by the exact spelling of the id; an id cased
differently in the project file and in `Directory.Packages.props` finds no
version (see REQ-CS-004).
