module Shop.Web.Program

open System
open System.Text.Json
open Microsoft.FSharp.Collections
open FSharp.Control
open Argu
open Newtonsoft.Json.Linq
open Shop.Domain
open type Shop.Domain.Cart.Cart
open Fake.Core.TargetOperators
open Mystery.Lib
module P = Shop.Domain.Pricing

[<EntryPoint>]
let main argv =
    let c = Cart.empty
    let eur = Currency.EUR
    printfn "%s %M" (Views.page "shop") (P.total c.Items)
    0
