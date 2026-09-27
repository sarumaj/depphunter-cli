//// The shop: an HTTP server rendering carts.

import gleam
import gleam/dict.{type Dict} as d
import gleam/erlang/process.{type Subject}
import gleam/http/request
import gleam/io
import gleam/json
import gleam/list.{map, type Foo}
import gleam/otp/actor
import glitch
import inventory/stock
import lustre/element.{text}
import mist
import nothere/thing
import gleam/community/ansi
import shop/cart.{type Cart, Cart as C}
import shop/missing

// import commented/out

/// An order.
pub type Order {
  Order(id: Int, items: List(String))
  @deprecated("use Order")
  Cancelled
}

pub opaque type Token {
  Token(value: String)
}

pub type Stock(a) =
  Dict(String, a)

pub const version = "1.0"

const greeting = "import not/this"

pub fn main() {
  let render = fn(x) { text(x) }
  io.println(greeting)
  list.map([1, 2], fn(n) { n + 1 })
}

fn helper(cart: Cart) -> Int {
  { 1 + 2 } * 3
}
