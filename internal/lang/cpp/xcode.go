package cpp

import (
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/kballard/go-shellquote"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// An Xcode project keeps its include path in build settings rather than in a
// compilation database: HEADER_SEARCH_PATHS and USER_HEADER_SEARCH_PATHS, written in
// the project file (X.xcodeproj/project.pbxproj) or in .xcconfig files. Their
// entries are made project-relative: $(SRCROOT), $(PROJECT_DIR) and $(SOURCE_ROOT)
// are the directory holding X.xcodeproj, a relative entry is taken from it,
// $(inherited) adds nothing, other variables are expanded from the settings beside
// them, and an entry ending in /** searches the directories below it too. An entry
// naming anything else - an unknown variable, an absolute path, a directory outside
// the repository or one it does not have - is dropped.

// maxXcodeFile bounds a project file or .xcconfig read; a large app's project file
// runs to a few megabytes.
const maxXcodeFile = 16 << 20

// maxXcodeNesting bounds variable expansion and .xcconfig #include chains.
const maxXcodeNesting = 8

// sourceRoot stands for the project directory while a value is expanded, so that
// it is told apart from an absolute path the value spells out; unexpanded marks a
// variable that could not be expanded.
const (
	sourceRoot = "\x00"
	unexpanded = "\x01"
)

// searchDirectory is one header search path entry: a project directory ("" the
// root), and whether the directories below it are searched too.
type searchDirectory struct {
	directory string
	recursive bool
}

// xcodePaths holds the header search paths of each Xcode project, by project
// directory.
type xcodePaths struct {
	byProject map[string][]searchDirectory
	projects  []string // project directories, deepest first
}

var (
	xcodeSearchKey = regexp.MustCompile(`^(?:USER_)?HEADER_SEARCH_PATHS$`)
	xcodeVariable  = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)(:[^)}]*)?[)}]`)
	xcodeInclude   = regexp.MustCompile(`^#include\??\s*"([^"]+)"`)
	// xcodeReference is a file reference's path or name in a project file.
	xcodeReference = regexp.MustCompile(`\b(?:path|name)\s*=\s*("(?:[^"\\]|\\.)*"|[^;\s]+)\s*;`)
)

// readXcode reads the header search paths of the project files and .xcconfig files
// among the scanned files. An .xcconfig file belongs to the projects whose file
// references name it, else to the project in its directory or the nearest one
// above, else to the repository's root; one that another includes is read only as
// part of that one.
//
// Implements: REQ-OBJC-015
func readXcode(all []*scan.File) *xcodePaths {
	x := &xcodePaths{byProject: map[string][]searchDirectory{}}
	directories := map[string]bool{"": true}
	absolute := map[string]string{}
	var projectFiles, xcconfigFiles []string
	for _, scanned := range all {
		for directory := path.Dir(scanned.Path); directory != "." && !directories[directory]; directory = path.Dir(directory) {
			directories[directory] = true
		}
		if scanned.Binary || scanned.TooLarge {
			continue
		}
		absolute[scanned.Path] = scanned.Abs
		switch {
		case path.Base(scanned.Path) == "project.pbxproj" && strings.HasSuffix(path.Dir(scanned.Path), ".xcodeproj"):
			projectFiles = append(projectFiles, scanned.Path)
		case strings.HasSuffix(scanned.Path, ".xcconfig"):
			xcconfigFiles = append(xcconfigFiles, scanned.Path)
		}
	}
	add := func(project string, values []string) {
		for _, entry := range searchEntries(project, values, directories) {
			if !slices.Contains(x.byProject[project], entry) {
				x.byProject[project] = append(x.byProject[project], entry)
			}
		}
	}
	projectDirectories := map[string]bool{}
	referencing := map[string][]string{} // .xcconfig file name -> the projects naming it
	for _, file := range projectFiles {
		project := relativeDirectory(path.Dir(path.Dir(file)))
		projectDirectories[project] = true
		source, ok := readXcodeFile(absolute[file])
		if !ok {
			continue
		}
		settings, paths := projectSettings(source)
		add(project, expandAll(paths, settings))
		for _, match := range xcodeReference.FindAllStringSubmatch(source, -1) {
			if name := path.Base(unquotePropertyList(match[1])); strings.HasSuffix(name, ".xcconfig") &&
				!slices.Contains(referencing[name], project) {
				referencing[name] = append(referencing[name], project)
			}
		}
	}
	type xcconfig struct {
		settings map[string]string
		paths    []string
	}
	read := map[string]xcconfig{}
	included := map[string]bool{}
	for _, file := range xcconfigFiles {
		configuration := xcconfig{settings: map[string]string{}}
		seen := map[string]bool{}
		readXcconfig(file, absolute, configuration.settings, &configuration.paths, seen, 0)
		read[file] = configuration
		for other := range seen {
			included[other] = included[other] || other != file
		}
	}
	for _, file := range xcconfigFiles {
		if included[file] {
			continue // read with the file that includes it, for that file's projects
		}
		projects := referencing[path.Base(file)]
		if len(projects) == 0 {
			project := relativeDirectory(path.Dir(file))
			for !projectDirectories[project] && project != "" {
				project = relativeDirectory(path.Dir(project))
			}
			projects = []string{project}
		}
		for _, project := range projects {
			add(project, expandAll(read[file].paths, read[file].settings))
		}
	}
	for project := range x.byProject {
		x.projects = append(x.projects, project)
	}
	sort.Slice(x.projects, func(i, j int) bool {
		if left, right := strings.Count(x.projects[i], "/"), strings.Count(x.projects[j], "/"); left != right {
			return left > right
		}
		return x.projects[i] < x.projects[j]
	})
	return x
}

