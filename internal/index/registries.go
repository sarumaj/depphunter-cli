package index

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// The ecosystems whose transitive dependencies are read from a registry rather than
// from the repository: crates.io and its mirrors, NuGet feeds, and OCI registries.
// Each speaks its own protocol; ociBase, below, explains the odd one out.

// ---------------------------------------------------------------- crates.io

// cargoCrate reads a crate's own dependencies from a sparse index (RFC 2789): one
// line of JSON per published version, at a path derived from the crate's name. Every
// modern registry speaks it, but crates.io serves its index from a different host
// than the registry Cargo is configured with, so the one is turned into the other
// here; a mirror already names its index and is used as given.
func (c *Client) cargoCrate(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	return c.cargoSparse(ctx, cargoIndex(index), t)
}

// cargoIndex is the sparse index for a configured Cargo source. Cargo writes the
// protocol into the URL ("sparse+https://…"), and crates.io keeps its index on a host
// of its own.
func cargoIndex(index string) string {
	index = strings.TrimRight(strings.TrimPrefix(index, "sparse+"), "/")
	if host := Host(index); host == "" || host == "crates.io" {
		return "https://index.crates.io"
	}
	return index
}

// cargoSparse reads the last line of a crate's sparse-index file, which is its newest
// published version, or the line for the version asked for.
func (c *Client) cargoSparse(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	body, err := c.accept(ctx, index+"/"+sparsePath(t.Package), "text/plain, */*")
	if err != nil {
		return nil, err
	}
	type line struct {
		Vers string `json:"vers"`
		Deps []struct {
			Name     string `json:"name"`
			Req      string `json:"req"`
			Kind     string `json:"kind"`
			Optional bool   `json:"optional"`
		} `json:"deps"`
		Yanked bool `json:"yanked"`
	}
	var best *line
	for _, raw := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var l line
		if json.Unmarshal([]byte(raw), &l) != nil || l.Yanked {
			continue
		}
		if t.Version != "" && l.Vers == strings.TrimPrefix(t.Version, "v") {
			best = &l
			break
		}
		best = &l // in published order, so the last one standing is the newest
	}
	if best == nil {
		return nil, nil
	}
	var out []dep
	for _, d := range best.Deps {
		if d.Kind != "" && d.Kind != "normal" || d.Optional {
			continue
		}
		out = append(out, dep{Name: d.Name, Version: d.Req})
	}
	return out, nil
}

// sparsePath is where a sparse index keeps a crate: one directory per name length up
// to three, and two letters at a time past that. Names are matched in lower case.
func sparsePath(name string) string {
	n := strings.ToLower(name)
	switch {
	case len(n) <= 2:
		return fmt.Sprintf("%d/%s", len(n), n)
	case len(n) == 3:
		return fmt.Sprintf("3/%s/%s", n[:1], n)
	default:
		return fmt.Sprintf("%s/%s/%s", n[:2], n[2:4], n)
	}
}

// ---------------------------------------------------------------- NuGet

// nugetPackage reads a package's dependencies from its nuspec.
//
// A NuGet feed is a service index naming the resources it offers, so the address of
// the packages themselves has to be asked for before anything can be fetched from it.
// That answer is the same for every package on the feed and is kept for the run.
func (c *Client) nugetPackage(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	base, err := c.nugetBase(ctx, index)
	if err != nil || base == "" {
		return nil, err
	}
	id := strings.ToLower(t.Package)
	version, err := c.nugetVersion(ctx, base, id, t.Version)
	if err != nil || version == "" {
		return nil, err
	}
	body, err := c.accept(ctx, fmt.Sprintf("%s/%s/%s/%s.nuspec", base, id, version, id), "application/xml, */*")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Metadata struct {
			Dependencies struct {
				// A nuspec lists dependencies either flat or grouped by target
				// framework, and plenty list both.
				Direct []nuspecDep `xml:"dependency"`
				Groups []struct {
					Dependency []nuspecDep `xml:"dependency"`
				} `xml:"group"`
			} `xml:"dependencies"`
		} `xml:"metadata"`
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []dep
	add := func(list []nuspecDep) {
		for _, d := range list {
			if d.ID == "" || seen[strings.ToLower(d.ID)] {
				continue
			}
			seen[strings.ToLower(d.ID)] = true
			out = append(out, dep{Name: d.ID, Version: d.Version})
		}
	}
	add(doc.Metadata.Dependencies.Direct)
	for _, g := range doc.Metadata.Dependencies.Groups {
		add(g.Dependency)
	}
	return out, nil
}

type nuspecDep struct {
	ID      string `xml:"id,attr"`
	Version string `xml:"version,attr"`
}

