module Test.Main where

import Prelude
import Effect (Effect)
import Shop.Cart (empty)
import Test.Helpers (check)
import Test.Spec (describe)

main :: Effect Unit
main = check empty
