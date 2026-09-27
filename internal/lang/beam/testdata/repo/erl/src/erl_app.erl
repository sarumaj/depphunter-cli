%% @doc The application: "strings like cowboy:start() are not calls".
-module(erl_app).
-behaviour(application).

-include("erl_app.hrl").
-include_lib("kernel/include/logger.hrl").
-include_lib("cowlib/include/cow_inline.hrl").
-include_lib("erl_app/include/erl_app.hrl").
-include("generated.hrl").

-export([start/2, stop/1]).
-export_type([mode/0]).

-type mode() :: fast | slow.
-opaque token() :: binary().

start(_Type, _Args) ->
    Dispatch = cowboy_router:compile([{'_', [{"/", erl_handler, []}]}]),
    {ok, _} = cowboy:start_clear(http, [{port, 8080}], #{env => #{dispatch => Dispatch}}),
    Body = jsx:encode(#{<<"ok">> => true}),
    ec_file:copy("a", "b"),
    Hash = crypto:hash(sha256, Body),
    try erl_worker:run(Hash) of
        ok -> ?MODULE:stop(ok)
    catch
        throw:not_found -> ok;
        error:Reason -> {error, Reason}
    end,
    Fun = fun lists:reverse/1,
    Fun([$a, $:, $b]),
    {ok, self()}.

stop(_State) ->
    ok;
stop(extra) ->
    ok.

helper(X, Y) ->
    'Elixir.Jason':encode(X), unknown_mod:call(Y).
