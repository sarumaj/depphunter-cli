{-# LANGUAGE PackageImports #-}
module Shop.Cart (Cart, add, checkout) where

import {-# SOURCE #-} Shop.Types (Item)
import Shop.Types
import qualified Data.Map.Strict as M
import Data.Text (Text)
import qualified Data.Text as T
import Control.Monad.State
import Data.Aeson (ToJSON)
import Data.Hashable (hash)
import Control.DeepSeq (NFData)
import Network.HTTP.Client
import Data.Hermes (decode)
import Data.List (sortOn)
import qualified "containers" Data.Set as Set
import "this" Shop.Parser
import Paths_shop_core (version)
import Shop.Parser
import Main

type Cart = M.Map Text Int

add :: Item -> Cart -> Cart
add _ = id

checkout :: Cart -> IO ()
checkout = mapM_ print . M.toList
