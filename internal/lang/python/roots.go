package python

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tidwall/jsonc"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

// importRoot is a directory absolute imports are looked up under, and the directory
// whose files it serves: "" for every file, else a Pyright execution environment's
// root.
type importRoot struct {
	directory string
	scope     string
}

// serves reports whether the root applies to an import made in file.
func (root importRoot) serves(file string) bool {
	return root.scope == "" || root.scope == "." || strings.HasPrefix(file, root.scope+"/")
}

// declaredRoot is an import root a project file or the environment declares, before
// it takes its place among the others.
type declaredRoot struct {
	importRoot
	// rank orders the declarations: the process environment first, then the files
	// of deeper directories, as the more specific.
	rank int
}

// rootFinder gathers the import roots Python tools are told about beyond the ones a
// project's layout implies: PYTHONPATH (the process's, a .env file's, a VS Code
// terminal's), the extra paths of VS Code, Pyright and mypy, pytest's pythonpath,
// and the source directories packaging tools build from. Only directories inside
// the repository that hold Python files count; an entry outside it, or naming
// nothing, is dropped.
//
// Implements: REQ-PY-016
type rootFinder struct {
	resolver *resolver
	// absolute is the analyzed repository root, for absolute entries and for
	// ${workspaceFolder}, ${PWD} and $MYPY_CONFIG_FILE_DIR.
	absolute string
	// packaging are the source roots of packaging tools: a project's own roots.
	packaging []declaredRoot
	// configured are PYTHONPATH entries and the tools' extra paths.
	configured []declaredRoot
	// environmentFiles are the .env files read, so none is read twice.
	environmentFiles map[string]bool
}

// rankOf is how deep the directory a declaring file configures sits (a
// .vscode/settings.json configures the folder holding .vscode): deeper ones are more
// specific.
func rankOf(file string) int {
	if file == "" {
		return 1 << 30 // the process environment, which is what actually runs
	}
	directory := path.Dir(file)
	if path.Base(directory) == ".vscode" {
		directory = path.Dir(directory)
	}
	if directory == "." {
		return 0
	}
	return strings.Count(directory, "/") + 1
}

