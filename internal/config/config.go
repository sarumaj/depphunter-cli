// Package config resolves settings from, in increasing precedence: defaults, the user
// config file, the project config file, DEPPHUNTER_* environment variables, and flags.
// Flags are pflag (the cobra command registers them with RegisterFlags); files,
// environment and flags are layered with viper.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

const ProjectFile = ".depphunter.yaml"

// UI holds the initial state of settings the user can also change in the browser.
type UI struct {
	Theme       string `yaml:"theme" mapstructure:"theme" json:"theme"`                     // auto | light | dark
	ColorBy     string `yaml:"color_by" mapstructure:"color_by" json:"colorBy"`             // language | size | commits | churn | age | authors
	HeightScale string `yaml:"height_scale" mapstructure:"height_scale" json:"heightScale"` // linear | sqrt | log
	// Style is what the map is dressed as (see web/static/map/city.js): the same layout
	// drawn as a city, a printed circuit board or a galaxy.
	Style   string `yaml:"style,omitempty" mapstructure:"style" json:"style"` // city | circuit | galaxy
	ShowStd bool   `yaml:"show_std" mapstructure:"show_std" json:"showStd"`
	// Tool is what walk mode puts in the walker's hands (see web/static/walk/tools.js).
	//
	// Implements: REQ-TOOL-003
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
			return oneOf("tool", u.Tool,
				// The primary tools, which hunt, and then the secondary ones, which
				// carry the walker (web/static/walk/tools.js) - and "skimmers", the swim
				// ring's old name, which the browser still reads as the ring.
				"rod", "net", "camera", "bubbles", "extinguisher", "dart", "nailer",
				"parachute", "grapple", "jetpack", "ring", "skimmers")
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
	Address     string   `yaml:"addr" mapstructure:"addr"`
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
	//
	// Implements: REQ-SUP-008
	ResolveDepth int `yaml:"resolve_depth" mapstructure:"resolve_depth"`
	// Online allows asking package indexes about dependencies the repository's own
	// files do not record. Analysis is offline without it.
	//
	// Implements: REQ-SUP-020
	Online bool `yaml:"online" mapstructure:"online"`
	// Explain writes the resolution report to the log when the analysis is over (see
	// internal/trace). The report is served at /api/resolution either way; this is
	// what puts it on the terminal.
	//
	// Implements: REQ-TRC-010
	Explain bool `yaml:"explain" mapstructure:"explain"`
	// Private names the packages that are this organization's own, as glob patterns
	// with GOPRIVATE's meaning; nothing matched is named to a public index or sent to
	// the vulnerability database (see internal/scope, which says why). GOPRIVATE and
	// GONOPROXY are read on top of whatever is set here.
	//
	// Implements: REQ-SUP-034, REQ-SUP-041
	Private []string `yaml:"private" mapstructure:"private"`
	// TrustIndexes are index URLs to treat as though this machine's own configuration
	// named them, for the organization whose repositories carry their own .npmrc and
	// would otherwise draw a warning on every package (see internal/index). It comes
	// from the user's own config or the command line only: a repository vouching for
	// itself would be no guard at all.
	//
	// Implements: REQ-SUP-042
	TrustIndexes []string `yaml:"trust_indexes" mapstructure:"trust_indexes"`
	// Python is the interpreter whose installed distributions resolve Python imports
	// that no index has (internal/lang/python); it is read, never run. Empty uses an
	// activated VIRTUAL_ENV, else the project's own .venv or venv.
	//
	// Implements: REQ-PY-015
	Python string `yaml:"python" mapstructure:"python"`
	// Findings are files (or globs) holding what a scanner already reported:
	// govulncheck, npm audit, trivy, golangci-lint, eslint or osv-scanner JSON.
	Findings []string `yaml:"findings" mapstructure:"findings"`
	// Vulnerabilities places those reports on the map and, with Online, additionally asks the
	// OSV database about every pinned external package.
	Vulnerabilities bool `yaml:"vulns" mapstructure:"vulns"`
	// Links follows the links the repository's Markdown carries and reports the ones
	// that lead nowhere. It needs nothing but the repository, so it is on by default;
	// with Online the http(s) links are asked about as well.
	//
	// Implements: REQ-MD-010, REQ-MD-016
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
	// Implements: REQ-SEC-010
	Embed []string `yaml:"-" mapstructure:"-"`
	// AllowHost lists the origins of an editor served from elsewhere - a browser
	// build of VS Code at https://example.com - that hosts the map. Each is added to
	// Embed, and unless --addr says otherwise the server listens on every interface
	// (0.0.0.0:0), since the page is then loaded from a machine other than this
	// one. Like Embed it comes from flags only.
	// Implements: REQ-SEC-011
	AllowHost []string `yaml:"-" mapstructure:"-"`
}

