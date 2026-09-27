import gleam/option.{type Option, None, Some}

pub type Cart {
  Cart(items: List(#(String, Int)), coupon: Option(String))
}

pub fn new() -> Cart {
  Cart([], None)
}

@external(javascript, "../shop_ffi.mjs", "total")
pub fn total(cart: Cart) -> Int {
  0
}
