// Package config resolves settings from, in increasing precedence: defaults, the user
// config file, the project config file, DEPPHUNTER_* environment variables, and flags.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const ProjectFile = ".depphunter.yaml"

// UI holds the initial state of settings the user can also change in the browser.
type UI struct {
	Theme       string `yaml:"theme" json:"theme"`              // auto | light | dark
	ColorBy     string `yaml:"color_by" json:"colorBy"`         // language | size
	HeightScale string `yaml:"height_scale" json:"heightScale"` // linear | sqrt | log
	ShowStd     bool   `yaml:"show_std" json:"showStd"`
	ExpandDepth int    `yaml:"expand_depth" json:"expandDepth"` // 0 = auto, -1 = everything
}

type Config struct {
	Root        string   `yaml:"-"`
	Addr        string   `yaml:"addr"`
	Open        bool     `yaml:"open"`
	Exclude     []string `yaml:"exclude"`
	MaxFileSize int64    `yaml:"max_file_size"`
	UI          UI       `yaml:"ui"`
}

func Default() Config {
	return Config{
		Addr:        "127.0.0.1:0",
		Open:        true,
		MaxFileSize: 2 << 20,
		UI:          UI{Theme: "auto", ColorBy: "language", HeightScale: "sqrt"},
	}
}

// ErrHelp is returned when -h was requested; usage has already been printed.
var ErrHelp = flag.ErrHelp

// Load builds the configuration. getenv and userDir are injected for testability;
// userDir is the directory holding the user-level config (e.g. ~/.config/depphunter).
func Load(args []string, getenv func(string) string, userDir string, usage io.Writer) (Config, error) {
	cfg := Default()

	fs := flag.NewFlagSet("depphunter", flag.ContinueOnError)
	fs.SetOutput(usage)
	fs.Usage = func() {
		fmt.Fprintf(usage, "Usage: depphunter [flags] [path]\n\nOpens an interactive map of the code base at path (default: current directory).\n\nFlags:\n")
		fs.PrintDefaults()
	}
	var (
		flagConfig  = fs.String("config", "", "config file to use instead of <path>/"+ProjectFile)
		flagAddr    = fs.String("addr", cfg.Addr, "listen address (port 0 picks a free port)")
		flagNoOpen  = fs.Bool("no-open", false, "do not open the browser")
		flagMaxSize = fs.Int64("max-file-size", cfg.MaxFileSize, "files larger than this many bytes are not read")
		flagTheme   = fs.String("theme", cfg.UI.Theme, "colour theme: auto, light, dark")
		flagColorBy = fs.String("color-by", cfg.UI.ColorBy, "building colour: language, size")
		flagHeight  = fs.String("height-scale", cfg.UI.HeightScale, "building height scale: linear, sqrt, log")
		flagShowStd = fs.Bool("show-std", cfg.UI.ShowStd, "show standard-library islands")
		flagDepth   = fs.Int("expand-depth", cfg.UI.ExpandDepth, "initially expanded directory depth (0 = auto, -1 = all)")
		flagExclude stringList
	)
	fs.Var(&flagExclude, "exclude", "glob of paths to skip; repeatable")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() > 1 {
		return cfg, fmt.Errorf("expected at most one path, got %d", fs.NArg())
	}

	root := "."
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return cfg, err
	}
	// Walking a symlinked root would yield nothing; analyse the directory it points to.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return cfg, fmt.Errorf("%s is not a directory", root)
	}
	cfg.Root = root

	if userDir != "" {
		if err := mergeFile(&cfg, filepath.Join(userDir, "config.yaml"), false); err != nil {
			return cfg, err
		}
	}
	if *flagConfig != "" {
		err = mergeFile(&cfg, *flagConfig, true)
	} else {
		err = mergeFile(&cfg, filepath.Join(root, ProjectFile), false)
	}
	if err != nil {
		return cfg, err
	}
	if err := mergeEnv(&cfg, getenv); err != nil {
		return cfg, err
	}

	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			cfg.Addr = *flagAddr
		case "no-open":
			cfg.Open = !*flagNoOpen
		case "max-file-size":
			cfg.MaxFileSize = *flagMaxSize
		case "theme":
			cfg.UI.Theme = *flagTheme
		case "color-by":
			cfg.UI.ColorBy = *flagColorBy
		case "height-scale":
			cfg.UI.HeightScale = *flagHeight
		case "show-std":
			cfg.UI.ShowStd = *flagShowStd
		case "expand-depth":
			cfg.UI.ExpandDepth = *flagDepth
		case "exclude":
			cfg.Exclude = append(cfg.Exclude, flagExclude...)
		}
	})
	return cfg, cfg.validate()
}

// mergeFile overlays the YAML file onto cfg; keys absent from the file keep their value.
func mergeFile(cfg *Config, name string, required bool) error {
	data, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) && !required {
		return nil
	}
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func mergeEnv(cfg *Config, getenv func(string) string) error {
	str := func(key string, dst *string) {
		if v := getenv("DEPPHUNTER_" + key); v != "" {
			*dst = v
		}
	}
	boolean := func(key string, dst *bool) error {
		if v := getenv("DEPPHUNTER_" + key); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("DEPPHUNTER_%s: %w", key, err)
			}
			*dst = b
		}
		return nil
	}
	str("ADDR", &cfg.Addr)
	str("THEME", &cfg.UI.Theme)
	str("COLOR_BY", &cfg.UI.ColorBy)
	str("HEIGHT_SCALE", &cfg.UI.HeightScale)
	if v := getenv("DEPPHUNTER_EXCLUDE"); v != "" {
		cfg.Exclude = append(cfg.Exclude, strings.Split(v, ",")...)
	}
	if err := boolean("OPEN", &cfg.Open); err != nil {
		return err
	}
	return boolean("SHOW_STD", &cfg.UI.ShowStd)
}

func (c Config) validate() error {
	oneOf := func(name, v string, allowed ...string) error {
		for _, a := range allowed {
			if v == a {
				return nil
			}
		}
		return fmt.Errorf("invalid %s %q (want one of %s)", name, v, strings.Join(allowed, ", "))
	}
	return errors.Join(
		oneOf("theme", c.UI.Theme, "auto", "light", "dark"),
		oneOf("color-by", c.UI.ColorBy, "language", "size"),
		oneOf("height-scale", c.UI.HeightScale, "linear", "sqrt", "log"),
	)
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }
