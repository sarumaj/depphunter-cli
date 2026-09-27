module Shop.Domain.Cart

open Shop.Domain
open Shop.Domain.Later
open Newtonsoft.Json

(* open Commented.Out (* nested *) "*)" still a comment *)
// open LineComment
let private banner = "open NotAnOpen"
let verbatim = @"C:\temp\ ""open Quoted"""
let triple = """open Triple "x" """
let interpolated name = $"hello {name} {Pricing.total []} open Interpolated"
let quote = '"'
let brace = '{'
let id<'a> (x: 'a) : 'a = x

type Cart(items: Item list) =
    member this.Items = items
    member _.Total = Pricing.total items
    static member Empty = Cart([])
    override this.ToString() = JsonConvert.SerializeObject items

let empty = Cart.Empty
let summed = add zero 1m
let (|Big|Small|) (c: Cart) = if c.Total > 10m then Big else Small
let later = Later.value