// inside maps a declared entry to a repository directory holding Python files:
// relative entries against base (a repository directory), absolute ones against
// the repository root. ok is false for anything outside the repository or holding
// no Python file (pyDirectories holds only repository directories that do), and for
// the repository root itself, which is always a root.
func (finder *rootFinder) inside(base, entry string) (string, bool) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", false
	}
	if filepath.IsAbs(entry) {
		relative, err := filepath.Rel(finder.absolute, entry)
		if err != nil {
			return "", false
		}
		base, entry = ".", filepath.ToSlash(relative)
	} else if strings.HasPrefix(entry, "/") || strings.HasPrefix(entry, `\`) || strings.HasPrefix(entry, "~") {
		return "", false // another platform's absolute path, or a home directory
	}
	directory := path.Join(base, strings.ReplaceAll(entry, `\`, "/"))
	if directory == "." || !finder.resolver.pyDirectories[directory] {
		return "", false
	}
	return directory, true
}

// add records entry, declared by file (source says how, for --explain), as a root
// of list.
func (finder *rootFinder) add(list *[]declaredRoot, file, source, base, entry, scope string) {
	directory, ok := finder.inside(base, entry)
	if !ok {
		return
	}
	*list = append(*list, declaredRoot{importRoot{directory, scope}, rankOf(file)})
	message := fmt.Sprintf("%s adds %s to the import roots", source, directory)
	if scope != "" && scope != "." {
		message += " of the files under " + scope
	}
	finder.resolver.Note(file, trace.NoteImportRoot, message)
}

// configure and project record a configured root and a packaging source root.
func (finder *rootFinder) configure(file, source, base, entry, scope string) {
	finder.add(&finder.configured, file, source, base, entry, scope)
}

func (finder *rootFinder) project(file, source, base, entry string) {
	finder.add(&finder.packaging, file, source, base, entry, "")
}

// environment reads the PYTHONPATH of the process running depphunter: its entries
// are this machine's paths, and relative ones are taken against the repository
// root, the usual working directory.
func (finder *rootFinder) environment(getenv func(string) string) {
	if getenv == nil {
		return
	}
	for _, entry := range filepath.SplitList(getenv("PYTHONPATH")) {
		finder.configure("", "PYTHONPATH", ".", entry, "")
	}
}

// splitPathList splits a PYTHONPATH written in a project file, which may have been
// written on either kind of system: on ";" when there is one, else on ":" except
// after a drive letter.
func splitPathList(value string) []string {
	if strings.Contains(value, ";") {
		return strings.Split(value, ";")
	}
	var entries []string
	start := 0
	for i := 0; i < len(value); i++ {
		if value[i] != ':' {
			continue
		}
		if i-start == 1 && i+1 < len(value) && (value[i+1] == '/' || value[i+1] == '\\') {
			continue // C:\ or C:/
		}
		entries = append(entries, value[start:i])
		start = i + 1
	}
	return append(entries, value[start:])
}

// dotenv reads a .env file's PYTHONPATH (relative entries against the file's
// directory). file is a repository path.
func (finder *rootFinder) dotenv(file, source string) {
	if finder.environmentFiles[file] {
		return
	}
	finder.environmentFiles[file] = true
	data, err := os.ReadFile(filepath.Join(finder.absolute, filepath.FromSlash(file)))
	if err != nil {
		return
	}
	value, ok := dotenvValue(string(data), "PYTHONPATH", map[string]string{"workspaceFolder": finder.absolute, "PWD": finder.absolute})
	if !ok {
		return
	}
	for _, entry := range splitPathList(value) {
		finder.configure(file, source, path.Dir(file), entry, "")
	}
}

var dotenvReference = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

// dotenvValue is the value a .env file gives key, read as python-dotenv reads it:
// "export " prefixes, single quotes (literal), double quotes (escapes), unquoted
// values up to a " #" comment, quoted values spanning lines, and ${NAME} or
// ${NAME:-default} references to keys set earlier in the file or to predefined.
// The other keys' values stay in this function: a .env file often holds secrets.
//
// Implements: REQ-PY-016
func dotenvValue(data, key string, predefined map[string]string) (string, bool) {
	values := map[string]string{}
	expand := func(value string) string {
		return dotenvReference.ReplaceAllStringFunc(value, func(reference string) string {
			match := dotenvReference.FindStringSubmatch(reference)
			if v, ok := values[match[1]]; ok {
				return v
			}
			if v, ok := predefined[match[1]]; ok {
				return v
			}
			return match[2]
		})
	}
	rest := data
	for {
		rest = strings.TrimLeft(rest, " \t\r\n")
		if rest == "" {
			break
		}
		line, _, _ := strings.Cut(rest, "\n")
		equals := strings.Index(line, "=")
		if strings.HasPrefix(line, "#") || equals < 0 {
			_, rest, _ = strings.Cut(rest, "\n")
			continue
		}
		name := strings.TrimSpace(line[:equals])
		if trimmed, ok := strings.CutPrefix(name, "export"); ok && trimmed != strings.TrimLeft(trimmed, " \t") {
			name = strings.TrimSpace(trimmed)
		}
		name = strings.Trim(name, `'"`)
		rest = strings.TrimLeft(rest[equals+1:], " \t")
		var value string
		if rest != "" && (rest[0] == '\'' || rest[0] == '"') {
			quote := rest[0]
			end := closingQuote(rest, quote)
			if end < 0 {
				_, rest, _ = strings.Cut(rest, "\n")
				continue // unterminated: python-dotenv skips it too
			}
			value = unescape(rest[1:end], quote)
			if quote == '"' {
				value = expand(value)
			}
			_, rest, _ = strings.Cut(rest[end+1:], "\n") // a comment may follow
		} else {
			value, rest, _ = strings.Cut(rest, "\n")
			if i := strings.Index(value, " #"); i >= 0 {
				value = value[:i]
			}
			if i := strings.Index(value, "\t#"); i >= 0 {
				value = value[:i]
			}
			value = expand(strings.TrimSpace(value))
		}
		values[name] = value
	}
	value, ok := values[key]
	return value, ok
}

// closingQuote is the index of the quote closing the value quoted at s[0], or -1.
func closingQuote(s string, quote byte) int {
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case quote:
			return i
		}
	}
	return -1
}

// unescape decodes the escapes python-dotenv decodes: \\ and the quote in single
// quotes, the usual character escapes in double quotes.
func unescape(s string, quote byte) string {
	var builder strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			builder.WriteByte(s[i])
			continue
		}
		next := s[i+1]
		switch {
		case next == '\\' || next == quote:
			builder.WriteByte(next)
		case quote == '"' && next == 'n':
			builder.WriteByte('\n')
		case quote == '"' && next == 't':
			builder.WriteByte('\t')
		case quote == '"' && next == 'r':
			builder.WriteByte('\r')
		case quote == '"' && next == '\'':
			builder.WriteByte('\'')
		default:
			builder.WriteByte('\\')
			builder.WriteByte(next)
		}
		i++
	}
	return builder.String()
}

