// Package config resolves settings from, in increasing precedence: defaults, the user
// config file, the project config file, DEPPHUNTER_* environment variables, and flags.
// Flags are pflag (the cobra command registers them with RegisterFlags); files,
// environment and flags are layered with viper.
package config

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const ProjectFile = ".depphunter.yaml"

// UI holds the initial state of settings the user can also change in the browser.
type UI struct {
	Theme       string `yaml:"theme" mapstructure:"theme" json:"theme"`                     // auto | light | dark
	ColorBy     string `yaml:"color_by" mapstructure:"color_by" json:"colorBy"`             // language | size | commits | churn | age | authors
	HeightScale string `yaml:"height_scale" mapstructure:"height_scale" json:"heightScale"` // linear | sqrt | log
	// Style is what the map is dressed as (see web/static/city.js): the same layout
	// drawn as a city, a printed circuit board or a galaxy.
	Style   string `yaml:"style,omitempty" mapstructure:"style" json:"style"` // city | circuit | galaxy
	ShowStd bool   `yaml:"show_std" mapstructure:"show_std" json:"showStd"`
	// Tool is what walk mode puts in the walker's hands (see web/static/tools.js).
	Tool        string `yaml:"tool,omitempty" mapstructure:"tool" json:"tool"`
	ExpandDepth int    `yaml:"expand_depth" mapstructure:"expand_depth" json:"expandDepth"` // 0 = auto, -1 = everything
	// Filters, as the browser's Filters panel sets them.
	HideLanguages []string `yaml:"hide_languages,omitempty" mapstructure:"hide_languages" json:"hideLanguages"`
	HideIslands   []string `yaml:"hide_islands,omitempty" mapstructure:"hide_islands" json:"hideIslands"` // ecosystem ids, e.g. "npm"
	PathFilter    string   `yaml:"path_filter,omitempty" mapstructure:"path_filter" json:"pathFilter"`
}

// Validate reports settings outside their allowed values.
func (u UI) Validate() error {
	return errors.Join(
		oneOf("theme", u.Theme, "auto", "light", "dark"),
		oneOf("color-by", u.ColorBy, "language", "size", "commits", "churn", "age", "authors"),
		oneOf("height-scale", u.HeightScale, "linear", "sqrt", "log"),
		func() error {
			if u.Style == "" {
				return nil // the browser's default
			}
			return oneOf("style", u.Style, "city", "circuit", "galaxy")
		}(),
		func() error {
			if u.Tool == "" {
				return nil // the browser's default
			}
			return oneOf("tool", u.Tool, "rod", "net", "camera", "bubbles", "dart")
		}(),
	)
}

func oneOf(name, v string, allowed ...string) error {
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	return fmt.Errorf("invalid %s %q (want one of %s)", name, v, strings.Join(allowed, ", "))
}

