port module Main exposing (main)

{-| The shop's entry point.

    import Fake.InDocComment

{- nested {- deeper -} still comment
import Fake.Nested
-}
-}

import Browser
import Dict exposing (Dict)
import Glitter.Sparkle as Sparkle
import Html exposing (..)
import Html.Attributes as A exposing (class)
import Html.Styled
import Json.Decode as D exposing (Decoder, field)
import Json.Decode.Pipeline exposing (required)
import List.Extra
import Markdown
import Mystery.Thing
import Shared.Format exposing (price)
import Shop.Cart as Cart exposing (Cart, Item(..))
import Widget.Button


port sendMessage : String -> Cmd msg


port messageReceiver : (String -> msg) -> Sub msg


help : String
help =
    """
import Fake.InString
type Fake = Nope
"""


shader =
    [glsl|
import Fake.InShader
void main () {}
|]


quote =
    '"'


main =
    Browser.sandbox { init = Cart.empty, update = \_ m -> m, view = \_ -> text help }
-- import Fake.InLineComment