// Implements: REQ-SEC-001
func Default() Config {
	return Config{
		Address:         "127.0.0.1:0",
		Open:            true,
		Cache:           true,
		History:         true,
		Vulnerabilities: true,
		Links:           true,
		HistoryCommits:  10000,
		LSPTimeout:      5 * time.Minute,
		MaxFileSize:     2 << 20,
		UI:              UI{Theme: "auto", ColorBy: "language", HeightScale: "sqrt", Style: "city"},
	}
}

// RegisterFlags declares the command-line flags on flags. Load reads them back.
//
// Implements: REQ-CFG-005, REQ-CLI-004
func RegisterFlags(flags *pflag.FlagSet) {
	d := Default()
	flags.String("config", "", "config file to use instead of <path>/"+ProjectFile)
	flags.String("addr", d.Address, "listen address (port 0 picks a free port)")
	flags.Bool("no-open", false, "do not open the browser")
	flags.StringArray("exclude", nil, "glob of paths to skip; repeatable")
	flags.Int64("max-file-size", d.MaxFileSize, "files larger than this many bytes are not read")
	flags.String("theme", d.UI.Theme, "color theme: auto, light, dark")
	flags.String("color-by", d.UI.ColorBy, "building color: language, size, commits, churn, age, authors")
	flags.String("height-scale", d.UI.HeightScale, "building height scale: linear, sqrt, log")
	flags.String("style", d.UI.Style, "what the map is dressed as: city, circuit, galaxy")
	flags.Bool("show-std", d.UI.ShowStd, "show standard-library islands")
	flags.Int("expand-depth", d.UI.ExpandDepth, "initially expanded directory depth (0 = auto, -1 = all)")
	flags.StringArray("ui-default", nil,
		"seed a view setting for a repository that has saved none: key=value, repeatable "+
			"(theme, color_by, height_scale, style, show_std, expand_depth, tool, path_filter)")
	flags.Bool("watch", false, "re-analyze on file changes and update the browser live")
	flags.Bool("no-cache", false, "do not read or write the analysis cache")
	flags.Bool("no-history", false, "do not read git history")
	// Implements: REQ-HIST-002
	flags.Int("history-commits", d.HistoryCommits, "read at most this many commits of git history")
	flags.Bool("online", false, "ask package indexes about dependencies the project's files do not record")
	flags.Bool("explain", false,
		"write the resolution report when the analysis is over: which index answered for which package, "+
			"what each level of --resolve-depth added, and what nothing answered for")
	flags.StringArray("private", nil,
		"glob naming packages your organization owns, as GOPRIVATE writes them (e.g. corp.example/*, npm:@acme/*); "+
			"they are never asked of a public index nor sent to the vulnerability database; repeatable")
	flags.String("python", "",
		"Python interpreter whose installed packages resolve imports no package index has (default: an activated "+
			"VIRTUAL_ENV, else the project's .venv or venv); its files are read, it is never run")
	flags.StringArray("trust-index", nil,
		"index URL to treat as configured on this machine, so a repository that names it is not marked; repeatable")
	// Implements: REQ-FND-001
	flags.StringArray("findings", nil,
		"scanner report to place on the map (govulncheck, npm audit, trivy, golangci-lint, eslint, osv-scanner JSON); repeatable, globs allowed")
	flags.Bool("no-vulns", false, "do not place scanner reports on the map, and do not ask the OSV database")
	// Implements: REQ-MD-016
	flags.Bool("no-links", false, "do not follow the links the repository's Markdown carries")
	flags.Int("resolve-depth", d.ResolveDepth,
		"levels of external dependencies-of-dependencies to resolve from lock files (-1 = all)")
	flags.Bool("lsp", false, "find symbol references with installed language servers (gopls, …)")
	// Implements: REQ-LSP-009
	flags.Duration("lsp-timeout", d.LSPTimeout, "time budget for language servers")
	flags.String("editor", "", `editor command template, e.g. "code -g {file}:{line}" (default: auto-detect)`)
	flags.StringArray("embed", nil,
		"origin allowed to show the map in a frame, e.g. vscode-webview: for an editor's browser; repeatable")
	flags.StringArray("allow-host", nil,
		"origin of an editor served from elsewhere that hosts the map, e.g. https://example.com; "+
			"as --embed <origin> plus --addr 0.0.0.0:0 unless --addr is given; repeatable")
	flags.String("export", "", "write the graph as json, graphml, dot or html and exit instead of serving")
	flags.StringP("output", "o", "", "output file for --export (default: stdout)")
}