var vscodeReference = regexp.MustCompile(`\$\{(workspaceFolder|env:[^}]*)\}`)

// vscode reads directory/.vscode/settings.json (JSON with comments and trailing
// commas), whose ${workspaceFolder} is directory: the extra paths of Pylance and of
// the older language server, the PYTHONPATH of the file python.envFile names (the
// directory's .env by default, read anyway), and the PYTHONPATH the integrated
// terminal is given on each system. ${env:NAME} expands to nothing.
func (finder *rootFinder) vscode(directory string) {
	file := path.Join(directory, ".vscode", "settings.json")
	data, err := os.ReadFile(filepath.Join(finder.absolute, filepath.FromSlash(file)))
	if err != nil {
		return
	}
	var settings map[string]json.RawMessage
	if json.Unmarshal(jsonc.ToJSON(data), &settings) != nil {
		return
	}
	workspace := filepath.Join(finder.absolute, filepath.FromSlash(directory))
	expand := func(value string) string {
		return vscodeReference.ReplaceAllStringFunc(value, func(reference string) string {
			if reference == "${workspaceFolder}" {
				return workspace
			}
			return ""
		})
	}
	for _, key := range []string{"python.analysis.extraPaths", "python.autoComplete.extraPaths"} {
		var entries []string
		if json.Unmarshal(settings[key], &entries) == nil {
			for _, entry := range entries {
				finder.configure(file, key, directory, expand(entry), "")
			}
		}
	}
	var environmentFile string
	if json.Unmarshal(settings["python.envFile"], &environmentFile) == nil && environmentFile != "" {
		if named, ok := finder.insideFile(directory, expand(environmentFile)); ok {
			finder.dotenv(named, "PYTHONPATH in "+named+" (python.envFile)")
		}
	}
	for _, system := range []string{"linux", "osx", "windows"} {
		var environment map[string]string
		key := "terminal.integrated.env." + system
		if json.Unmarshal(settings[key], &environment) == nil && environment["PYTHONPATH"] != "" {
			for _, entry := range splitPathList(expand(environment["PYTHONPATH"])) {
				finder.configure(file, key+" PYTHONPATH", directory, entry, "")
			}
		}
	}
}

