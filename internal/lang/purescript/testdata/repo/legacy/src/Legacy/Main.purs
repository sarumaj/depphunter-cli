module Legacy.Main where

import Prelude
import Control.Monad.State (State)
import DOM.HTML.Indexed (HTMLdiv)
import Data.Maybe (Maybe)
import Forked.Thing (thing)
import Legacy.Util (helper)
import Widgets.Button (button)

run :: Int
run = helper
