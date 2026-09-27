@external(erlang, "shop_ffi", "now")
@external(javascript, "./ffi_helpers.mjs", "now")
pub fn now() -> Int

@external(erlang, "os", "timestamp")
pub fn timestamp() -> #(Int, Int, Int)

@external(erlang, "gleam_stdlib", "identity")
pub fn coerce(a: a) -> b

@external(erlang, "gleam@list", "reverse")
pub fn reverse(list: List(a)) -> List(a)

@external(erlang, "hpack", "decode")
pub fn decode(bits: BitArray) -> a

@external(erlang, "gleam_otp_external", "identity")
pub fn otp(a: a) -> a

@external(erlang, "Elixir.Jason", "encode")
pub fn jason(a: a) -> a

@external(erlang, "made_up_nif", "go")
pub fn nif() -> Nil

@external(javascript, "../../gleam_stdlib/gleam/list.mjs", "reverse")
pub fn js_reverse(list: List(a)) -> List(a)

@external(javascript, "react", "createElement")
pub fn element(tag: String) -> a

@external(javascript, "left-pad", "leftPad")
pub fn pad(s: String) -> String
