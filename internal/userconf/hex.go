package userconf

import (
	"cmp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------- Dart

// DartConfigDir is where the Dart SDK keeps its user configuration, as pub finds it:
// %APPDATA%\dart on Windows (when %APPDATA% is set), ~/Library/Application
// Support/dart on macOS, else $XDG_CONFIG_HOME/dart (~/.config/dart).
//
// Implements: REQ-SUP-064, REQ-AUTH-027
func (m Machine) DartConfigDir() string {
	switch m.GOOS {
	case "windows":
		if dir := m.Env("APPDATA"); dir != "" {
			return filepath.Join(dir, "dart")
		}
	case "darwin", "ios":
		return join(m.Home, "Library", "Application Support", "dart")
	}
	return join(m.xdgConfigHome(), "dart")
}

// PubTokens is the file `dart pub token add` writes its tokens to.
//
// Implements: REQ-SUP-064, REQ-AUTH-027
func (m Machine) PubTokens() string { return join(m.DartConfigDir(), "pub-tokens.json") }

// ---------------------------------------------------------------- Hex

// HexHome is the directory Mix's Hex keeps hex.config in: HEX_HOME; else, under
// MIX_XDG=1 (or true), $XDG_CONFIG_HOME/hex (~/.config/hex); else ~/.hex.
//
// Implements: REQ-SUP-064, REQ-AUTH-028
func (m Machine) HexHome() string {
	if dir := m.Env("HEX_HOME"); dir != "" {
		return dir
	}
	if v := m.Env("MIX_XDG"); v == "1" || v == "true" {
		return join(m.xdgConfigHome(), "hex")
	}
	return join(m.Home, ".hex")
}

// HexConfig is what depphunter takes from Hex's hex.config: the API it talks to, and
// the keys and tokens it authenticates with.
type HexConfig struct {
	// APIURL is `mix hex.config api_url`.
	APIURL string
	// APIKey is `mix hex.config api_key`: a user or organization key, as written.
	APIKey string
	// OAuth is the token `mix hex.user auth` stores ($oauth_token).
	OAuth HexToken
	// Repos holds each repository's key and token by repository name
	// ("hexpm:acme"), as `mix hex.organization auth acme --key KEY` stores it.
	Repos map[string]HexRepo
}

// HexToken is an OAuth access token and when it expires (Unix seconds; 0 unknown).
type HexToken struct {
	Access  string
	Expires int64
}

// HexRepo is one repository of hex.config's $repos, or of rebar3's hex.config.
type HexRepo struct {
	// APIKey is an API key for this repository alone (rebar3's api_key).
	APIKey string
	// AuthKey is Mix's auth_key, rebar3's repo_key: the key
	// `mix hex.organization auth` or `rebar3 hex organization auth` stores.
	AuthKey string
	OAuth   HexToken
}

// ReadHexConfig reads hex.config in HexHome; an absent or unreadable file is an
// empty configuration.
//
// Implements: REQ-AUTH-028
func (m Machine) ReadHexConfig() HexConfig {
	dir := m.HexHome()
	if dir == "" {
		return HexConfig{}
	}
	data, err := os.ReadFile(filepath.Join(dir, "hex.config"))
	if err != nil {
		return HexConfig{}
	}
	return ParseHexConfig(data)
}

// HexAPIURL is the Hex API this machine's Mix talks to: HEX_API_URL, HEX_API, else
// the api_url of hex.config; "" for hex.pm's.
//
// Implements: REQ-SUP-015, REQ-SUP-064
func (m Machine) HexAPIURL() string {
	for _, name := range []string{"HEX_API_URL", "HEX_API"} {
		if v := strings.TrimSpace(m.Env(name)); v != "" {
			return v
		}
	}
	return m.ReadHexConfig().APIURL
}

// ParseHexConfig reads hex.config, a file of Erlang terms `{Key, Value}.` as Hex
// writes them with io_lib:print. A term it cannot read ends the reading; what came
// before it is kept. The encrypted keys of older Hex versions ($encrypted_key,
// $write_key) are left out: they need the user's local password.
//
// Implements: REQ-AUTH-028
func ParseHexConfig(data []byte) HexConfig {
	out := HexConfig{Repos: map[string]HexRepo{}}
	for _, form := range parseErlangTerms(string(data)) {
		if form.kind != 't' || len(form.items) != 2 {
			continue
		}
		value := form.items[1]
		switch form.items[0].text() {
		case "api_url":
			out.APIURL = value.text()
		case "api_key":
			out.APIKey = value.text()
		case "$oauth_token":
			out.OAuth = hexToken(value)
		case "$repos":
			for i := 0; value.kind == 'm' && i+1 < len(value.items); i += 2 {
				name, repo := value.items[i].text(), value.items[i+1]
				if name == "" || repo.kind != 'm' {
					continue
				}
				key, _ := repo.get("auth_key")
				token, _ := repo.get("oauth_token")
				out.Repos[name] = HexRepo{AuthKey: key.text(), OAuth: hexToken(token)}
			}
		}
	}
	return out
}

// hexToken reads an OAuth token map: #{access_token => ..., expires_at => N}.
func hexToken(t erlTerm) HexToken {
	access, _ := t.get("access_token")
	expires, _ := t.get("expires_at")
	n, _ := strconv.ParseInt(expires.text(), 10, 64)
	return HexToken{Access: access.binary(), Expires: n}
}

// ---------------------------------------------------------------- Erlang terms

// erlTerm is an Erlang term of the kinds a configuration file holds.
type erlTerm struct {
	kind  byte // 'a' atom, 's' string or binary, 'n' number, 't' tuple, 'l' list, 'm' map
	s     string
	items []erlTerm // a map's are key, value, key, value, ...
}

// text is the atom, string, binary or number a term holds, "" for anything else.
func (t erlTerm) text() string {
	if t.kind == 'a' || t.kind == 's' || t.kind == 'n' {
		return t.s
	}
	return ""
}

// binary is the string or binary a term holds, "" for anything else: rebar3 writes
// an absent key as the atom undefined.
func (t erlTerm) binary() string {
	if t.kind == 's' {
		return t.s
	}
	return ""
}

// get looks a key up in a map, the key an atom, a string or a binary alike.
func (t erlTerm) get(key string) (erlTerm, bool) {
	for i := 0; t.kind == 'm' && i+1 < len(t.items); i += 2 {
		if t.items[i].text() == key {
			return t.items[i+1], true
		}
	}
	return erlTerm{}, false
}

// erlParser reads the forms of a file:consult/1 file.
type erlParser struct {
	src string
	i   int
}

// parseErlangTerms reads the terms of src, each ended by a full stop, up to the
// first it cannot read.
func parseErlangTerms(src string) []erlTerm {
	p := &erlParser{src: src}
	var out []erlTerm
	for {
		p.space()
		if p.i >= len(p.src) {
			return out
		}
		t, ok := p.term()
		if !ok || !p.eat(".") {
			return out
		}
		out = append(out, t)
	}
}

// space skips white space and % comments.
func (p *erlParser) space() {
	for p.i < len(p.src) {
		switch c := p.src[p.i]; {
		case c == '%':
			for p.i < len(p.src) && p.src[p.i] != '\n' {
				p.i++
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			p.i++
		default:
			return
		}
	}
}

// eat consumes s after white space.
func (p *erlParser) eat(s string) bool {
	p.space()
	if strings.HasPrefix(p.src[p.i:], s) {
		p.i += len(s)
		return true
	}
	return false
}

func (p *erlParser) term() (erlTerm, bool) {
	p.space()
	if p.i >= len(p.src) {
		return erlTerm{}, false
	}
	switch c := p.src[p.i]; {
	case p.eat("#{"):
		return p.seq('m', "}")
	case c == '{':
		p.i++
		return p.seq('t', "}")
	case c == '[':
		p.i++
		return p.seq('l', "]")
	case p.eat("<<"):
		return p.binary()
	case c == '"':
		s, ok := p.strings()
		return erlTerm{kind: 's', s: s}, ok
	case c == '\'':
		s, ok := p.quoted('\'')
		return erlTerm{kind: 'a', s: s}, ok
	case c >= 'a' && c <= 'z':
		start := p.i
		for p.i < len(p.src) && isErlNameByte(p.src[p.i]) {
			p.i++
		}
		return erlTerm{kind: 'a', s: p.src[start:p.i]}, true
	case c >= '0' && c <= '9' || c == '-':
		return p.number()
	}
	return erlTerm{}, false
}

func isErlNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '@'
}

// seq reads the elements of a tuple, list or map up to close; a map's are
// key => value pairs.
func (p *erlParser) seq(kind byte, close string) (erlTerm, bool) {
	out := erlTerm{kind: kind}
	if p.eat(close) {
		return out, true
	}
	for {
		t, ok := p.term()
		if !ok {
			return out, false
		}
		out.items = append(out.items, t)
		if kind == 'm' {
			if !p.eat("=>") && !p.eat(":=") {
				return out, false
			}
			v, ok := p.term()
			if !ok {
				return out, false
			}
			out.items = append(out.items, v)
		}
		if p.eat(close) {
			return out, true
		}
		if !p.eat(",") {
			return out, false
		}
	}
}

// binary reads what follows "<<": strings (with an optional /utf8) or byte values.
func (p *erlParser) binary() (erlTerm, bool) {
	var b strings.Builder
	for !p.eat(">>") {
		p.space()
		if p.i >= len(p.src) {
			return erlTerm{}, false
		}
		if p.src[p.i] == '"' {
			s, ok := p.strings()
			if !ok {
				return erlTerm{}, false
			}
			b.WriteString(s)
		} else {
			n, ok := p.number()
			v, err := strconv.Atoi(n.s)
			if !ok || err != nil || v < 0 || v > 255 {
				return erlTerm{}, false
			}
			b.WriteByte(byte(v))
		}
		if p.eat("/") {
			for p.i < len(p.src) && isErlNameByte(p.src[p.i]) {
				p.i++
			}
		}
		if !p.eat(",") {
			if !p.eat(">>") {
				return erlTerm{}, false
			}
			break
		}
	}
	return erlTerm{kind: 's', s: b.String()}, true
}

// strings reads one string literal and those adjacent to it, which Erlang joins.
func (p *erlParser) strings() (string, bool) {
	var b strings.Builder
	for {
		s, ok := p.quoted('"')
		if !ok {
			return "", false
		}
		b.WriteString(s)
		p.space()
		if p.i >= len(p.src) || p.src[p.i] != '"' {
			return b.String(), true
		}
	}
}

// quoted reads a literal between two quote characters, with its escapes.
func (p *erlParser) quoted(q byte) (string, bool) {
	p.i++ // the opening quote
	var b strings.Builder
	for p.i < len(p.src) {
		c := p.src[p.i]
		p.i++
		switch {
		case c == q:
			return b.String(), true
		case c == '\\' && p.i < len(p.src):
			e := p.src[p.i]
			p.i++
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 's':
				b.WriteByte(' ')
			default:
				b.WriteByte(e)
			}
		default:
			b.WriteByte(c)
		}
	}
	return "", false
}

