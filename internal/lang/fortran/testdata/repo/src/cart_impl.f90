submodule (shop_cart) cart_impl
  implicit none
contains
  module procedure checkout
    c%n = 0
  end procedure checkout
end submodule cart_impl
