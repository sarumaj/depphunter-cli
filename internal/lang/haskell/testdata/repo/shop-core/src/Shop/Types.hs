{-# LANGUAGE PatternSynonyms #-}
{-# LANGUAGE TypeFamilies #-}
{- A module header written at the margin, {- a nested comment -},
import Not.An.Import
-}
module Shop.Types
( Item (..)
, Price
, pattern Free
) where

#if MIN_VERSION_base(4,18,0)
import Data.Kind (Type)
#else
import Data.Kind (Type)
#endif
import GHC.Generics (Generic)

-- | An item. The string below is not an import.
data Item = Item
  { itemName  :: String
  , itemPrice :: Price
  } deriving (Show, Generic)

newtype Price = Price Int

type Label = String

data a :+: b = L a | R b

type family Elem c :: Type

type instance Elem [e] = e

class (Show a) => Priced a where
  price :: a -> Price
  discount, surcharge :: a -> Int
  default discount :: a -> Int
  discount _ = 0

instance Priced Item where
  price = itemPrice

instance {-# OVERLAPPABLE #-} Show a => Show (Priced' a) where
  show _ = "import Fake"

deriving instance Eq Price

pattern Free :: Price
pattern Free = Price 0

label :: Item -> Label
label i = itemName i ++ ['\'', '"']

(<+>) :: Price -> Price -> Price
Price a <+> Price b = Price (a + b)

total !xs = foldr (<+>) Free xs

foldl' = undefined

makeLenses ''Item