// number reads an integer or a float, as text.
func (p *erlParser) number() (erlTerm, bool) {
	start := p.i
	if p.i < len(p.src) && p.src[p.i] == '-' {
		p.i++
	}
	digits := p.i
	for p.i < len(p.src) && (p.src[p.i] >= '0' && p.src[p.i] <= '9' || p.src[p.i] == '_') {
		p.i++
	}
	if p.i == digits {
		return erlTerm{}, false
	}
	// A fraction needs a digit after the point: "1." ends a form.
	if p.i+1 < len(p.src) && p.src[p.i] == '.' && p.src[p.i+1] >= '0' && p.src[p.i+1] <= '9' {
		p.i++
		for p.i < len(p.src) && (p.src[p.i] >= '0' && p.src[p.i] <= '9' || strings.IndexByte("eE+-", p.src[p.i]) >= 0) {
			p.i++
		}
	}
	return erlTerm{kind: 'n', s: strings.ReplaceAll(p.src[start:p.i], "_", "")}, true
}

// ---------------------------------------------------------------- rebar3

// Rebar3GlobalConfig is the rebar.config rebar3 reads for every project, in
// .config/rebar3 under REBAR_GLOBAL_CONFIG_DIR, else the home directory.
//
// Implements: REQ-SUP-064, REQ-BEAM-013
func (m Machine) Rebar3GlobalConfig() string {
	return join(cmp.Or(m.Env("REBAR_GLOBAL_CONFIG_DIR"), m.Home), ".config", "rebar3", "rebar.config")
}

