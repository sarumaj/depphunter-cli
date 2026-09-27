module Main where

import Shop.Cart (checkout)
import Options.Applicative
import "conduit" Data.Conduit
import System.Win32.Console
import Data.Unknown.Thing
import Numeric.Natural (Natural)

main :: IO ()
main = checkout mempty
