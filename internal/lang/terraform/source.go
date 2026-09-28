package terraform

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// publicRegistries are the hosts of the public Terraform and OpenTofu registries.
// Both serve the same namespaces (OpenTofu's registry mirrors the providers and
// modules published to Terraform's), so an address on either is named without its
// host: hashicorp/aws is one provider whichever tool installs it.
var publicRegistries = map[string]bool{"registry.terraform.io": true, "registry.opentofu.org": true}

// registryHost is the host part of a registry address as it is named: "" for the
// public registries, else the host lower-cased.
func registryHost(host string) string {
	host = strings.ToLower(host)
	if publicRegistries[host] {
		return ""
	}
	return host
}

var (
	registryName = regexp.MustCompile(`^[0-9A-Za-z](?:[0-9A-Za-z_-]*[0-9A-Za-z])?$`)
	scpLike      = regexp.MustCompile(`^[A-Za-z0-9._~-]+@([A-Za-z0-9.-]+):(.+)$`)
)

// hostLike reports whether s can be a registry's host name.
func hostLike(s string) bool {
	return strings.Contains(s, ".") || strings.HasPrefix(s, "localhost")
}

// moduleSource is what a module source address names.
type moduleSource struct {
	local       string // a directory relative to the calling module ("./x", "../x")
	packageName string // a registry module's or remote module's name
	reference   string // the git reference or the version a tfr:// address asks for
	origin      string // where a remote module is fetched from
	archive     bool   // a remote module that is not a git or hg repository
}

// parseSource reads a module source address: a local path, a registry address
// (namespace/name/provider with an optional host and //subdirectory, or Terragrunt's
// tfr:// form), or anything go-getter fetches - git, hg, GitHub and Bitbucket
// shorthands, scp-like git@host:path, http(s) archives, s3:: and gcs:: buckets.
//
// Implements: REQ-TERRAFORM-004
func parseSource(s string) (moduleSource, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return moduleSource{}, false
	}
	if s == "." || s == ".." || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return moduleSource{local: s}, true
	}
	if rest, ok := strings.CutPrefix(s, "tfr://"); ok {
		address, query, _ := strings.Cut(rest, "?")
		q, _ := url.ParseQuery(query)
		host, path, _ := strings.Cut(address, "/")
		if packageName, ok := registryModule(path); ok {
			if h := registryHost(host); h != "" {
				packageName = h + "/" + packageName
			}
			return moduleSource{packageName: packageName, reference: q.Get("version")}, true
		}
		return moduleSource{}, false
	}
	if !strings.Contains(s, "::") && !strings.Contains(s, "://") && !strings.HasPrefix(s, "/") {
		parts := strings.SplitN(strings.SplitN(s, "//", 2)[0], "/", 4)
		if len(parts) == 4 && hostLike(parts[0]) {
			if packageName, ok := registryModule(s[len(parts[0])+1:]); ok {
				if h := registryHost(parts[0]); h != "" {
					packageName = h + "/" + packageName
				}
				return moduleSource{packageName: packageName}, true
			}
		}
		if len(parts) == 3 {
			if packageName, ok := registryModule(s); ok {
				return moduleSource{packageName: packageName}, true
			}
		}
	}
	return remoteSource(s)
}

// registryModule reads namespace/name/provider[//subdir], lower-casing the name
// (registries match names without regard to case).
func registryModule(s string) (string, bool) {
	address, subdirectory, hasSub := strings.Cut(s, "//")
	parts := strings.Split(address, "/")
	if len(parts) != 3 {
		return "", false
	}
	for _, p := range parts {
		if !registryName.MatchString(p) {
			return "", false
		}
	}
	name := strings.ToLower(address)
	if hasSub && strings.Trim(subdirectory, "/") != "" {
		name += "//" + strings.Trim(subdirectory, "/")
	}
	return name, true
}

// remoteSource names a module fetched from a URL by the repository or object it
// comes from: getter prefix, scheme, credentials, ".git" and the query dropped, the
// host lower-cased, a //subdirectory kept (github.com/org/repo//modules/x).
func remoteSource(s string) (moduleSource, bool) {
	getter := ""
	if i := strings.Index(s, "::"); i > 0 && !strings.ContainsAny(s[:i], "/:") {
		getter, s = strings.ToLower(s[:i]), s[i+2:]
	}
	address, query, _ := strings.Cut(s, "?")
	q, _ := url.ParseQuery(query)
	origin := address
	host, rest := "", ""
	if m := scpLike.FindStringSubmatch(address); m != nil && !strings.Contains(address, "://") {
		host, rest = m[1], m[2]
	} else {
		if _, after, ok := strings.Cut(address, "://"); ok {
			address = after
		}
		if at := strings.IndexByte(address, '@'); at >= 0 && at < strings.IndexByte(address+"/", '/') {
			address = address[at+1:]
		}
		host, rest, _ = strings.Cut(address, "/")
		if i := strings.IndexByte(host, ':'); i >= 0 && getter != "s3" {
			host = host[:i] // a port, or ssh's user@host:port
		}
	}
	if host == "" || rest == "" || !hostLike(host) {
		return moduleSource{}, false
	}
	repository, subdirectory, _ := strings.Cut(rest, "//")
	repository = strings.TrimSuffix(strings.TrimSuffix(repository, "/"), ".git")
	name := strings.ToLower(host) + "/" + repository
	if subdirectory = strings.Trim(subdirectory, "/"); subdirectory != "" {
		name += "//" + subdirectory
	}
	lower := strings.ToLower(repository)
	archive := getter == "s3" || getter == "gcs" || getter == "http" || getter == "https" ||
		q.Get("archive") != "" || strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tar.xz")
	if getter == "" && !archive && (strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")) &&
		!strings.HasSuffix(strings.ToLower(strings.SplitN(rest, "//", 2)[0]), ".git") {
		// A plain web address is go-getter's HTTP getter, which reads the module's
		// real location from the page: its name here is the address itself.
		archive = true
	}
	return moduleSource{packageName: name, reference: q.Get("ref"), origin: getter + prefixIf(getter) + origin, archive: archive}, true
}

func prefixIf(getter string) string {
	if getter == "" {
		return ""
	}
	return "::"
}

// exactVersion reads a constraint that allows one version ("1.2.3", "= 1.2.3",
// "=1.2.3") as that version.
func exactVersion(c string) (string, bool) {
	v := strings.TrimSpace(c)
	v = strings.TrimSpace(strings.TrimPrefix(v, "="))
	if v == "" || strings.ContainsAny(v, ", ") || !lang.Pinned(v) || lang.Commit(v) {
		return "", false
	}
	return strings.TrimPrefix(v, "v"), true
}

// providerSource normalizes a provider source address: lower case, the public
// registries' hosts dropped (registry.terraform.io/hashicorp/aws: hashicorp/aws). A
// bare type (the legacy "aws") is HashiCorp's.
//
// Implements: REQ-TERRAFORM-007
func providerSource(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	parts := strings.Split(s, "/")
	switch len(parts) {
	case 1:
		return "hashicorp/" + s
	case 3:
		if h := registryHost(parts[0]); h == "" {
			return parts[1] + "/" + parts[2]
		}
	}
	return s
}
