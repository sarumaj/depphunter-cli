-module(erl_worker).
-behaviour(gen_server).
-import(lists, [map/2]).
-compile(export_all).

run(X) -> gen_server:call(?MODULE, {run, X}, ?TIMEOUT).

init([]) -> {ok, #state{}}.
