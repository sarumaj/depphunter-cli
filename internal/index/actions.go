package index

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/ci"
	"github.com/sarumaj/depphunter-cli/internal/lang/docker"
	"github.com/sarumaj/depphunter-cli/internal/trace"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// ---------------------------------------------------------------- sources

// machineActions records the GitHub instances this machine works with besides
// github.com, each asked, before github.com, for the actions and reusable
// workflows a workflow names (a reference names no host: an Enterprise Server
// runs one it holds and, with GitHub Connect, falls back to github.com's): the
// instance `GITHUB_API_URL` names (on a runner of that instance), the one
// `GH_HOST` names, and every other host the GitHub CLI's hosts.yml names. A
// repository cannot name one.
//
// Implements: REQ-SUP-077
func machineActions(m userconf.Machine, k sink) {
	if u, err := url.Parse(strings.TrimSpace(m.Environment("GITHUB_API_URL"))); err == nil && u.Host != "" &&
		!strings.EqualFold(u.Hostname(), "api.github.com") {
		k.extra(Actions, u.String())
	}
	hosts := []string{m.Environment("GH_HOST")}
	for _, h := range m.GitHubCLIHosts() {
		hosts = append(hosts, h.Host)
	}
	for _, host := range hosts {
		if api := userconf.GitHubAPI(host); api != "" && api != public[Actions] {
			k.extra(Actions, api)
		}
	}
}

// ---------------------------------------------------------------- client

// actionDependencies reads what an action or a reusable workflow runs, from its
// repository at the reference the workflow names (lang.Target.Registry is the
// path inside the repository): a reusable workflow's file, else the action's
// action.yml, else its action.yaml. A composite action's steps, a Docker action's
// image (named in action.yml, or in the FROM of the Dockerfile it is built from)
// and a reusable workflow's jobs are the answer, read by the ci plugin; a
// JavaScript action has none.
//
// Implements: REQ-SUP-077, REQ-CI-016
func (c *Client) actionDependencies(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	directory := path.Clean("/" + t.Registry)[1:]
	var files []string
	if extension := path.Ext(directory); extension == ".yml" || extension == ".yaml" {
		files = []string{directory}
	} else {
		files = []string{path.Join(directory, "action.yml"), path.Join(directory, "action.yaml")}
	}
	for _, file := range files {
		body, err := c.githubFile(ctx, index, t.Package, t.Version, file)
		if notFound(err) {
			continue
		}
		if err != nil {
			var status *statusError
			if errors.As(err, &status) && (status.code == http.StatusForbidden || status.code == http.StatusTooManyRequests) {
				// Implements: REQ-SUP-077, REQ-TRC-017
				c.note(trace.NoteForbidden, "GitHub's API at "+index+" refused to answer ("+status.status+"): it "+
					"answers 60 requests an hour without a token; GH_TOKEN, GITHUB_TOKEN or a token `gh auth login` "+
					"keeps in hosts.yml (not the keyring) lets depphunter ask it")
			}
			return nil, err
		}
		if len(files) == 1 {
			return actionAnswer(ci.WorkflowDependencies(body, t.Package, t.Version)), nil
		}
		targets, dockerfile := ci.ActionDependencies(body)
		if dockerfile != "" {
			// Relative to action.yml, and inside the repository.
			if p := path.Clean("/" + path.Join(directory, dockerfile))[1:]; p != "" {
				source, err := c.githubFile(ctx, index, t.Package, t.Version, p)
				if err != nil && !notFound(err) {
					return nil, err
				}
				targets = append(targets, docker.DockerfileImages(source)...)
			}
		}
		return actionAnswer(targets), nil
	}
	return nil, errAbsent
}

// actionAnswer turns what the ci plugin read into an answer, each package once.
func actionAnswer(targets []lang.Target) []dependency {
	seen := map[string]bool{}
	out := make([]dependency, 0, len(targets))
	for _, t := range targets {
		d := dependency{Name: t.Package, Version: t.Version, Registry: t.Registry}
		if t.Ecosystem != Actions {
			d.Ecosystem = t.Ecosystem
		}
		if key := t.Ecosystem + " " + t.Package; !seen[key] {
			seen[key] = true
			out = append(out, d)
		}
	}
	return out
}

// githubFile reads one file of a repository at a reference through the REST
// contents API of index (`GET <api>/repos/<owner>/<repo>/contents/<path>?ref=`,
// the raw media type), with the token this machine holds for that API. A
// github.com repository is read from raw.githubusercontent.com instead when this
// machine holds no credential for api.github.com: the files of a public
// repository are served there without the API's limit of 60 requests an hour.
func (c *Client) githubFile(ctx context.Context, index, repository, reference, file string) ([]byte, error) {
	contents := "/repos/" + escapePath(repository) + "/contents/" + escapePath(file) + "?ref=" + url.QueryEscape(reference)
	if c.config.Public(Actions, index) {
		if !c.auth.Authorizes(index + contents) {
			return c.accept(ctx, githubRaw+"/"+escapePath(repository)+"/"+escapePath(reference)+"/"+escapePath(file), "*/*")
		}
		index = githubAPI
	}
	return c.accept(ctx, index+contents, "application/vnd.github.raw+json")
}

// escapePath escapes each segment of a slash-separated path.
func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}