// relativeDirectory turns path.Dir's "." for the root into "".
func relativeDirectory(directory string) string {
	if directory == "." {
		return ""
	}
	return directory
}

func readXcodeFile(absolutePath string) (string, bool) {
	info, err := os.Stat(absolutePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxXcodeFile {
		return "", false
	}
	data, err := os.ReadFile(absolutePath)
	return string(data), err == nil
}

// find looks an include up in the header search path of a file: the entries of the
// projects whose directory holds it, the innermost project's first.
//
// Implements: REQ-CPP-004, REQ-OBJC-015
func (x *xcodePaths) find(file, name string, files map[string]bool, byBase map[string][]string) string {
	for _, project := range x.projects {
		if project != "" && !strings.HasPrefix(file, project+"/") {
			continue
		}
		for _, entry := range x.byProject[project] {
			if found := entry.find(name, files, byBase); found != "" {
				return found
			}
		}
	}
	return ""
}

// projectSettings reads every buildSettings dictionary of a project file: the
// values of its settings (the first one met of each name, a list's items joined by
// spaces) and, in order, every header search path value.
func projectSettings(source string) (settings map[string]string, paths []string) {
	settings = map[string]string{}
	const marker = "buildSettings"
	for position := 0; ; {
		found := strings.Index(source[position:], marker)
		if found < 0 {
			return settings, paths
		}
		list := &propertyList{source: source, position: position + found + len(marker)}
		position = list.position
		list.space()
		if !list.eat('=') {
			continue
		}
		list.space()
		if !list.eat('{') {
			continue
		}
		for {
			list.space()
			if list.position >= len(source) || list.eat('}') {
				break
			}
			key, ok := list.word()
			list.space()
			if !ok || !list.eat('=') {
				break
			}
			value, ok := list.value()
			list.space()
			if !ok || !list.eat(';') {
				break
			}
			name, _, _ := strings.Cut(key, "[") // conditions: [sdk=*], [config=Debug]
			if xcodeSearchKey.MatchString(name) {
				paths = append(paths, value)
			} else if _, seen := settings[name]; !seen {
				settings[name] = value
			}
		}
		position = max(position, list.position)
	}
}

// propertyList reads the old-style property list of a project file.
type propertyList struct {
	source   string
	position int
}

// space skips white space and comments.
func (l *propertyList) space() {
	for l.position < len(l.source) {
		rest := l.source[l.position:]
		switch {
		case strings.IndexByte(" \t\r\n", rest[0]) >= 0:
			l.position++
		case strings.HasPrefix(rest, "/*"):
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				l.position = len(l.source)
				return
			}
			l.position += end + 4
		case strings.HasPrefix(rest, "//"):
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				l.position = len(l.source)
				return
			}
			l.position += end
		default:
			return
		}
	}
}

func (l *propertyList) eat(character byte) bool {
	if l.position < len(l.source) && l.source[l.position] == character {
		l.position++
		return true
	}
	return false
}

// word reads a quoted string, with its escapes resolved, or a bare word.
func (l *propertyList) word() (string, bool) {
	if l.eat('"') {
		var text strings.Builder
		for l.position < len(l.source) {
			character := l.source[l.position]
			l.position++
			switch character {
			case '"':
				return text.String(), true
			case '\\':
				if l.position < len(l.source) {
					text.WriteByte(unescape(l.source[l.position]))
					l.position++
				}
			default:
				text.WriteByte(character)
			}
		}
		return "", false
	}
	start := l.position
	for l.position < len(l.source) && bareWordByte(l.source[l.position]) {
		l.position++
	}
	return l.source[start:l.position], l.position > start
}

// value reads a setting's value: a word, or a list's items joined by spaces.
func (l *propertyList) value() (string, bool) {
	l.space()
	if !l.eat('(') {
		return l.word()
	}
	var items []string
	for {
		l.space()
		if l.eat(')') {
			return strings.Join(items, " "), true
		}
		item, ok := l.word()
		if !ok {
			return "", false
		}
		items = append(items, item)
		l.space()
		l.eat(',')
	}
}