type Config struct {
	Root        string   `yaml:"-" mapstructure:"-"`
	ConfigFile  string   `yaml:"-" mapstructure:"-"` // where the browser's "Save settings" writes
	Addr        string   `yaml:"addr" mapstructure:"addr"`
	Open        bool     `yaml:"open" mapstructure:"open"`
	Exclude     []string `yaml:"exclude" mapstructure:"exclude"`
	MaxFileSize int64    `yaml:"max_file_size" mapstructure:"max_file_size"`
	Watch       bool     `yaml:"watch" mapstructure:"watch"`
	Cache       bool     `yaml:"cache" mapstructure:"cache"`
	// History reads git history (up to HistoryCommits commits) for the history overlay.
	History        bool `yaml:"history" mapstructure:"history"`
	HistoryCommits int  `yaml:"history_commits" mapstructure:"history_commits"`
	// ResolveDepth adds an external package's own dependencies, read from the
	// project's lock files: 0 none, -1 as far as they reach.
	ResolveDepth int `yaml:"resolve_depth" mapstructure:"resolve_depth"`
	// Online allows asking package indexes about dependencies the repository's own
	// files do not record. Analysis is offline without it.
	Online bool `yaml:"online" mapstructure:"online"`
	// Explain writes the resolution report to the log when the analysis is over (see
	// internal/trace). The report is served at /api/resolution either way; this is
	// what puts it on the terminal.
	Explain bool `yaml:"explain" mapstructure:"explain"`
	// Private names the packages that are this organization's own, as glob patterns
	// with GOPRIVATE's meaning; nothing matched is named to a public index or sent to
	// the vulnerability database (see internal/scope, which says why). GOPRIVATE and
	// GONOPROXY are read on top of whatever is set here.
	Private []string `yaml:"private" mapstructure:"private"`
	// TrustIndexes are index URLs to treat as though this machine's own configuration
	// named them, for the organization whose repositories carry their own .npmrc and
	// would otherwise draw a warning on every package (see internal/index). It comes
	// from the user's own config or the command line only: a repository vouching for
	// itself would be no guard at all.
	TrustIndexes []string `yaml:"trust_indexes" mapstructure:"trust_indexes"`
	// Findings are files (or globs) holding what a scanner already reported:
	// govulncheck, npm audit, trivy, golangci-lint, eslint or osv-scanner JSON.
	Findings []string `yaml:"findings" mapstructure:"findings"`
	// Vulns places those reports on the map and, with Online, additionally asks the
	// OSV database about every pinned external package.
	Vulns bool `yaml:"vulns" mapstructure:"vulns"`
	// Links follows the links the repository's Markdown carries and reports the ones
	// that lead nowhere. It needs nothing but the repository, so it is on by default;
	// with Online the http(s) links are asked about as well.
	Links bool `yaml:"links" mapstructure:"links"`
	// LSP asks installed language servers for symbol-level references (slow, opt-in).
	LSP        bool          `yaml:"lsp" mapstructure:"lsp"`
	LSPTimeout time.Duration `yaml:"lsp_timeout" mapstructure:"lsp_timeout"`
	// Editor is a command template such as "code -g {file}:{line}"; empty = auto-detect.
	Editor string `yaml:"editor" mapstructure:"editor"`
	UI     UI     `yaml:"ui" mapstructure:"ui"`

	// Export and Output are one-shot actions, so they only come from flags.
	Export string `yaml:"-" mapstructure:"-"`
	Output string `yaml:"-" mapstructure:"-"`

	// Embed lists the origins allowed to show the map in a frame of their own - an
	// editor's built-in browser, which is how the VS Code extension hosts it. It
	// relaxes what the server otherwise refuses outright, so it comes from flags
	// only: a program that launches depphunter passes it, and neither a config file
	// nor the environment can turn it on behind the user's back.
	Embed []string `yaml:"-" mapstructure:"-"`
}

func Default() Config {
	return Config{
		Addr:           "127.0.0.1:0",
		Open:           true,
		Cache:          true,
		History:        true,
		Vulns:          true,
		Links:          true,
		HistoryCommits: 10000,
		LSPTimeout:     5 * time.Minute,
		MaxFileSize:    2 << 20,
		UI:             UI{Theme: "auto", ColorBy: "language", HeightScale: "sqrt", Style: "city"},
	}
}

// RegisterFlags declares the command-line flags on fs. Load reads them back.
func RegisterFlags(fs *pflag.FlagSet) {
	d := Default()
	fs.String("config", "", "config file to use instead of <path>/"+ProjectFile)
	fs.String("addr", d.Addr, "listen address (port 0 picks a free port)")
	fs.Bool("no-open", false, "do not open the browser")
	fs.StringArray("exclude", nil, "glob of paths to skip; repeatable")
	fs.Int64("max-file-size", d.MaxFileSize, "files larger than this many bytes are not read")
	fs.String("theme", d.UI.Theme, "color theme: auto, light, dark")
	fs.String("color-by", d.UI.ColorBy, "building color: language, size, commits, churn, age, authors")
	fs.String("height-scale", d.UI.HeightScale, "building height scale: linear, sqrt, log")
	fs.String("style", d.UI.Style, "what the map is dressed as: city, circuit, galaxy")
	fs.Bool("show-std", d.UI.ShowStd, "show standard-library islands")
	fs.Int("expand-depth", d.UI.ExpandDepth, "initially expanded directory depth (0 = auto, -1 = all)")
	fs.Bool("watch", false, "re-analyze on file changes and update the browser live")
	fs.Bool("no-cache", false, "do not read or write the analysis cache")
	fs.Bool("no-history", false, "do not read git history")
	fs.Int("history-commits", d.HistoryCommits, "read at most this many commits of git history")
	fs.Bool("online", false, "ask package indexes about dependencies the project's files do not record")
	fs.Bool("explain", false,
		"write the resolution report when the analysis is over: which index answered for which package, "+
			"what each level of --resolve-depth added, and what nothing answered for")
	fs.StringArray("private", nil,
		"glob naming packages your organization owns, as GOPRIVATE writes them (e.g. corp.example/*, npm:@acme/*); "+
			"they are never asked of a public index nor sent to the vulnerability database; repeatable")
	fs.StringArray("trust-index", nil,
		"index URL to treat as configured on this machine, so a repository that names it is not marked; repeatable")
	fs.StringArray("findings", nil,
		"scanner report to place on the map (govulncheck, npm audit, trivy, golangci-lint, eslint, osv-scanner JSON); repeatable, globs allowed")
	fs.Bool("no-vulns", false, "do not place scanner reports on the map, and do not ask the OSV database")
	fs.Bool("no-links", false, "do not follow the links the repository's Markdown carries")
	fs.Int("resolve-depth", d.ResolveDepth,
		"levels of external dependencies-of-dependencies to resolve from lock files (-1 = all)")
	fs.Bool("lsp", false, "find symbol references with installed language servers (gopls, …)")
	fs.Duration("lsp-timeout", d.LSPTimeout, "time budget for language servers")
	fs.String("editor", "", `editor command template, e.g. "code -g {file}:{line}" (default: auto-detect)`)
	fs.StringArray("embed", nil,
		"origin allowed to show the map in a frame, e.g. vscode-webview: for an editor's browser; repeatable")
	fs.String("export", "", "write the graph as json, graphml, dot or html and exit instead of serving")
	fs.StringP("output", "o", "", "output file for --export (default: stdout)")
}