// Settings a flag sets directly, by flag name.
var flagKeys = map[string]string{
	"addr": "addr", "max-file-size": "max_file_size", "watch": "watch",
	"history-commits": "history_commits", "resolve-depth": "resolve_depth", "online": "online",
	"explain": "explain",
	"lsp":     "lsp", "lsp-timeout": "lsp_timeout", "editor": "editor", "python": "python",
	"theme": "ui.theme", "color-by": "ui.color_by", "height-scale": "ui.height_scale", "style": "ui.style",
	"show-std": "ui.show_std", "expand-depth": "ui.expand_depth",
}

// Settings a --no-* flag turns off.
//
// Implements: REQ-CFG-008
var negatedFlags = map[string]string{
	"no-open": "open", "no-cache": "cache", "no-history": "history",
	"no-vulns": "vulns", "no-links": "links",
}

// Environment variables (after the DEPPHUNTER_ prefix), by setting. EXCLUDE is
// handled apart: it adds to the configured globs instead of replacing them.
//
// Implements: REQ-CFG-006
var environmentKeys = map[string]string{
	"addr": "ADDR", "open": "OPEN", "max_file_size": "MAX_FILE_SIZE", "watch": "WATCH", "cache": "CACHE",
	"history": "HISTORY", "history_commits": "HISTORY_COMMITS", "resolve_depth": "RESOLVE_DEPTH", "online": "ONLINE",
	"explain": "EXPLAIN",
	"vulns":   "VULNS", "links": "LINKS", "lsp": "LSP", "lsp_timeout": "LSP_TIMEOUT",
	"editor": "EDITOR", "python": "PYTHON", "ui.theme": "THEME", "ui.color_by": "COLOR_BY", "ui.height_scale": "HEIGHT_SCALE", "ui.style": "STYLE",
	"ui.show_std": "SHOW_STD", "ui.expand_depth": "EXPAND_DEPTH", "ui.tool": "TOOL",
}

// Load builds the configuration from the parsed flags (see RegisterFlags), the
// positional args (at most one path), the environment, and the config files.
// userDirectory holds the user-level config (e.g. ~/.config/depphunter); "" skips it.
//
// Implements: REQ-CFG-001, REQ-CFG-002, REQ-CFG-003, REQ-CLI-002
func Load(flags *pflag.FlagSet, arguments []string, userDirectory string) (Config, error) {
	config := Default()
	if len(arguments) > 1 {
		return config, fmt.Errorf("expected at most one path, got %d", len(arguments))
	}
	root := "."
	if len(arguments) == 1 {
		root = arguments[0]
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return config, err
	}
	// Walking a symlinked root would yield nothing; analyze the directory it points to.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if fileInfo, err := os.Stat(root); err != nil || !fileInfo.IsDir() {
		return config, fmt.Errorf("%s is not a directory", root)
	}

	v := viper.New()
	setDefaults(v, config)
	seeds, _ := flags.GetStringArray("ui-default")
	if err := uiDefaults(v, seeds); err != nil {
		return config, err
	}
	if userDirectory != "" {
		if err := mergeFile(v, filepath.Join(userDirectory, "config.yaml"), false, true); err != nil {
			return config, err
		}
	}
	// The project config comes with the (possibly untrusted) repository, so it may not
	// choose a program this machine runs or send it onto the network: only the keys in
	// projectKeys are taken from it. A file named with --config is the user's own choice.
	configFile, _ := flags.GetString("config")
	if configFile != "" {
		if configFile, err = filepath.Abs(configFile); err != nil {
			return config, err
		}
		err = mergeFile(v, configFile, true, true)
	} else {
		configFile = filepath.Join(root, ProjectFile)
		err = mergeFile(v, configFile, false, false)
	}
	if err != nil {
		return config, err
	}
	for key, name := range environmentKeys {
		if err := v.BindEnv(key, "DEPPHUNTER_"+name); err != nil {
			return config, err
		}
	}
	for name, key := range flagKeys {
		if err := v.BindPFlag(key, flags.Lookup(name)); err != nil {
			return config, err
		}
	}
	for name, key := range negatedFlags {
		if flags.Changed(name) {
			off, _ := flags.GetBool(name)
			v.Set(key, !off)
		}
	}
	if err := v.Unmarshal(&config); err != nil {
		return config, err
	}

	config.Root, config.ConfigFile = root, configFile
	// Implements: REQ-CFG-009
	if e := os.Getenv("DEPPHUNTER_EXCLUDE"); e != "" {
		config.Exclude = append(config.Exclude, strings.Split(e, ",")...)
	}
	flagExclude, _ := flags.GetStringArray("exclude")
	config.Exclude = append(config.Exclude, flagExclude...)
	if f := os.Getenv("DEPPHUNTER_FINDINGS"); f != "" {
		config.Findings = append(config.Findings, strings.Split(f, ",")...)
	}
	flagFindings, _ := flags.GetStringArray("findings")
	config.Findings = append(config.Findings, flagFindings...)
	if p := os.Getenv("DEPPHUNTER_PRIVATE"); p != "" {
		config.Private = append(config.Private, strings.Split(p, ",")...)
	}
	flagPrivate, _ := flags.GetStringArray("private")
	config.Private = append(config.Private, flagPrivate...)
	if t := os.Getenv("DEPPHUNTER_TRUST_INDEXES"); t != "" {
		config.TrustIndexes = append(config.TrustIndexes, strings.Split(t, ",")...)
	}
	flagTrust, _ := flags.GetStringArray("trust-index")
	config.TrustIndexes = append(config.TrustIndexes, flagTrust...)
	config.Embed, _ = flags.GetStringArray("embed")
	config.AllowHost, _ = flags.GetStringArray("allow-host")
	if len(config.AllowHost) > 0 {
		config.Embed = append(config.Embed, config.AllowHost...)
		if !flags.Changed("addr") {
			config.Address = "0.0.0.0:0"
		}
	}
	config.Export, _ = flags.GetString("export")
	config.Output, _ = flags.GetString("output")
	return config, config.validate()
}