// Rebar3HexConfig is the hex.config rebar3 and its hex plugin keep repository keys
// and tokens in: .config/rebar3 under REBAR_GLOBAL_CONFIG_DIR, else REBAR_CACHE_DIR
// (rebar3 makes that its global directory once the project is loaded), else the
// home directory.
//
// Implements: REQ-SUP-064, REQ-AUTH-028
func (m Machine) Rebar3HexConfig() string {
	return join(cmp.Or(m.Env("REBAR_GLOBAL_CONFIG_DIR"), m.Env("REBAR_CACHE_DIR"), m.Home), ".config", "rebar3", "hex.config")
}

// ReadRebar3HexRepos reads the Hex repositories the global rebar.config names in
// {hex, [{repos, [#{name => <<"hexpm:acme">>}, ...]}]}, in its order, and whether
// its first repos entry is {repos, replace, [...]}: without replace rebar3 asks
// hex.pm's public repository ("hexpm") after them.
//
// Implements: REQ-BEAM-013
func (m Machine) ReadRebar3HexRepos() (repos []string, replace bool) {
	name := m.Rebar3GlobalConfig()
	if name == "" {
		return nil, false
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, false
	}
	return ParseRebar3HexRepos(data)
}

// ParseRebar3HexRepos reads the repos entries of a rebar.config's {hex, Options}:
// the repository names in order, and whether the first entry replaces the default.
//
// Implements: REQ-BEAM-013
func ParseRebar3HexRepos(data []byte) (repos []string, replace bool) {
	first := true
	for _, form := range parseErlangTerms(string(data)) {
		if form.kind != 't' || len(form.items) != 2 || form.items[0].kind != 'a' || form.items[0].s != "hex" {
			continue
		}
		for _, opt := range form.items[1].items {
			if opt.kind != 't' || len(opt.items) < 2 || opt.items[0].kind != 'a' || opt.items[0].s != "repos" {
				continue
			}
			list := opt.items[len(opt.items)-1]
			if first {
				replace = len(opt.items) == 3 && opt.items[1].kind == 'a' && opt.items[1].s == "replace"
				first = false
			}
			for _, r := range list.items {
				if n, ok := r.get("name"); ok && n.text() != "" {
					repos = append(repos, n.text())
				}
			}
		}
	}
	return repos, replace
}

