effect module UiKit.Button where { command = MyCmd } exposing (button)

import Elm.Kernel.Kit
import Elm.Kernel.Nope
import Elm.Kernel.Utils
import Html
import UiKit.Missing


button =
    Html.button [] []
