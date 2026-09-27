module Test.Helpers (check) where

import Prelude

check :: forall a. a -> Effect Unit
check _ = pure unit
