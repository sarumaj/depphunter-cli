module Shop.Cart where

import Prelude
import Data.Array (length)
import Data.Maybe hiding (fromJust)

data Item = Book String | Toy { name :: String } | Gift

newtype Cart = Cart (Array Item)

type Total = Int

class Priced a where
  price :: a -> Int
  discount :: a -> Int

instance pricedItem :: Priced Item where
  price _ = 1
  discount _ = 0

derive instance eqItem :: Eq Item
derive newtype instance showCart :: Show Cart

instance Show Item where
  show _ = "item"

empty :: Cart
empty = Cart []

total :: Cart -> Total
total (Cart xs)
  | length xs > 0 = length xs
  | otherwise = 0