// setDefaults makes every setting known to viper, so environment variables and
// Unmarshal see it even when no file mentions it.
func setDefaults(v *viper.Viper, d Config) {
	for key, value := range map[string]any{
		"addr": d.Address, "open": d.Open, "exclude": d.Exclude, "max_file_size": d.MaxFileSize,
		"watch": d.Watch, "cache": d.Cache, "history": d.History, "history_commits": d.HistoryCommits,
		"resolve_depth": d.ResolveDepth, "online": d.Online, "explain": d.Explain,
		"findings": d.Findings, "vulns": d.Vulnerabilities, "links": d.Links,
		"private": d.Private, "trust_indexes": d.TrustIndexes,
		"lsp": d.LSP, "lsp_timeout": d.LSPTimeout,
		"editor": d.Editor, "python": d.Python,
		"ui.theme": d.UI.Theme, "ui.color_by": d.UI.ColorBy, "ui.height_scale": d.UI.HeightScale, "ui.style": d.UI.Style,
		"ui.show_std": d.UI.ShowStd, "ui.expand_depth": d.UI.ExpandDepth, "ui.tool": d.UI.Tool,
		"ui.hide_languages": d.UI.HideLanguages, "ui.hide_islands": d.UI.HideIslands, "ui.path_filter": d.UI.PathFilter,
	} {
		v.SetDefault(key, value)
	}
}

// uiDefaults applies --ui-default pairs, which are exactly that: defaults. They replace
// what depphunter would otherwise start a view at, and every config file, environment
// variable and flag beats them.
//
// That is what separates them from --theme and the rest, and it is the whole point of
// having both. An editor can say what a repository should look like before anyone has
// said otherwise in it, and still never overrule what the map's own Save button wrote
// into that repository's ui: section - which a flag would, silently and for good.
//
// Only the keys one value can seed are accepted; a list is a thing to write in the
// file rather than to spell on a command line.
//
// Implements: REQ-CFG-016, REQ-CFG-017
func uiDefaults(v *viper.Viper, pairs []string) error {
	for _, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" {
			return fmt.Errorf("ui-default %q: expected key=value", pair)
		}
		switch key {
		case "show_std":
			on, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("ui-default %s: %w", key, err)
			}
			v.SetDefault("ui."+key, on)
		case "expand_depth":
			depth, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("ui-default %s: %w", key, err)
			}
			v.SetDefault("ui."+key, depth)
		case "theme", "color_by", "height_scale", "style", "tool", "path_filter":
			v.SetDefault("ui."+key, value)
		default:
			return fmt.Errorf("ui-default %q: not a view setting a single value can seed", key)
		}
	}
	return nil
}

