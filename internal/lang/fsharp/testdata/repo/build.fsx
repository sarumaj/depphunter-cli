#!/usr/bin/env -S dotnet fsi
#r "nuget: Fake.Core.Process, 6.0.0"
#r "nuget: FSharp.Data"
#r "nuget: Serilog, 3.*"
#I "scripts"
#load "helpers.fsx"
#load "missing.fsx"
#r "packages/Argu/lib/netstandard2.0/Argu.dll"
#r "System.Xml.Linq"
#r "bin/Debug/Shop.dll"
#if INTERACTIVE
#r "nuget: Expecto"
#endif

open Fake.Core
open FSharp.Data
open Serilog

let run () = Helpers.greet "build"
