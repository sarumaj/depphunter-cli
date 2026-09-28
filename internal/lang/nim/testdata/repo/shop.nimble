# Package

version       = "0.1.0"
author        = "Acme"
description   = "A shop"
license       = "MIT"
srcDir        = "src"
bin           = @["shop"]

# Dependencies

requires "nim >= 2.0.0", "jester >= 0.5"
requires "chronos"
requires "https://github.com/acme/nim-widgets.git#head"
requires "stew#0123abcd",
         "results == 0.4.0",
         "zippy ^= 0.10"
requires("gh:treeform/pixie#v5.0.0")
requires "https://git.acme.internal/shop/vault.git#abcdef0123456789abcdef0123456789abcdef01"
requires "sdl2_nim", "file://libs/localdep"

when defined(windows):
  requires "winim >= 3.9"

taskRequires "test", "unittest2 >= 0.2"

feature "ssl":
  requires "openssl_evp"

task test, "Runs the tests":
  exec "nim c -r tests/tshop"
  exec "nimble install -y fakepkg"