// mergeFile overlays the YAML file onto v; keys absent from the file keep their value.
//
// Implements: REQ-CFG-004, REQ-CFG-010
func mergeFile(v *viper.Viper, name string, required, trusted bool) error {
	data, err := readConfigFile(name, trusted)
	// A project file that is a link out of the repository is as good as none.
	if (errors.Is(err, os.ErrNotExist) || errors.Is(err, lang.ErrOutside)) && !required {
		return nil
	}
	if err != nil {
		return err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	// Viper matches keys without regard to case, so the rules below see the keys as it
	// will: `Online:` is `online:`.
	m = lowercaseKeys(m)
	if !trusted {
		// Implements: REQ-CFG-018, REQ-SUP-020, REQ-SUP-041, REQ-SUP-043
		for key := range m {
			if !projectKeys[key] {
				delete(m, key)
			}
		}
		// A repository may point at its own scanner reports, which is how a project
		// ships the output its CI already produces - but only at paths inside itself.
		if list, ok := m["findings"]; ok {
			m["findings"] = confine(list)
		}
	}
	// A list adds to what an earlier file set rather than replacing it, so a project
	// file cannot drop the user's own globs or private patterns.
	//
	// Implements: REQ-CFG-009, REQ-SUP-041
	for _, key := range []string{"exclude", "findings", "private", "trust_indexes"} {
		if list, ok := m[key]; ok {
			m[key] = append(v.GetStringSlice(key), stringList(list)...)
		}
	}
	return v.MergeConfigMap(m)
}

// projectKeys are the top-level settings a project configuration file may set. That
// file comes with the repository, which may be anybody's, so a setting is left out
// unless a repository choosing it is harmless: a new setting is user-only until it is
// added here. userOnlyKeys says why each of the others is left out, and
// TestEveryKeyIsProjectSettableOrUserOnly makes every setting be in one of the two.
//
// `private` is here although it concerns what is sent where: all it can do is stop
// depphunter from naming a package to somebody else, and a repository saying "these
// are ours" is exactly who would know. `findings` is here, but confined to paths inside
// the repository.
//
// Implements: REQ-CFG-018
var projectKeys = map[string]bool{
	"addr": true, "open": true, "exclude": true, "max_file_size": true, "watch": true,
	"cache": true, "history": true, "history_commits": true, "resolve_depth": true,
	"explain": true, "private": true, "findings": true, "vulns": true, "links": true,
	"lsp": true, "lsp_timeout": true, "ui": true,
}

// userOnlyKeys are the top-level settings only the user's own configuration, the
// environment and the command line may set, with the reason a repository may not.
var userOnlyKeys = map[string]string{
	"editor": "the command this machine runs to open a file (REQ-CFG-010)",
	"online": "whether this machine asks package indexes over the network (REQ-SUP-020)",
	"python": "which interpreter's installed files this machine reads (REQ-PY-015)",
	// The whole point of marking an index is that a repository's word for its own
	// registry is not enough.
	"trust_indexes": "vouching for an index the repository names (REQ-SUP-043)",
}

// lowercaseKeys returns m with its keys lowercased. When two keys differ only in case,
// either value may win, as it would in viper.
func lowercaseKeys(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for key, value := range m {
		out[strings.ToLower(key)] = value
	}
	return out
}

// readConfigFile reads a configuration file. The project's, untrusted, comes
// with the repository and is read inside it (its directory): committed as a
// symbolic link out of the repository, it is refused rather than followed.
//
// Implements: REQ-LANG-031
func readConfigFile(name string, trusted bool) ([]byte, error) {
	if trusted {
		return os.ReadFile(name)
	}
	f, err := lang.OpenRoot(filepath.Dir(name)).Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// stringList is a list as YAML decodes it ([]any), or as confine returns it, as strings.
func stringList(list any) []string {
	switch l := list.(type) {
	case []string:
		return l
	case []any:
		out := make([]string, 0, len(l))
		for _, item := range l {
			out = append(out, fmt.Sprint(item))
		}
		return out
	}
	return nil
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
//
// Implements: REQ-MD-010
func (c Config) FindingsEnabled() bool {
	return c.Links || c.Vulnerabilities && (len(c.Findings) > 0 || c.Online)
}

// Implements: REQ-CFG-007
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
			for _, origin := range c.AllowHost {
				if !frameOrigin(origin) {
					return fmt.Errorf("allow-host: %q is not a scheme or an origin, e.g. https://example.com", origin)
				}
			}
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
//
// Implements: REQ-SEC-010
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
