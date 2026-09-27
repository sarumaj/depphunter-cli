local MODREV, SPECREV = "dev", "-1"
package = "shop"
version = MODREV .. SPECREV
source = { url = "git+https://example.test/shop.git" }
dependencies = {
   "lua >= 5.1, < 5.5",
   "penlight ~> 1.5",
   "luasocket",
   "lua-cjson == 2.1.0",
   "lpeg 1.1.0",
   platforms = {
      unix = { "luaposix >= 35" },
   },
}
test_dependencies = { "busted" }
build = {
   type = "builtin",
   modules = {
      ["shop"] = "src/shop/init.lua",
      ["shop.cart"] = "src/shop/cart.lua",
      ["shop.native"] = { sources = { "csrc/native.c" } },
   },
}