// Settings a flag sets directly, by flag name.
var flagKeys = map[string]string{
	"addr": "addr", "max-file-size": "max_file_size", "watch": "watch",
	"history-commits": "history_commits", "resolve-depth": "resolve_depth", "online": "online",
	"explain": "explain",
	"lsp":     "lsp", "lsp-timeout": "lsp_timeout", "editor": "editor",
	"theme": "ui.theme", "color-by": "ui.color_by", "height-scale": "ui.height_scale", "style": "ui.style",
	"show-std": "ui.show_std", "expand-depth": "ui.expand_depth",
}

// Settings a --no-* flag turns off.
var negatedFlags = map[string]string{
	"no-open": "open", "no-cache": "cache", "no-history": "history",
	"no-vulns": "vulns", "no-links": "links",
}

// Environment variables (after the DEPPHUNTER_ prefix), by setting. EXCLUDE is
// handled apart: it adds to the configured globs instead of replacing them.
var envKeys = map[string]string{
	"addr": "ADDR", "open": "OPEN", "max_file_size": "MAX_FILE_SIZE", "watch": "WATCH", "cache": "CACHE",
	"history": "HISTORY", "history_commits": "HISTORY_COMMITS", "resolve_depth": "RESOLVE_DEPTH", "online": "ONLINE",
	"explain": "EXPLAIN",
	"vulns":   "VULNS", "links": "LINKS", "lsp": "LSP", "lsp_timeout": "LSP_TIMEOUT",
	"editor": "EDITOR", "ui.theme": "THEME", "ui.color_by": "COLOR_BY", "ui.height_scale": "HEIGHT_SCALE", "ui.style": "STYLE",
	"ui.show_std": "SHOW_STD", "ui.expand_depth": "EXPAND_DEPTH",
}

