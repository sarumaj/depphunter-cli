module Lib (someFunc) where

import Acme.Missiles (launchMissiles)
import qualified Data.Text as T
import Data.LeftPad (leftPad)

someFunc :: IO ()
someFunc = launchMissiles
