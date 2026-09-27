-module(legacy_parser).
-export([parse/1]).

parse(X) -> lists:reverse('Elixir.Shop.Cart':new([X])).
