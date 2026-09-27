{- A legacy spago 0.20 project. -}
{ name = "legacy"
, dependencies =
  [ "console"
  , "dom-indexed"
  , "effect"
  , "forked"
  , "maybe"
  , "prelude"
  , "transformers"
  , "widgets"
  ]
, packages = ./packages.dhall
, sources = [ "src/**/*.purs" ]
}
