-module(erl_handler).
-export([init/2]).
-include("erl_app.hrl").

init(Req, State) ->
    {ok, cowboy_req:reply(200, #{}, <<"hi">>, Req), State}.
