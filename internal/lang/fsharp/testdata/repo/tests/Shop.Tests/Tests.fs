module Shop.Tests.Tests

open Expecto
open FsCheck
open Shop.Domain

[<Tests>]
let tests = testList "cart" [ testCase "empty" <| fun _ -> () ]
