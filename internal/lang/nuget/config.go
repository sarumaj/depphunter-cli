package nuget

import (
	"encoding/xml"
	"sort"
	"strconv"
	"strings"
)

// ConfigFile is one NuGet.Config, as far as package sources go: the sections that
// name them, disable them, map packages to them and hold their credentials, each
// with its <add>, <clear/> and <packageSource> elements in document order, since a
// <clear/> drops only what comes before it.
type ConfigFile struct {
	sections []element
}

// element is any element of a NuGet.Config: a section, or an entry of one.
type element struct {
	XMLName  xml.Name
	Key      string    `xml:"key,attr"`
	Value    string    `xml:"value,attr"`
	Pattern  string    `xml:"pattern,attr"`
	Children []element `xml:",any"`
}

// ParseConfig reads a NuGet.Config. A file that is not XML says nothing.
func ParseConfig(data []byte) (ConfigFile, bool) {
	var doc struct {
		Sections []element `xml:",any"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return ConfigFile{}, false
	}
	return ConfigFile{sections: doc.Sections}, true
}

// Feed is a package source of the merged configuration.
type Feed struct {
	Key, URL string
	// Layer is the index, in the list given to Merge, of the file that set the URL.
	Layer int
}

// Credential is a <packageSourceCredentials> entry.
type Credential struct {
	Username, ClearTextPassword string
	// Encrypted says the entry holds a Password, which NuGet encrypts with a key
	// only Windows holds.
	Encrypted bool
	// Types is ValidAuthenticationTypes, "" for any.
	Types string
	// Layer is the index, in the list given to Merge, of the file that holds it.
	Layer int
}

// Basic reports whether the credential may be sent as Basic credentials.
func (c Credential) Basic() bool { return basicAllowed(c.Types) }

// Settings is what a stack of NuGet.Config files says together, the way NuGet merges
// them: the files are read from the farthest (machine-wide) to the closest (the
// project's), each <add> of a closer file overriding the same key of a farther one
// and each <clear/> dropping everything the section held before it. Keys are
// compared without regard to case.
type Settings struct {
	// Feeds are the package sources in the order NuGet lists them: farthest first,
	// a key overridden by a closer file keeping its place. A disabled one is kept
	// here; see Enabled.
	Feeds []Feed
	// Cleared says a <clear/> of <packageSources> took effect, which drops the
	// nuget.org NuGet adds when no configuration names any source.
	Cleared bool
	// disabled are the keys <disabledPackageSources> disables, lower-cased.
	disabled map[string]bool
	// mapping is <packageSourceMapping>: source key (lower-cased) -> patterns.
	mapping map[string][]string
	// Credentials are by source key, lower-cased.
	Credentials map[string]Credential
}

// Merge merges files given closest first.
func Merge(files []ConfigFile) Settings {
	s := Settings{disabled: map[string]bool{}, mapping: map[string][]string{}, Credentials: map[string]Credential{}}
	for layer := len(files) - 1; layer >= 0; layer-- {
		for _, section := range files[layer].sections {
			switch section.XMLName.Local {
			case "packageSources":
				s.readSources(section, layer)
			case "disabledPackageSources":
				for _, e := range section.Children {
					switch e.XMLName.Local {
					case "clear":
						clear(s.disabled)
					case "add":
						if strings.EqualFold(strings.TrimSpace(e.Value), "true") {
							s.disabled[strings.ToLower(e.Key)] = true
						} else {
							delete(s.disabled, strings.ToLower(e.Key))
						}
					}
				}
			case "packageSourceMapping":
				for _, e := range section.Children {
					switch e.XMLName.Local {
					case "clear":
						clear(s.mapping)
					case "packageSource":
						var patterns []string
						for _, p := range e.Children {
							if p.XMLName.Local == "package" && strings.TrimSpace(p.Pattern) != "" {
								patterns = append(patterns, strings.TrimSpace(p.Pattern))
							}
						}
						s.mapping[strings.ToLower(e.Key)] = patterns
					}
				}
			case "packageSourceCredentials":
				for _, e := range section.Children {
					if e.XMLName.Local == "clear" {
						clear(s.Credentials)
						continue
					}
					c := Credential{Layer: layer}
					for _, keyValue := range e.Children {
						switch strings.ToLower(keyValue.Key) {
						case "username":
							c.Username = keyValue.Value
						case "cleartextpassword":
							c.ClearTextPassword = keyValue.Value
						case "password":
							c.Encrypted = keyValue.Value != ""
						case "validauthenticationtypes":
							c.Types = keyValue.Value
						}
					}
					s.Credentials[strings.ToLower(DecodeName(e.XMLName.Local))] = c
				}
			}
		}
	}
	return s
}

func (s *Settings) readSources(section element, layer int) {
	for _, e := range section.Children {
		switch e.XMLName.Local {
		case "clear":
			s.Feeds, s.Cleared = nil, true
		case "add":
			key, u := strings.TrimSpace(e.Key), strings.TrimSpace(e.Value)
			if key == "" || u == "" {
				continue
			}
			i := s.index(key)
			if i < 0 {
				s.Feeds = append(s.Feeds, Feed{Key: key, URL: u, Layer: layer})
			} else {
				s.Feeds[i].URL, s.Feeds[i].Layer = u, layer
			}
		}
	}
}

// index is the position of the feed with a key, -1 for none.
func (s Settings) index(key string) int {
	for i, f := range s.Feeds {
		if strings.EqualFold(f.Key, key) {
			return i
		}
	}
	return -1
}

// Lookup is the feed of a key.
func (s Settings) Lookup(key string) (Feed, bool) {
	if i := s.index(key); i >= 0 {
		return s.Feeds[i], true
	}
	return Feed{}, false
}

// Enabled reports whether <disabledPackageSources> leaves a source key enabled.
func (s Settings) Enabled(key string) bool { return !s.disabled[strings.ToLower(key)] }

// Mapped reports whether a packageSourceMapping is in force: NuGet then restores
// only packages some pattern covers.
func (s Settings) Mapped() bool { return len(s.mapping) > 0 }

// Route is <packageSourceMapping> for one package id: the keys of the sources whose
// pattern for it is the most specific - an exact id over any prefix, a longer prefix
// (`Contoso.*`) over a shorter one, and `*` last - in the order of Feeds (keys no
// feed has after them, sorted). ok is false when no pattern matches the id, or the
// configuration maps nothing.
//
// Implements: REQ-SUP-065
func (s Settings) Route(id string) (keys []string, ok bool) {
	best := -1
	for key, patterns := range s.mapping {
		score := -1
		for _, p := range patterns {
			score = max(score, specificity(p, id))
		}
		switch {
		case score < 0 || score < best:
			continue
		case score > best:
			best, keys = score, nil
		}
		keys = append(keys, key)
	}
	if best < 0 {
		return nil, false
	}
	order := func(key string) int {
		if i := s.index(key); i >= 0 {
			return i
		}
		return len(s.Feeds)
	}
	sort.Slice(keys, func(i, j int) bool {
		if a, b := order(keys[i]), order(keys[j]); a != b {
			return a < b
		}
		return keys[i] < keys[j]
	})
	return keys, true
}

// specificity is how closely a pattern matches a package id, -1 for not at all.
// A pattern is an id, a prefix ending in `*`, or `*` alone; NuGet accepts no other
// wildcard, so a pattern with one elsewhere matches nothing.
func specificity(pattern, id string) int {
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		if !strings.Contains(prefix, "*") && len(id) >= len(prefix) && strings.EqualFold(id[:len(prefix)], prefix) {
			return len(prefix)
		}
		return -1
	}
	if !strings.Contains(pattern, "*") && strings.EqualFold(pattern, id) {
		return 1 << 20
	}
	return -1
}

// DecodeName reverses the escaping NuGet applies to a source key to make it an XML
// element name (XmlConvert.EncodeLocalName): each character a name may not hold is
// written `_xHHHH_`, a space as `_x0020_`.
func DecodeName(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); {
		if i+7 <= len(name) && name[i] == '_' && name[i+1] == 'x' && name[i+6] == '_' {
			if r, err := strconv.ParseUint(name[i+2:i+6], 16, 32); err == nil {
				b.WriteRune(rune(r))
				i += 7
				continue
			}
		}
		b.WriteByte(name[i])
		i++
	}
	return b.String()
}

// EnvironmentCredential reads the value of a NuGetPackageSourceCredentials_<source> variable:
// `Username=<user>;Password=<password>`, perhaps with
// `ValidAuthenticationTypes=<types>`. The password is required, and when the types
// are given they must allow basic.
//
// Implements: REQ-AUTH-022
func EnvironmentCredential(v string) (user, password string, ok bool) {
	var types string
	for _, part := range strings.Split(v, ";") {
		k, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "username":
			user = value
		case "password":
			password = value
		case "validauthenticationtypes":
			types = value
		}
	}
	return user, password, password != "" && basicAllowed(types)
}

// basicAllowed reports whether a ValidAuthenticationTypes value allows basic.
func basicAllowed(types string) bool {
	if strings.TrimSpace(types) == "" {
		return true
	}
	for _, t := range strings.Split(types, ",") {
		if strings.EqualFold(strings.TrimSpace(t), "basic") {
			return true
		}
	}
	return false
}
