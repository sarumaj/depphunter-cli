// Package config resolves settings from, in increasing precedence: defaults, the user
// config file, the project config file, DEPPHUNTER_* environment variables, and flags.
// Flags are pflag (the cobra command registers them with RegisterFlags); files,
// environment and flags are layered with viper.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/termview"
)

const ProjectFile = ".depphunter.yaml"

// UI holds the initial state of settings the user can also change in the browser.
type UI struct {
	Theme       string `yaml:"theme" mapstructure:"theme" json:"theme"`                     // auto | light | dark
	ColorBy     string `yaml:"color_by" mapstructure:"color_by" json:"colorBy"`             // language | size | commits | churn | age | authors
	HeightScale string `yaml:"height_scale" mapstructure:"height_scale" json:"heightScale"` // linear | sqrt | log
	ShowStd     bool   `yaml:"show_std" mapstructure:"show_std" json:"showStd"`
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
	// LSP asks installed language servers for symbol-level references (slow, opt-in).
	LSP        bool          `yaml:"lsp" mapstructure:"lsp"`
	LSPTimeout time.Duration `yaml:"lsp_timeout" mapstructure:"lsp_timeout"`
	// Editor is a command template such as "code -g {file}:{line}"; empty = auto-detect.
	Editor string `yaml:"editor" mapstructure:"editor"`
	// Terminal draws the map in the terminal itself, through a headless browser.
	// TerminalBrowser names the browser to drive (empty = look for one) and
	// TerminalGraphics the protocol to draw with (see termview.Protocols).
	Terminal         bool   `yaml:"terminal" mapstructure:"terminal"`
	TerminalBrowser  string `yaml:"terminal_browser" mapstructure:"terminal_browser"`
	TerminalGraphics string `yaml:"terminal_graphics" mapstructure:"terminal_graphics"`
	UI               UI     `yaml:"ui" mapstructure:"ui"`

	// Export and Output are one-shot actions, so they only come from flags.
	Export string `yaml:"-" mapstructure:"-"`
	Output string `yaml:"-" mapstructure:"-"`
}

func Default() Config {
	return Config{
		Addr:             "127.0.0.1:0",
		Open:             true,
		Cache:            true,
		History:          true,
		HistoryCommits:   10000,
		LSPTimeout:       5 * time.Minute,
		MaxFileSize:      2 << 20,
		TerminalGraphics: "auto",
		UI:               UI{Theme: "auto", ColorBy: "language", HeightScale: "sqrt"},
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
	fs.Bool("show-std", d.UI.ShowStd, "show standard-library islands")
	fs.Int("expand-depth", d.UI.ExpandDepth, "initially expanded directory depth (0 = auto, -1 = all)")
	fs.Bool("watch", false, "re-analyze on file changes and update the browser live")
	fs.Bool("no-cache", false, "do not read or write the analysis cache")
	fs.Bool("no-history", false, "do not read git history")
	fs.Int("history-commits", d.HistoryCommits, "read at most this many commits of git history")
	fs.Int("resolve-depth", d.ResolveDepth,
		"levels of external dependencies-of-dependencies to resolve from lock files (-1 = all)")
	fs.Bool("lsp", false, "find symbol references with installed language servers (gopls, …)")
	fs.Duration("lsp-timeout", d.LSPTimeout, "time budget for language servers")
	fs.String("editor", "", `editor command template, e.g. "code -g {file}:{line}" (default: auto-detect)`)
	fs.Bool("terminal", false, "draw the map in the terminal instead of opening a browser window")
	fs.String("terminal-browser", "", "browser binary the terminal view drives (default: the first Chromium found)")
	fs.String("terminal-graphics", d.TerminalGraphics,
		"how the terminal view draws: "+strings.Join(termview.Protocols(), ", "))
	fs.String("export", "", "write the graph as json, graphml, dot or html and exit instead of serving")
	fs.StringP("output", "o", "", "output file for --export (default: stdout)")
}

// Settings a flag sets directly, by flag name.
var flagKeys = map[string]string{
	"addr": "addr", "max-file-size": "max_file_size", "watch": "watch",
	"history-commits": "history_commits", "resolve-depth": "resolve_depth", "lsp": "lsp", "lsp-timeout": "lsp_timeout", "editor": "editor",
	"terminal": "terminal", "terminal-browser": "terminal_browser", "terminal-graphics": "terminal_graphics",
	"theme": "ui.theme", "color-by": "ui.color_by", "height-scale": "ui.height_scale",
	"show-std": "ui.show_std", "expand-depth": "ui.expand_depth",
}

// Settings a --no-* flag turns off.
var negatedFlags = map[string]string{"no-open": "open", "no-cache": "cache", "no-history": "history"}

// Environment variables (after the DEPPHUNTER_ prefix), by setting. EXCLUDE is
// handled apart: it adds to the configured globs instead of replacing them.
var envKeys = map[string]string{
	"addr": "ADDR", "open": "OPEN", "max_file_size": "MAX_FILE_SIZE", "watch": "WATCH", "cache": "CACHE",
	"history": "HISTORY", "history_commits": "HISTORY_COMMITS", "resolve_depth": "RESOLVE_DEPTH", "lsp": "LSP", "lsp_timeout": "LSP_TIMEOUT",
	"editor": "EDITOR", "terminal": "TERMINAL", "terminal_browser": "TERMINAL_BROWSER",
	"terminal_graphics": "TERMINAL_GRAPHICS", "ui.theme": "THEME", "ui.color_by": "COLOR_BY", "ui.height_scale": "HEIGHT_SCALE",
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
	// choose a program this machine runs: its editor and terminal_browser keys are
	// dropped. A file named with --config is the user's own choice.
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
		"resolve_depth": d.ResolveDepth, "lsp": d.LSP, "lsp_timeout": d.LSPTimeout,
		"editor": d.Editor, "terminal": d.Terminal,
		"terminal_browser": d.TerminalBrowser, "terminal_graphics": d.TerminalGraphics,
		"ui.theme": d.UI.Theme, "ui.color_by": d.UI.ColorBy, "ui.height_scale": d.UI.HeightScale,
		"ui.show_std": d.UI.ShowStd, "ui.expand_depth": d.UI.ExpandDepth,
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
		// The keys that name a program to execute: a repository must not choose
		// what its reader's machine runs.
		delete(m, "editor")
		delete(m, "terminal_browser")
	}
	return v.MergeConfigMap(m)
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
		oneOf("terminal-graphics", c.TerminalGraphics, termview.Protocols()...),
	)
}