// nugetBase resolves a feed's service index to the flat container that serves nuspecs
// and version listings, and remembers the answer.
func (c *Client) nugetBase(ctx context.Context, index string) (string, error) {
	c.mu.Lock()
	base, ok := c.feeds[index]
	c.mu.Unlock()
	if ok {
		return base, nil
	}
	base, err := c.readNuGetIndex(ctx, index)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	if c.feeds == nil {
		c.feeds = map[string]string{}
	}
	c.feeds[index] = base
	c.mu.Unlock()
	return base, nil
}

func (c *Client) readNuGetIndex(ctx context.Context, index string) (string, error) {
	address := index
	if !strings.HasSuffix(address, ".json") {
		address += "/index.json" // a feed named by its root rather than its document
	}
	body, err := c.get(ctx, address)
	if err != nil {
		return "", err
	}
	var doc struct {
		Resources []struct {
			ID   string `json:"@id"`
			Type string `json:"@type"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	for _, r := range doc.Resources {
		if strings.HasPrefix(r.Type, "PackageBaseAddress/3.0") {
			return strings.TrimRight(r.ID, "/"), nil
		}
	}
	return "", nil
}

// nugetVersion is the version to fetch: the one asked for when it names one, and the
// newest release on the feed when it does not.
func (c *Client) nugetVersion(ctx context.Context, base, id, want string) (string, error) {
	if lang.Pinned(want) {
		return strings.ToLower(strings.Trim(strings.TrimPrefix(strings.TrimSpace(want), "["), "[]")), nil
	}
	body, err := c.accept(ctx, fmt.Sprintf("%s/%s/index.json", base, id), "application/json")
	if err != nil {
		return "", err
	}
	var doc struct {
		Versions []string `json:"versions"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	newest := ""
	for _, v := range doc.Versions {
		if strings.ContainsAny(v, "-") {
			continue // a pre-release is not what a project gets by default
		}
		newest = v
	}
	if newest == "" && len(doc.Versions) > 0 {
		newest = doc.Versions[len(doc.Versions)-1]
	}
	return strings.ToLower(newest), nil
}

// ---------------------------------------------------------------- OCI

// The media types a registry may answer a manifest request with: the OCI ones and
// Docker's older equivalents, single images and the indexes that point at them.
const ociAccept = "application/vnd.oci.image.manifest.v1+json," +
	"application/vnd.oci.image.index.v1+json," +
	"application/vnd.docker.distribution.manifest.v2+json," +
	"application/vnd.docker.distribution.manifest.list.v2+json"

// ociBase resolves the image an image was built on.
//
// A container image has no dependency list. What it has, when whoever built it said
// so, is a base image - an annotation on the manifest or a label in the config blob -
// and that is the image whose vulnerabilities this one inherits, which is what makes
// an image on the map more than a leaf. Two or three requests, no layers: the
// manifest, one more for a multi-platform index, and the config blob.
func (c *Client) ociBase(ctx context.Context, index string, t lang.Target) ([]dep, error) {
	repo := ociRepository(t.Package)
	ref := t.Version
	if ref == "" {
		ref = "latest"
	}
	manifest, err := c.ociFetch(ctx, index, repo, ref, ociAccept)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Annotations map[string]string `json:"annotations"`
		Manifests   []struct {
			Digest   string `json:"digest"`
			Platform struct {
				OS   string `json:"os"`
				Arch string `json:"architecture"`
			} `json:"platform"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(manifest, &doc); err != nil {
		return nil, err
	}
	if base := ociBaseOf(doc.Annotations); base.Name != "" {
		return []dep{base}, nil
	}
	// A multi-platform index points at one manifest per platform, and the base image
	// is the same in all of them: whichever comes first will do, with linux/amd64
	// preferred only because it is the one most likely to exist.
	if doc.Config.Digest == "" && len(doc.Manifests) > 0 {
		pick := doc.Manifests[0].Digest
		for _, m := range doc.Manifests {
			if m.Platform.OS == "linux" && m.Platform.Arch == "amd64" {
				pick = m.Digest
				break
			}
		}
		if manifest, err = c.ociFetch(ctx, index, repo, pick, ociAccept); err != nil {
			return nil, err
		}
		doc.Config.Digest, doc.Annotations, doc.Manifests = "", nil, nil
		if err := json.Unmarshal(manifest, &doc); err != nil {
			return nil, err
		}
		if base := ociBaseOf(doc.Annotations); base.Name != "" {
			return []dep{base}, nil
		}
	}
	if doc.Config.Digest == "" {
		return nil, nil
	}
	blob, err := c.ociBlob(ctx, index, repo, doc.Config.Digest)
	if err != nil {
		return nil, err
	}
	var config struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	if err := json.Unmarshal(blob, &config); err != nil {
		return nil, err
	}
	if base := ociBaseOf(config.Config.Labels); base.Name != "" {
		return []dep{base}, nil
	}
	return nil, nil
}

// ociBaseOf reads the standard base-image keys out of annotations or labels. The
// digest is preferred over the tag where both are given: a tag moves, a digest is the
// image that was actually built on.
func ociBaseOf(kv map[string]string) dep {
	name := strings.TrimSpace(kv["org.opencontainers.image.base.name"])
	if name == "" {
		return dep{}
	}
	image, tag := name, ""
	if i := strings.LastIndex(name, "@"); i > 0 {
		image, tag = name[:i], name[i+1:]
	} else if i := strings.LastIndex(name, ":"); i > 0 && !strings.Contains(name[i:], "/") {
		image, tag = name[:i], name[i+1:]
	}
	if digest := strings.TrimSpace(kv["org.opencontainers.image.base.digest"]); digest != "" {
		tag = digest
	}
	if tag == "" {
		tag = "latest"
	}
	return dep{Name: image, Version: tag}
}

// ociRepository is the path part of an image reference, as a registry's API wants it:
// a Docker Hub image with no namespace lives under "library".
func ociRepository(image string) string {
	first, rest, ok := strings.Cut(image, "/")
	if !ok {
		return "library/" + image
	}
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return rest // the first segment was the registry
	}
	return image
}

func (c *Client) ociFetch(ctx context.Context, index, repo, ref, accept string) ([]byte, error) {
	return c.ociGet(ctx, index, repo, fmt.Sprintf("%s/v2/%s/manifests/%s", index, repo, ref), accept)
}

func (c *Client) ociBlob(ctx context.Context, index, repo, digest string) ([]byte, error) {
	return c.ociGet(ctx, index, repo, fmt.Sprintf("%s/v2/%s/blobs/%s", index, repo, digest), "application/json, */*")
}

// ociGet performs one registry request, answering the pull-token challenge a registry
// sends when it will not serve anonymously without one. Only the token the registry
// itself points at is asked for, and it is used for this request alone.
func (c *Client) ociGet(ctx context.Context, index, repo, address, accept string) ([]byte, error) {
	resp, err := c.do(ctx, address, accept, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge := resp.Header.Get("Www-Authenticate")
		resp.Body.Close()
		token, err := c.ociToken(ctx, index, repo, challenge)
		if err != nil {
			return nil, err
		}
		if token == "" {
			// No challenge this client can answer: the registry refused.
			return nil, fmt.Errorf("%s: %s", address, http.StatusText(http.StatusUnauthorized))
		}
		if resp, err = c.do(ctx, address, accept, token); err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", address, resp.Status)
	}
	return readLimited(resp)
}

// ociToken asks for the pull token a registry's challenge describes.
//
// A challenge is a header, and a header names wherever it likes, so the realm is
// checked before it is followed: either https, or the registry this request was
// already going to - which keeps a registry on a private plain-http network working
// without letting a plain-http challenge redirect the request elsewhere. Credentials
// are chosen by the realm's own host (credentials.apply), so a redirected realm gets
// none of the registry's.
func (c *Client) ociToken(ctx context.Context, index, repo, challenge string) (string, error) {
	scheme, params, ok := strings.Cut(challenge, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", nil
	}
	fields := map[string]string{}
	for _, part := range splitChallenge(params) {
		if k, v, ok := strings.Cut(part, "="); ok {
			fields[strings.ToLower(strings.TrimSpace(k))] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	realm := fields["realm"]
	if realm == "" {
		return "", nil
	}
	u, err := url.Parse(realm)
	if err != nil {
		return "", nil
	}
	if registry, err := url.Parse(index); u.Scheme != "https" && (err != nil || u.Host != registry.Host) {
		return "", nil
	}
	q := u.Query()
	if s := fields["service"]; s != "" {
		q.Set("service", s)
	}
	scope := fields["scope"]
	if scope == "" {
		scope = "repository:" + repo + ":pull"
	}
	q.Set("scope", scope)
	u.RawQuery = q.Encode()
	body, err := c.get(ctx, u.String())
	if err != nil {
		return "", err
	}
	var doc struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if doc.Token != "" {
		return doc.Token, nil
	}
	return doc.AccessToken, nil
}

// splitChallenge splits a challenge's comma-separated parameters, leaving the commas
// that are inside a quoted scope where they are.
func splitChallenge(params string) []string {
	var out []string
	quoted, start := false, 0
	for i, r := range params {
		switch {
		case r == '"':
			quoted = !quoted
		case r == ',' && !quoted:
			out = append(out, params[start:i])
			start = i + 1
		}
	}
	return append(out, params[start:])
}
