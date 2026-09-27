package scan

import "testing"

// Verifies: REQ-LANG-015, REQ-DOCKER-001
func TestLanguageByName(t *testing.T) {
	for p, want := range map[string]string{
		"a.go":                           "Go",
		"x.py":                           "Python",
		"Dockerfile":                     "Docker",
		"build/Containerfile":            "Docker",
		"Dockerfile.dev":                 "Docker",
		"api.Dockerfile":                 "Docker",
		"docker/worker.dockerfile":       "Docker",
		"Dockerfile.dockerignore":        "",
		"docs/dockerfile-guide.md":       "Markdown",
		"notes.unknownext":               "",
		"compose.yaml":                   "YAML",
		"deploy/docker-compose.prod.yml": "YAML",
	} {
		if got := Language(p); got != want {
			t.Errorf("%s: got %q, want %q", p, got, want)
		}
	}
}
