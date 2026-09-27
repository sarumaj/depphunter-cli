namespace Shop.Domain

open System

/// A line of a cart.
type Item = { Sku: string; Quantity: int }

type Status =
    | Open
    | Paid of DateTime

[<AutoOpen>]
module Money =
    let add (a: decimal) (b: decimal) = a + b
    let zero = 0m

    type Currency =
        | EUR
        | USD

type IPriced =
    abstract Price: decimal

exception OutOfStock of string