// unquotePropertyList reads a quoted or bare property list word.
func unquotePropertyList(text string) string {
	word, _ := (&propertyList{source: text}).word()
	return word
}

func unescape(character byte) byte {
	switch character {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	}
	return character
}

func bareWordByte(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' || strings.IndexByte("_$+/:.-", character) >= 0
}

// readXcconfig reads an .xcconfig file, and first the files it includes: the
// settings it assigns (a later assignment replaces an earlier one, whose value its
// $(inherited) stands for) and every header search path value.
func readXcconfig(file string, absolute, settings map[string]string, paths *[]string, seen map[string]bool, depth int) {
	if seen[file] || depth > maxXcodeNesting {
		return
	}
	seen[file] = true
	absolutePath, ok := absolute[file]
	if !ok {
		return
	}
	source, ok := readXcodeFile(absolutePath)
	if !ok {
		return
	}
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if match := xcodeInclude.FindStringSubmatch(line); match != nil {
			if included := path.Join(path.Dir(file), match[1]); !path.IsAbs(match[1]) && !strings.HasPrefix(included, "../") {
				readXcconfig(included, absolute, settings, paths, seen, depth+1)
			}
			continue
		}
		if comment := strings.Index(line, "//"); comment >= 0 {
			line = line[:comment] // a comment starts anywhere, even inside a value
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimSpace(key), "[")
		value = strings.TrimSpace(value)
		if xcodeSearchKey.MatchString(name) {
			*paths = append(*paths, value)
			continue
		}
		previous := settings[name]
		settings[name] = strings.TrimSpace(xcodeVariable.ReplaceAllStringFunc(value, func(reference string) string {
			if xcodeVariable.FindStringSubmatch(reference)[1] == "inherited" {
				return previous
			}
			return reference
		}))
	}
}

// expandAll expands the variables of each value.
func expandAll(values []string, settings map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, expandXcode(value, settings, 0))
	}
	return out
}

// expandXcode replaces the variables of a value: the project directory by
// sourceRoot, $(inherited) by nothing, a setting by its expanded value. A variable
// it cannot expand, or one with an operator ($(X:dir)), becomes unexpanded, which
// drops the entry.
func expandXcode(value string, settings map[string]string, depth int) string {
	return xcodeVariable.ReplaceAllStringFunc(value, func(reference string) string {
		match := xcodeVariable.FindStringSubmatch(reference)
		switch {
		case match[2] != "":
			return unexpanded
		case match[1] == "SRCROOT" || match[1] == "PROJECT_DIR" || match[1] == "SOURCE_ROOT":
			return sourceRoot
		case match[1] == "inherited":
			return ""
		}
		if setting, ok := settings[match[1]]; ok && depth < maxXcodeNesting {
			return expandXcode(setting, settings, depth+1)
		}
		return unexpanded
	})
}

// searchEntries turns expanded values into the directories of the repository they
// name, relative to the project directory unless they start at it.
func searchEntries(project string, values []string, directories map[string]bool) []searchDirectory {
	var out []searchDirectory
	for _, value := range values {
		words, err := shellquote.Split(value)
		if err != nil {
			words = strings.Fields(value)
		}
		for _, word := range words {
			entry := searchDirectory{}
			word, entry.recursive = strings.CutSuffix(word, "/**")
			if word == "**" {
				word, entry.recursive = "", true
			}
			if rest, ok := strings.CutPrefix(word, sourceRoot); ok {
				word = strings.TrimPrefix(rest, "/")
			} else if path.IsAbs(word) || strings.HasPrefix(word, "~") || len(word) > 1 && word[1] == ':' {
				continue // this machine's, not the repository's
			}
			if strings.ContainsAny(word, sourceRoot+unexpanded+`\$`) {
				continue
			}
			directory := relativeDirectory(path.Join(project, word))
			if directory == ".." || strings.HasPrefix(directory, "../") || !directories[directory] {
				continue
			}
			entry.directory = directory
			out = append(out, entry)
		}
	}
	return out
}

// find looks an include up in a search entry: the file at directory/name, or for a
// recursive entry, the shallowest file below the directory whose path ends in name.
func (entry searchDirectory) find(name string, files map[string]bool, byBase map[string][]string) string {
	if candidate := path.Join(entry.directory, name); files[candidate] && !strings.HasPrefix(candidate, "../") {
		return candidate
	}
	if !entry.recursive {
		return ""
	}
	var best string
	for _, candidate := range byBase[path.Base(name)] {
		if !strings.HasSuffix("/"+candidate, "/"+name) {
			continue
		}
		if entry.directory != "" && !strings.HasPrefix(candidate, entry.directory+"/") {
			continue
		}
		depth, bestDepth := strings.Count(candidate, "/"), strings.Count(best, "/")
		if best == "" || depth < bestDepth || depth == bestDepth && candidate < best {
			best = candidate
		}
	}
	return best
}
