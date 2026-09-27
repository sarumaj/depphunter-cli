package beam

// cSpell: words: behaviour

import "strings"

// elixirStd are the top-level modules Elixir ships (the elixir, logger, ex_unit,
// mix, iex and eex applications), the pseudo-modules protocols are implemented
// for, and Hex, the client Mix installs. A module reference whose first segment is one of
// them, and that the project does not define, goes to the elixir-std island named
// by that segment.
//
// Implements: REQ-BEAM-007
var elixirStd = setOf(`Access Agent Application ArgumentError ArithmeticError Atom
BadArityError BadBooleanError BadFunctionError BadMapError BadStructError Base
Behaviour Bitwise Calendar CaseClauseError Code CompileError CondClauseError Config
Collectable Date DateTime Dict Duration DynamicSupervisor EEx Enum Enumerable
ErlangError Exception ExUnit File Float Function FunctionClauseError GenEvent GenServer
HashDict HashSet IEx IO Inspect Integer JSON Kernel KeyError Keyword List Logger Macro
MapSet Map MatchError MismatchedDelimiterError Mix Module NaiveDateTime Node OptionParser
PartitionSupervisor Path Port Process Protocol Range Record Regex Registry RuntimeError
Set Stream String StringIO Supervisor SyntaxError System SystemLimitError Task Time
TokenMissingError TryClauseError Tuple URI UndefinedFunctionError
UnicodeConversionError Version WithClauseError Any BitString PID Reference Hex`)

// elixirApps are Elixir's own applications, as an application list names them, and
// the island package each is shown as.
var elixirApps = map[string]string{
	"elixir": "elixir", "logger": "Logger", "ex_unit": "ExUnit", "mix": "Mix", "iex": "IEx", "eex": "EEx",
}

// otpApps are the applications Erlang/OTP ships, for -include_lib("app/...") and
// application lists.
//
// Implements: REQ-BEAM-007
var otpApps = setOf(`asn1 common_test compiler crypto debugger dialyzer diameter edoc
eldap erl_docgen erl_interface erts et eunit ftp hipe inets jinterface kernel megaco
mnesia observer odbc os_mon otp_mibs parsetools public_key reltool runtime_tools sasl
snmp ssh ssl stdlib syntax_tools tftp tools wx xmerl`)

// otpModules are the modules of Erlang/OTP code commonly calls; otpPrefixes cover the
// families named by a prefix (ssl_*, mnesia_*, ...).
//
// Implements: REQ-BEAM-007
var otpModules = setOf(`application array atomics base64 beam_lib binary c calendar
code compile counters cover cprof crypto dbg dets dict digraph digraph_utils disk_log
epp erl_anno erl_ddll erl_epmd erl_error erl_eval erl_features erl_lint erl_parse
erl_pp erl_prettypr erl_scan erl_syntax erl_tar erlang erpc error_handler error_logger
escript ets eunit eprof file file_sorter filelib filename fprof gb_sets gb_trees
gen_event gen_fsm gen_server gen_statem gen_tcp gen_udp gen_sctp global global_group
heart http_uri httpc httpd inet inet_parse inet_res init io io_lib json lcnt lists
logger make maps math mnesia ms_transform net_adm net_kernel observer orddict ordsets
os peer persistent_term pg pg2 pool prim_file proc_lib proplists public_key qlc queue
rand random re release_handler rpc sets shell slave socket sofs ssh ssl string
supervisor supervisor_bridge sys systools timer unicode uri_string xref zip zlib
argparse auth ct inets leex yecc sasl eldap memsup cpu_sup disksup odbc win32reg int
unicode_util dialyzer edoc erl_boot_server erl_prim_loader erts_debug file_io_server
instrument msacc net os_mon seq_trace snmp tftp ftp user_drv wx xmerl
appmon_info cerl`)

var otpPrefixes = []string{"asn1", "beam_", "code_", "ct_", "cth_", "dbg_", "diameter", "dialyzer_", "edoc_", "eldap_",
	"erl_", "error_logger_", "erts_", "eunit_", "gen_", "httpc_", "httpd_", "inet_", "io_lib", "logger_", "megaco", "mnesia_",
	"observer_", "prim_", "public_key", "release_handler", "sasl_", "systools", "snmp", "ssh_", "ssl_", "tls_", "dtls_", "wx", "xmerl"}

// otpModule reports whether an Erlang module is one of OTP's.
func otpModule(m string) bool {
	if otpModules[m] {
		return true
	}
	for _, p := range otpPrefixes {
		if strings.HasPrefix(m, p) {
			return true
		}
	}
	return false
}

// moduleAliases name the Hex package of a module prefix the package name does not
// spell out. The longest matching prefix wins; it is used when the project knows
// the package, and names an unknown one otherwise.
//
// Implements: REQ-BEAM-008
var moduleAliases = map[string]string{
	"Ecto.Adapters.SQL":      "ecto_sql",
	"Ecto.Adapters.Postgres": "ecto_sql",
	"Ecto.Adapters.MyXQL":    "ecto_sql",
	"Ecto.Adapters.Tds":      "ecto_sql",
	"Ecto.Migration":         "ecto_sql",
	"Ecto.Migrator":          "ecto_sql",
	"Phoenix.LiveViewTest":   "phoenix_live_view",
	"Phoenix.Component":      "phoenix_live_view",
	"Phoenix.LiveComponent":  "phoenix_live_view",
	"Phoenix.ConnTest":       "phoenix",
	"Phoenix.ChannelTest":    "phoenix",
	"Bcrypt":                 "bcrypt_elixir",
	"Argon2":                 "argon2_elixir",
	"Pbkdf2":                 "pbkdf2_elixir",
	"Plug.Adapters.Cowboy":   "plug_cowboy",
	"Absinthe.Relay":         "absinthe_relay",
	"Mix.Tasks.Phx":          "phoenix",
	"Mix.Tasks.Ecto":         "ecto",
	"Credo.Check":            "credo",
	"ExUnitProperties":       "stream_data",
	"Cluster":                "libcluster",
}

// erlangAliases name the package of Erlang module prefixes that differ from it.
var erlangAliases = map[string]string{
	"cow_": "cowlib", "ranch_": "ranch", "hackney_": "hackney", "jiffy": "jiffy",
	"certifi": "certifi", "ssl_verify_": "ssl_verify_fun", "idna": "idna",
	"mimerl": "mimerl", "gun_": "gun", "hex_": "hex_core", "jsx": "jsx", "lager": "lager",
}

func setOf(words string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(words) {
		out[w] = true
	}
	return out
}

// fold is the key module prefixes and package names are compared by: lower case,
// without underscores and dots, so Phoenix.LiveView meets phoenix_live_view and
// WebSockAdapter websock_adapter.
func fold(s string) string {
	s = strings.ToLower(s)
	return strings.NewReplacer("_", "", ".", "").Replace(s)
}

// underscore is Macro.underscore for one alias segment: HTTPoison -> http_poison,
// LiveView -> live_view.
func underscore(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUpper(c) {
			prevLower := i > 0 && (s[i-1] >= 'a' && s[i-1] <= 'z' || isDigit(s[i-1]))
			nextLower := i > 0 && isUpper(s[i-1]) && i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z'
			if prevLower || nextLower {
				b.WriteByte('_')
			}
			b.WriteByte(c + 'a' - 'A')
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
