module Shared.Format exposing (price)


price : Int -> String
price cents =
    String.fromInt cents