// ReadRebar3HexConfig reads rebar3's hex.config (Rebar3HexConfig): one map from a
// repository name to what authenticates to it,
//
//	#{<<"hexpm">> => #{api_key => <<"...">>},
//	  <<"hexpm:acme">> => #{name => <<"hexpm:acme">>, repo_key => <<"...">>},
//	  <<"$oauth">> => #{access_token => <<"...">>, expires_at => 1893456000}}.
//
// as the same HexConfig Mix's is read into: hexpm's api_key is APIKey, $oauth (the
// token `rebar3 hex user auth` stores) is OAuth, and each repository's api_key,
// repo_key (auth_key when it has none) and oauth_token are its Repos entry.
//
// Implements: REQ-AUTH-028
func (m Machine) ReadRebar3HexConfig() HexConfig {
	name := m.Rebar3HexConfig()
	if name == "" {
		return HexConfig{}
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return HexConfig{}
	}
	return ParseRebar3HexConfig(data)
}

// ParseRebar3HexConfig reads the map of rebar3's hex.config (see ReadRebar3HexConfig).
//
// Implements: REQ-AUTH-028
func ParseRebar3HexConfig(data []byte) HexConfig {
	out := HexConfig{Repos: map[string]HexRepo{}}
	forms := parseErlangTerms(string(data))
	if len(forms) == 0 || forms[0].kind != 'm' {
		return out
	}
	m := forms[0]
	for i := 0; i+1 < len(m.items); i += 2 {
		name, repo := m.items[i].text(), m.items[i+1]
		if name == "" || repo.kind != 'm' {
			continue
		}
		if name == "$oauth" {
			out.OAuth = hexToken(repo)
			continue
		}
		apiKey, _ := repo.get("api_key")
		key, _ := repo.get("repo_key")
		if key.binary() == "" {
			key, _ = repo.get("auth_key")
		}
		token, _ := repo.get("oauth_token")
		out.Repos[name] = HexRepo{APIKey: apiKey.binary(), AuthKey: key.binary(), OAuth: hexToken(token)}
		if name == "hexpm" {
			out.APIKey = apiKey.binary()
		}
	}
	return out
}
