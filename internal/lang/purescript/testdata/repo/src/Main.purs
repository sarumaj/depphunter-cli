-- | The shop's entry point.
module Main
  ( main
  , greet
  ) where

import Prelude

import Data.Argonaut.Core as J -- not listed: unresolved
import Data.Map (Map)
import Data.Maybe (Maybe(..), fromMaybe)
import Effect (Effect)
import Effect.Aff (launchAff_)
import Effect.Console (log)
import Glitter.Sparkle (sparkle)
import Halogen.HTML as HH
import Mystery.Thing (x)
import Node.FS.Aff (readTextFile)
import Prim.Row (class Cons)
import Registry.PackageName (PackageName)
import Shop.Cart (Cart, empty) as Cart
import Widgets.Button (button)
{- import Fake.Block -}
-- import Fake.Comment

greeting :: String
greeting = "import Fake.String \" still a string"

raw :: String
raw = """
import Fake.Raw
"""

foreign import greet :: String -> Effect Unit
foreign import data Handle :: Type

main :: Effect Unit
main = do
  log greeting
  launchAff_ (pure unit)

infixr 5 append as <+>
