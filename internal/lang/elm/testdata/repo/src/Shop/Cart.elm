module Shop.Cart exposing (Cart, Item(..), empty, total, (|>>))

import Shared.Format


type Item
    = Book String
    | Toy { name : String, price : Int }
    | Gift (List Item)


type alias Cart =
    List Item


empty : Cart
empty =
    []


total cart =
    List.length cart


infix left 0 (|>>) = andThen


andThen a f =
    f a