// Load builds the configuration from the parsed flags fs (see RegisterFlags), the
// positional args (at most one path), the environment, and the config files.
// userDir holds the user-level config (e.g. ~/.config/depphunter); "" skips it.
func Load(fs *pflag.FlagSet, args []string, userDir string) (Config, error) {
	cfg := Default()
	if len(args) > 1 {
		return cfg, fmt.Errorf("expected at most one path, got %d", len(args))
	}
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return cfg, err
	}
	// Walking a symlinked root would yield nothing; analyze the directory it points to.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return cfg, fmt.Errorf("%s is not a directory", root)
	}

	v := viper.New()
	setDefaults(v, cfg)
	if userDir != "" {
		if err := mergeFile(v, filepath.Join(userDir, "config.yaml"), false, true); err != nil {
			return cfg, err
		}
	}
	// The project config comes with the (possibly untrusted) repository, so it may not
	// choose a program this machine runs or send it onto the network: its editor and
	// online keys are dropped. A file named with --config is the user's own choice.
	configFile, _ := fs.GetString("config")
	if configFile != "" {
		if configFile, err = filepath.Abs(configFile); err != nil {
			return cfg, err
		}
		err = mergeFile(v, configFile, true, true)
	} else {
		configFile = filepath.Join(root, ProjectFile)
		err = mergeFile(v, configFile, false, false)
	}
	if err != nil {
		return cfg, err
	}
	for key, name := range envKeys {
		if err := v.BindEnv(key, "DEPPHUNTER_"+name); err != nil {
			return cfg, err
		}
	}
	for name, key := range flagKeys {
		if err := v.BindPFlag(key, fs.Lookup(name)); err != nil {
			return cfg, err
		}
	}
	for name, key := range negatedFlags {
		if fs.Changed(name) {
			off, _ := fs.GetBool(name)
			v.Set(key, !off)
		}
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, err
	}

	cfg.Root, cfg.ConfigFile = root, configFile
	if e := os.Getenv("DEPPHUNTER_EXCLUDE"); e != "" {
		cfg.Exclude = append(cfg.Exclude, strings.Split(e, ",")...)
	}
	flagExclude, _ := fs.GetStringArray("exclude")
	cfg.Exclude = append(cfg.Exclude, flagExclude...)
	if f := os.Getenv("DEPPHUNTER_FINDINGS"); f != "" {
		cfg.Findings = append(cfg.Findings, strings.Split(f, ",")...)
	}
	flagFindings, _ := fs.GetStringArray("findings")
	cfg.Findings = append(cfg.Findings, flagFindings...)
	if p := os.Getenv("DEPPHUNTER_PRIVATE"); p != "" {
		cfg.Private = append(cfg.Private, strings.Split(p, ",")...)
	}
	flagPrivate, _ := fs.GetStringArray("private")
	cfg.Private = append(cfg.Private, flagPrivate...)
	if t := os.Getenv("DEPPHUNTER_TRUST_INDEXES"); t != "" {
		cfg.TrustIndexes = append(cfg.TrustIndexes, strings.Split(t, ",")...)
	}
	flagTrust, _ := fs.GetStringArray("trust-index")
	cfg.TrustIndexes = append(cfg.TrustIndexes, flagTrust...)
	cfg.Embed, _ = fs.GetStringArray("embed")
	cfg.Export, _ = fs.GetString("export")
	cfg.Output, _ = fs.GetString("output")
	return cfg, cfg.validate()
}

// setDefaults makes every setting known to viper, so environment variables and
// Unmarshal see it even when no file mentions it.
func setDefaults(v *viper.Viper, d Config) {
	for key, val := range map[string]any{
		"addr": d.Addr, "open": d.Open, "exclude": d.Exclude, "max_file_size": d.MaxFileSize,
		"watch": d.Watch, "cache": d.Cache, "history": d.History, "history_commits": d.HistoryCommits,
		"resolve_depth": d.ResolveDepth, "online": d.Online, "explain": d.Explain,
		"findings": d.Findings, "vulns": d.Vulns, "links": d.Links,
		"private": d.Private, "trust_indexes": d.TrustIndexes,
		"lsp": d.LSP, "lsp_timeout": d.LSPTimeout,
		"editor":   d.Editor,
		"ui.theme": d.UI.Theme, "ui.color_by": d.UI.ColorBy, "ui.height_scale": d.UI.HeightScale, "ui.style": d.UI.Style,
		"ui.show_std": d.UI.ShowStd, "ui.expand_depth": d.UI.ExpandDepth, "ui.tool": d.UI.Tool,
		"ui.hide_languages": d.UI.HideLanguages, "ui.hide_islands": d.UI.HideIslands, "ui.path_filter": d.UI.PathFilter,
	} {
		v.SetDefault(key, val)
	}
}

// mergeFile overlays the YAML file onto v; keys absent from the file keep their value.
func mergeFile(v *viper.Viper, name string, required, trusted bool) error {
	data, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) && !required {
		return nil
	}
	if err != nil {
		return err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if !trusted {
		// The keys that decide what this machine runs or reaches: a repository must
		// not choose either. Vouching for an index is the same kind of decision -
		// the whole point of the marking is that a repository's word for its own
		// registry is not enough - so a project file does not get to do it.
		//
		// `private` is the other way round and stays: all it can do is stop
		// depphunter from naming a package to somebody else, and a repository saying
		// "these are ours" is exactly who would know.
		delete(m, "editor")
		delete(m, "online")
		delete(m, "trust_indexes")
		// A repository may point at its own scanner reports, which is how a project
		// ships the output its CI already produces - but only at paths inside itself.
		if list, ok := m["findings"]; ok {
			m["findings"] = confine(list)
		}
	}
	return v.MergeConfigMap(m)
}

