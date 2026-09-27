let upstream =
      https://github.com/purescript/package-sets/releases/download/psc-0.15.0-20220507/packages.dhall
        sha256:cf54330f3bc1b25a093b69bff8489180c954b43668c81288901a2ec29a08cc64

let overrides =
      { forked =
        { dependencies = [ "prelude", "maybe" ]
        , repo = "https://github.com/acme/purescript-forked.git"
        , version = "0123456789abcdef0123456789abcdef01234567"
        }
      }

in  upstream // overrides
  with dom-indexed =
    { repo = "https://github.com/purescript-halogen/purescript-dom-indexed.git"
    , version = "v11.0.0"
    , dependencies = upstream.dom-indexed.dependencies
    }
  with widgets =
    { repo = "../packages/widgets", version = "", dependencies = [ "prelude" ] }
