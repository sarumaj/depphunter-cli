{- A file importing every way Dhall can:
   ./commented.dhall is not an import. -}
let Prelude =
      https://prelude.dhall-lang.org/v23.0.0/package.dhall
        sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef

let k8s =
      https://raw.githubusercontent.com/dhall-lang/dhall-kubernetes/master/1.25/package.dhall

let Map = https://raw.githubusercontent.com/dhall-lang/dhall-lang/v17.0.0/Prelude/Map/Type

let Util =
        ../lib/util.dhall
      ? https://example.com/dhall/v1.2.0/util.dhall sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef

let Schema = ./schema.dhall

let fromEnv = env:DHALL_LOCAL ? ./missing.dhall ? /etc/dhall/x.dhall ? ~/x.dhall

let readme = ../README.md as Text

let here = ./schema.dhall as Location

let private = https://example.org/private/config.dhall using ./headers.dhall

let text = "./notanimport.dhall ${"x"}"

let quoted = ''
    ../lib/fake.dhall
    ''

let mk = \(x : Text) -> x

let Config = { name : Text }

in  { app = Schema::{ name = "x" }, readme, version = Util.version }
