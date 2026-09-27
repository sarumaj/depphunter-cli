import gleeunit
import gleeunit/should
import helpers
import shop
import shop/cart
import support/fixtures

pub fn main() {
  gleeunit.main()
}

pub fn total_test() {
  cart.new() |> cart.total |> should.equal(0)
}