// confine keeps the report paths a repository may name: relative ones that stay
// within it. Anything rooted, or climbing out, is dropped.
func confine(list any) []string {
	items, ok := list.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, it := range items {
		if s, ok := it.(string); ok && inside(s) {
			out = append(out, s)
		}
	}
	return out
}

// inside reports whether a path stays within the directory it was read from.
//
// The question is deliberately not put to path/filepath, which answers for the host,
// because this path travels: it comes out of a file in the repository and is read on
// whatever machine the repository was cloned to. On Windows filepath.IsAbs("/etc/shadow")
// is false - that path is rooted, not absolute - so a check that trusted it would let
// a repository point the scanner wherever it liked as soon as someone cloned it there.
// Both conventions are applied to every path, and either one objecting is enough.
func inside(s string) bool {
	if s == "" || rooted(s) {
		return false
	}
	// Slashes both ways, since the file may have been written on the other system.
	clean := path.Clean(strings.ReplaceAll(s, `\`, "/"))
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

// rooted reports whether a path starts from the root of some filesystem: a leading
// separator of either kind, a drive letter, or a UNC share.
func rooted(s string) bool {
	if s[0] == '/' || s[0] == '\\' {
		return true
	}
	return len(s) >= 2 && s[1] == ':' &&
		(s[0] >= 'a' && s[0] <= 'z' || s[0] >= 'A' && s[0] <= 'Z')
}

// FindingsEnabled reports whether anything will be placed on the map: a report to
// read, a database to ask, or documentation whose links may lead nowhere.
func (c Config) FindingsEnabled() bool {
	return c.Links || c.Vulns && (len(c.Findings) > 0 || c.Online)
}

func (c Config) validate() error {
	return errors.Join(
		c.UI.Validate(),
		func() error {
			if c.HistoryCommits < 1 {
				return errors.New("history-commits must be at least 1")
			}
			return nil
		}(),
		func() error {
			if c.ResolveDepth < -1 {
				return errors.New("resolve-depth must be -1 or more")
			}
			return nil
		}(),
		func() error {
			if c.Export == "" {
				return nil
			}
			return oneOf("export", c.Export, "json", "graphml", "dot", "html")
		}(),
		func() error {
			if c.Output != "" && c.Export == "" {
				return errors.New("-o requires --export")
			}
			return nil
		}(),
		func() error {
			for _, origin := range c.Embed {
				if !frameOrigin(origin) {
					return fmt.Errorf("embed: %q is not a scheme or an origin, e.g. vscode-webview: or https://example.test", origin)
				}
			}
			return nil
		}(),
	)
}

// frameOrigin says whether a string may go into the Content-Security-Policy header
// as a frame-ancestors source. What comes in is a command line argument and what it
// lands in is a security header, so what it may be is spelled out here rather than
// left to whatever the browser makes of it: a scheme on its own
// ("vscode-webview:"), or a scheme with a host and maybe a port. Nothing with a
// space, a quote, a slash or a semicolon in it gets through, so nothing passed here
// can end the directive early or start another one.
func frameOrigin(s string) bool {
	if scheme, host, ok := strings.Cut(s, "://"); ok {
		return isScheme(scheme) && isHost(host)
	}
	scheme, ok := strings.CutSuffix(s, ":")
	return ok && isScheme(scheme)
}

// isScheme: a letter, then letters, digits and the three punctuation marks a URL
// scheme is allowed (RFC 3986).
func isScheme(s string) bool {
	if s == "" || !isLetter(rune(s[0])) {
		return false
	}
	for _, r := range s {
		if !isLetter(r) && !isDigit(r) && r != '+' && r != '-' && r != '.' {
			return false
		}
	}
	return true
}

// isHost: a host name and an optional port. A leading "*." is allowed, and only
// there, because an editor's web build hands every session a host of its own and
// there is nothing else to name it by.
func isHost(s string) bool {
	host, port, hasPort := strings.Cut(s, ":")
	if hasPort {
		if port == "" {
			return false
		}
		for _, r := range port {
			if !isDigit(r) {
				return false
			}
		}
	}
	host = strings.TrimPrefix(host, "*.")
	if host == "" {
		return false
	}
	for _, r := range host {
		if !isLetter(r) && !isDigit(r) && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

func isLetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }
func isDigit(r rune) bool  { return r >= '0' && r <= '9' }