// insideFile maps a file a setting names to its repository path, when it is inside.
func (finder *rootFinder) insideFile(base, entry string) (string, bool) {
	if filepath.IsAbs(entry) {
		relative, err := filepath.Rel(finder.absolute, entry)
		if err != nil {
			return "", false
		}
		base, entry = ".", filepath.ToSlash(relative)
	}
	file := path.Join(base, strings.ReplaceAll(entry, `\`, "/"))
	if file == ".." || strings.HasPrefix(file, "../") || strings.HasPrefix(file, "/") {
		return "", false
	}
	return file, true
}

// pyrightSettings are the parts of a Pyright or basedpyright configuration that add
// import roots. An execution environment's root and its extra paths apply to the
// files under that root only; the top-level extra paths to every file.
type pyrightSettings struct {
	ExtraPaths            []string `json:"extraPaths" toml:"extraPaths"`
	ExecutionEnvironments []struct {
		Root       string   `json:"root" toml:"root"`
		ExtraPaths []string `json:"extraPaths" toml:"extraPaths"`
	} `json:"executionEnvironments" toml:"executionEnvironments"`
}

func (finder *rootFinder) pyright(file, source string, settings pyrightSettings) {
	base := path.Dir(file)
	for _, environment := range settings.ExecutionEnvironments {
		scope, ok := finder.inside(base, environment.Root)
		if !ok {
			if strings.TrimSpace(environment.Root) != "" && path.Clean(environment.Root) != "." {
				continue // a root outside the repository: nothing here runs in it
			}
			scope = base
		}
		finder.configure(file, source+" executionEnvironments root", base, environment.Root, scope)
		for _, entry := range environment.ExtraPaths {
			finder.configure(file, source+" executionEnvironments extraPaths", base, entry, scope)
		}
	}
	for _, entry := range settings.ExtraPaths {
		finder.configure(file, source+" extraPaths", base, entry, "")
	}
}

// pyrightConfig reads a pyrightconfig.json or basedpyrightconfig.json, which allow
// comments.
func (finder *rootFinder) pyrightConfig(f *scan.File) {
	data, err := os.ReadFile(f.AbsolutePath)
	if err != nil {
		return
	}
	var settings pyrightSettings
	if json.Unmarshal(jsonc.ToJSON(data), &settings) == nil {
		finder.pyright(f.Path, path.Base(f.Path), settings)
	}
}

// ini reads an INI file as Python's configparser does, closely enough for the
// keys read here: section -> lower-cased key -> value, continuation lines joined
// with newlines, whole-line and " #"/" ;" comments dropped.
func ini(absolute string) map[string]map[string]string {
	data, err := os.ReadFile(absolute)
	if err != nil {
		return nil
	}
	sections := map[string]map[string]string{}
	section, key := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		for _, marker := range []string{" #", " ;", "\t#", "\t;"} {
			if i := strings.Index(trimmed, marker); i >= 0 {
				trimmed = strings.TrimSpace(trimmed[:i])
			}
		}
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";"):
		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			section, key = strings.TrimSpace(trimmed[1:len(trimmed)-1]), ""
			if sections[section] == nil {
				sections[section] = map[string]string{}
			}
		case (line[0] == ' ' || line[0] == '\t') && key != "":
			sections[section][key] += "\n" + trimmed
		case section != "":
			separator := strings.IndexAny(trimmed, "=:")
			if separator < 0 {
				key = ""
				continue
			}
			key = strings.ToLower(strings.TrimSpace(trimmed[:separator]))
			sections[section][key] = strings.TrimSpace(trimmed[separator+1:])
		}
	}
	return sections
}

// pytest adds pytest's pythonpath, whose entries are relative to the rootdir: the
// directory of the file configuring it.
func (finder *rootFinder) pytest(file string, entries []string) {
	for _, entry := range entries {
		finder.configure(file, "pytest pythonpath", path.Dir(file), entry, "")
	}
}

// mypy adds mypy_path, whose entries are separated by commas or colons and are
// relative to the working directory mypy runs in (the repository root here), with
// $MYPY_CONFIG_FILE_DIR standing for the configuration file's directory.
func (finder *rootFinder) mypy(file string, values []string) {
	directory := filepath.Join(finder.absolute, filepath.FromSlash(path.Dir(file)))
	for _, value := range values {
		value = strings.NewReplacer("${MYPY_CONFIG_FILE_DIR}", directory, "$MYPY_CONFIG_FILE_DIR", directory).Replace(value)
		for _, part := range strings.Split(value, ",") {
			for _, entry := range splitPathList(strings.TrimSpace(part)) {
				finder.configure(file, "mypy_path", ".", entry, "")
			}
		}
	}
}

// iniFile reads the sections of pytest.ini, tox.ini, setup.cfg and mypy.ini that
// add import roots.
func (finder *rootFinder) iniFile(f *scan.File) {
	sections := ini(f.AbsolutePath)
	base := path.Base(f.Path)
	pytestSection := map[string]string{"pytest.ini": "pytest", ".pytest.ini": "pytest", "tox.ini": "pytest", "setup.cfg": "tool:pytest"}[base]
	if value, ok := sections[pytestSection]["pythonpath"]; ok && pytestSection != "" {
		finder.pytest(f.Path, strings.Fields(value))
	}
	if base != "tox.ini" && base != "pytest.ini" && base != ".pytest.ini" {
		if value, ok := sections["mypy"]["mypy_path"]; ok {
			finder.mypy(f.Path, []string{value})
		}
	}
	if base != "setup.cfg" {
		return
	}
	directory := path.Dir(f.Path)
	for _, line := range strings.Split(sections["options"]["package_dir"], "\n") {
		name, value, ok := strings.Cut(line, "=")
		if ok {
			finder.packageDirectory(f.Path, "package_dir", directory, strings.TrimSpace(name), strings.TrimSpace(value))
		}
	}
	for _, entry := range strings.Fields(sections["options.packages.find"]["where"]) {
		finder.project(f.Path, "[options.packages.find] where", directory, entry)
	}
}

// packageDirectory adds the root a package-dir mapping implies: the directory of
// the "" package, or the directory above a named package's when the mapping keeps
// the package's own path ("app" = "lib/app").
func (finder *rootFinder) packageDirectory(file, source, base, name, directory string) {
	directory = strings.TrimSuffix(strings.ReplaceAll(directory, `\`, "/"), "/")
	if name != "" {
		suffix := strings.ReplaceAll(name, ".", "/")
		if directory != suffix && !strings.HasSuffix(directory, "/"+suffix) {
			return
		}
		directory = strings.TrimSuffix(strings.TrimSuffix(directory, suffix), "/")
	}
	finder.project(file, source, base, directory)
}

var (
	setupPackageDirectory = regexp.MustCompile(`package_dir\s*=\s*\{\s*['"]['"]\s*:\s*['"]([^'"]+)['"]`)
	setupFindPackages     = regexp.MustCompile(`find_(?:namespace_)?packages\(\s*(?:where\s*=\s*)?['"]([^'"]+)['"]`)
)

// setupPy reads a literal package_dir={"": "src"} or find_packages("src").
func (finder *rootFinder) setupPy(f *scan.File) {
	data, err := os.ReadFile(f.AbsolutePath)
	if err != nil {
		return
	}
	for _, pattern := range []*regexp.Regexp{setupPackageDirectory, setupFindPackages} {
		if match := pattern.FindSubmatch(data); match != nil {
			finder.project(f.Path, "setup.py", path.Dir(f.Path), string(match[1]))
		}
	}
}

// field walks nested TOML tables.
func field(v any, keys ...string) any {
	for _, key := range keys {
		table, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = table[key]
	}
	return v
}

// stringsOf is a string, or the strings of a list.
func stringsOf(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// pyproject reads the [tool] tables of a pyproject.toml that add import roots.
func (finder *rootFinder) pyproject(f *scan.File) {
	var document struct{ Tool map[string]any }
	if _, err := toml.DecodeFile(f.AbsolutePath, &document); err != nil {
		return
	}
	tool, file, directory := document.Tool, f.Path, path.Dir(f.Path)

	for _, name := range []string{"pyright", "basedpyright"} {
		var settings pyrightSettings
		if table, ok := tool[name].(map[string]any); ok && remarshal(table, &settings) {
			finder.pyright(file, "[tool."+name+"]", settings)
		}
	}
	for _, keys := range [][]string{{"pytest", "ini_options", "pythonpath"}, {"pytest", "pythonpath"}} {
		var entries []string
		for _, value := range stringsOf(field(tool, keys...)) {
			entries = append(entries, strings.Fields(value)...)
		}
		finder.pytest(file, entries)
	}
	finder.mypy(file, stringsOf(field(tool, "mypy", "mypy_path")))

	// Packaging source roots.
	if mapping, ok := field(tool, "setuptools", "package-dir").(map[string]any); ok {
		for _, name := range slices.Sorted(maps.Keys(mapping)) {
			if value, ok := mapping[name].(string); ok {
				finder.packageDirectory(file, "[tool.setuptools] package-dir", directory, name, value)
			}
		}
	}
	for _, entry := range stringsOf(field(tool, "setuptools", "packages", "find", "where")) {
		finder.project(file, "[tool.setuptools.packages.find] where", directory, entry)
	}
	if packages, ok := field(tool, "poetry", "packages").([]any); ok {
		for _, item := range packages {
			if from, ok := field(item, "from").(string); ok {
				finder.project(file, "[tool.poetry] packages from", directory, from)
			}
		}
	}
	for _, build := range []any{field(tool, "hatch", "build"), field(tool, "hatch", "build", "targets", "wheel")} {
		for _, entry := range stringsOf(field(build, "packages")) {
			finder.project(file, "[tool.hatch.build] packages", directory, path.Dir(entry))
		}
		switch sources := field(build, "sources").(type) {
		case []any:
			for _, entry := range stringsOf(sources) {
				finder.project(file, "[tool.hatch.build] sources", directory, entry)
			}
		case map[string]any:
			for _, entry := range slices.Sorted(maps.Keys(sources)) {
				if sources[entry] == "" {
					finder.project(file, "[tool.hatch.build] sources", directory, entry)
				}
			}
		}
	}
	if entry, ok := field(tool, "pdm", "build", "package-dir").(string); ok {
		finder.project(file, "[tool.pdm.build] package-dir", directory, entry)
	}
	if entry, ok := field(tool, "maturin", "python-source").(string); ok {
		finder.project(file, "[tool.maturin] python-source", directory, entry)
	}
}

// remarshal decodes a TOML table into a struct through its JSON tags.
func remarshal(table map[string]any, out any) bool {
	data, err := json.Marshal(table)
	return err == nil && json.Unmarshal(data, out) == nil
}

// ordered lists the roots found, each directory once per scope in its first
// place: sorted by rank (stable, so a file's entries keep their order).
func ordered(roots []declaredRoot) []importRoot {
	sort.SliceStable(roots, func(i, j int) bool { return roots[i].rank > roots[j].rank })
	var out []importRoot
	for _, root := range roots {
		out = append(out, root.importRoot)
	}
	return out
}

var _ lang.Noter = (*resolver)(nil)
