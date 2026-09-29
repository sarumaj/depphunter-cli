package index

import (
	"slices"
	"strings"
	"unicode"
)

// hackageName is the name cabal knows Hackage's repository stanza by, whatever URL
// the stanza gives: under it, a mirror stands in for Hackage.
const hackageName = "hackage.haskell.org"

// cabalRepository is one repository stanza of a cabal configuration or
// cabal.project: "repository <name>" with an indented "url:".
type cabalRepository struct{ name, url string }

// cabalSettings is what one cabal file says about package repositories: its
// repository stanzas in order, and its active-repositories (nil when it sets none).
type cabalSettings struct {
	repositories []cabalRepository
	active       []string
}

// readCabal reads the repository stanzas and the active-repositories field of a
// cabal configuration or cabal.project. Field names are case-insensitive, and a
// field's value may go on over the indented lines below it.
func readCabal(data []byte) cabalSettings {
	var s cabalSettings
	stanza, activeField := -1, false
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			stanza, activeField = -1, false
			if keyword, name, _ := strings.Cut(trimmed, " "); strings.EqualFold(keyword, "repository") && strings.TrimSpace(name) != "" {
				s.repositories = append(s.repositories, cabalRepository{name: strings.TrimSpace(name)})
				stanza = len(s.repositories) - 1
				continue
			}
			key, value, ok := strings.Cut(trimmed, ":")
			if ok && strings.EqualFold(strings.TrimSpace(key), "active-repositories") {
				activeField = true
				s.active = append([]string{}, cabalNames(value)...)
			}
			continue
		}
		switch {
		case activeField:
			s.active = append(s.active, cabalNames(trimmed)...)
		case stanza >= 0:
			if key, value, ok := strings.Cut(trimmed, ":"); ok && strings.EqualFold(strings.TrimSpace(key), "url") {
				s.repositories[stanza].url = strings.TrimSpace(value)
			}
		}
	}
	if len(s.active) == 0 {
		s.active = nil // an empty field sets nothing
	}
	return s
}

// cabalNames splits an active-repositories value: names separated by commas or
// white space.
func cabalNames(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
}

// addCabalRepositories records the repositories of a cabal file. cabal combines the
// packages of every repository configured, so each serves every package beside
// Hackage (Additive), asked before it; a stanza under Hackage's own name with
// another URL is a mirror that replaces it. Hackage itself is the public index and
// is not recorded, and a local repository (file:, file+noindex:) cannot be asked.
//
// Implements: REQ-SUP-049, REQ-SUP-063
func addCabalRepositories(repositories []cabalRepository, k sink) {
	for _, r := range repositories {
		u := strings.TrimSpace(r.url)
		switch {
		case !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://"), HackageItself(u):
		case r.name == hackageName:
			k.put(Hackage, Source{URL: u, cabalName: r.name})
		default:
			k.put(Hackage, Source{URL: u, Kind: Additive, cabalName: r.name})
		}
	}
}

// machineCabal reads this machine's cabal configuration. A configuration that
// lists repositories without Hackage's leaves Hackage out: cabal uses Hackage by
// default only when no repository is listed.
//
// Implements: REQ-SUP-049, REQ-SUP-063
func (c *Config) machineCabal(data []byte, k sink) {
	s := readCabal(data)
	addCabalRepositories(s.repositories, k)
	if len(s.repositories) > 0 && !slices.ContainsFunc(s.repositories, func(r cabalRepository) bool {
		return r.name == hackageName || HackageItself(r.url)
	}) {
		k.off(Hackage)
	}
	c.cabalMachine = s.active
}

// projectCabal reads a cabal.project (or cabal.project.local, which is read after
// it and wins): its repositories are added to this machine's, as cabal adds them,
// and its active-repositories replaces the configuration's.
//
// Implements: REQ-SUP-049, REQ-SUP-063
func (c *Config) projectCabal(data []byte, k sink) {
	s := readCabal(data)
	addCabalRepositories(s.repositories, k)
	if s.active != nil {
		c.cabalProject = s.active
	}
}

// cabalCandidates are the repositories active-repositories activates, when the
// repository's cabal.project or this machine's configuration sets it (ok false
// otherwise): the names it lists, ":rest" standing for every configured repository
// it does not name (Hackage first unless the configuration leaves it out) and
// ":none" for none. cabal searches the list last to first, and a repository marked
// ":override" is the sole provider of the packages it has, which it can only be when
// it is asked before those above it: the candidates are the list reversed.
//
// Implements: REQ-SUP-049, REQ-SUP-063
func (c *Config) cabalCandidates() ([]candidate, bool) {
	active := c.cabalProject
	if active == nil {
		active = c.cabalMachine
	}
	if active == nil {
		return nil, false
	}
	var configured []candidate
	names := []string{}
	hackage := false
	for _, s := range c.sources[Hackage] {
		if s.cabalName != "" {
			configured = append(configured, candidate{url: s.URL, known: c.fetchable(Hackage, s), primary: true})
			names = append(names, s.cabalName)
			hackage = hackage || s.cabalName == hackageName
		}
	}
	if !hackage && c.off[Hackage] == "" {
		configured = append([]candidate{{url: c.publicURL(Hackage), known: true, primary: true}}, configured...)
		names = append([]string{hackageName}, names...)
	}
	listed := map[string]bool{}
	for _, name := range active {
		listed[cabalRepositoryName(name)] = true
	}
	var order []candidate
	for _, name := range active {
		switch name = cabalRepositoryName(name); name {
		case ":none":
		case ":rest":
			for i, k := range configured {
				if !listed[names[i]] {
					order = append(order, k)
				}
			}
		default:
			if i := slices.Index(names, name); i >= 0 {
				order = append(order, configured[i])
			}
		}
	}
	slices.Reverse(order)
	out := []candidate{}
	seen := map[string]bool{}
	for _, k := range order {
		if !seen[k.url] {
			seen[k.url] = true
			out = append(out, k)
		}
	}
	return out, true
}

// cabalRepositoryName is an active-repositories entry without its ":override" or
// ":merge" mode.
func cabalRepositoryName(entry string) string {
	if strings.HasPrefix(entry, ":") {
		return entry
	}
	name, _, _ := strings.Cut(entry, ":")
	return name
}
