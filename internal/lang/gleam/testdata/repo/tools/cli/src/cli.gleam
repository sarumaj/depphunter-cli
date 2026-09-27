import argv
import birl/duration
import gleam/io
import gleam/string
import shop
import shop/cart

pub fn main() {
  io.println(string.inspect(argv.load()))
}
