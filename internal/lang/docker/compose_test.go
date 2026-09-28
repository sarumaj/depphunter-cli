package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	analysis "github.com/sarumaj/depphunter-cli/internal/analyze"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/trace"
)

const secret = "s3cr3t-hunter2"

// composeProject is a Compose project spread over several files: an .env file
// beside the main one (with a password nothing may report), included files - one
// not named as a Compose file, with its own .env, one named through a variable - and
// services extending services of the same file and of another one.
var composeProject = map[string]string{
	"compose.yaml": `include:
  - infra/db.yml
  - path: [svc/compose.yaml]
  - ${INFRA_DIR}/cache.yml
  - oci://registry.example/stack:1
  - ../outside.yml
  - nowhere.yml
services:
  app:
    image: ${APP_IMAGE}:${APP_TAG:-latest}
  pinned:
    image: "redis:${REDIS_TAG:-7}"
  tool:
    image: ${UNSET_REGISTRY}/tool:1
    environment:
      DB_PASSWORD: ${DB_PASSWORD}
  base:
    image: busybox:1.36
  child:
    extends: base
  worker:
    extends:
      file: common/services.yml
      service: builder
  web:
    image: acme/web:1
    extends:
      file: common/services.yml
      service: tagged
  gone:
    extends:
      file: common/missing.yml
      service: x
  own:
    image: alpine:3.20
    extends: base
  partly:
    image: ${APP_IMAGE}-${UNSET_SUFFIX}:1
  deep:
    extends:
      file: common/services.yml
      service: chained
`,
	".env": "# the project's values\nAPP_IMAGE=ghcr.io/acme/app\nexport APP_TAG=\"2.1\"\n" +
		"REDIS_TAG='7.2'\nINFRA_DIR=infra\nDB_PASSWORD=" + secret + " # never shown\nHOSTNAME\n",
	"infra/db.yml":     "services:\n  db:\n    image: postgres:${PG:-16}\n",
	"infra/.env":       "PG=16.3\n",
	"infra/cache.yml":  "services:\n  cache:\n    image: valkey/valkey:8\n",
	"svc/compose.yaml": "services:\n  s:\n    image: nginx:1.27\n",
	"common/services.yml": "services:\n  builder:\n    build: ./ctx\n  tagged:\n    extends: builder\n" +
		"  chained:\n    extends: {file: ../base/more.yml, service: m}\n",
	"base/more.yml":         "services:\n  m:\n    image: debian:12\n",
	"common/ctx/Dockerfile": "FROM golang:1.22\n",
}

// Verifies: REQ-DOCKER-003, REQ-DOCKER-009
func TestComposeFilesTogether(t *testing.T) {
	result := langtest.Analyze(t, Plugin{}, langtest.Write(t, composeProject))["compose.yaml"]
	langtest.CheckImports(t, result, map[string]lang.Target{
		// Included files are edges; one no Compose name claims is read here, with
		// its own directory's .env file.
		"include: infra/db.yml":                    {Local: "infra/db.yml"},
		"image: postgres:${PG:-16} (infra/db.yml)": image("postgres", "16.3"),
		"include: svc/compose.yaml":                {Local: "svc/compose.yaml"},
		"include: ${INFRA_DIR}/cache.yml":          {Local: "infra/cache.yml"},
		"image: valkey/valkey:8 (infra/cache.yml)": image("valkey/valkey", "8"),
		"include: ../outside.yml":                  {},
		"include: nowhere.yml":                     {},
		// The .env file beside the Compose file sets what the references use.
		"image: ${APP_IMAGE}:${APP_TAG:-latest}": image("ghcr.io/acme/app", "2.1"),
		"image: redis:${REDIS_TAG:-7}":           image("redis", "7.2"),
		"image: ${UNSET_REGISTRY}/tool:1":        {Ecosystem: "oci", Package: "${UNSET_REGISTRY}/tool:1", Unresolved: true},
		// base's image, and child's through extends:.
		"image: busybox:1.36": image("busybox", "1.36"),
		// A service of another file: worker's build is read from that file's
		// directory, and web, which extends a service built there, is built too.
		"extends: common/services.yml": {Local: "common/services.yml"},
		"build: ctx/Dockerfile":        {Local: "common/ctx/Dockerfile"},
		"extends: common/missing.yml":  {},
		// A service's own image: stays; a variable left in a name keeps the name as
		// written, never with an .env value in it.
		"image: alpine:3.20": image("alpine", "3.20"),
		"image: ${APP_IMAGE}-${UNSET_SUFFIX}:1": {
			Ecosystem: "oci", Package: "${APP_IMAGE}-${UNSET_SUFFIX}:1", Unresolved: true,
		},
		// A chain through two files, the second named from the first one's directory.
		"extends: ../base/more.yml": {Local: "base/more.yml"},
		"image: debian:12":          image("debian", "12"),
	})
	lines := map[int]bool{}
	for _, imported := range result.Imports {
		if imported.Spec == "image: busybox:1.36" {
			lines[imported.Line] = true
		}
		if imported.Spec == "image: acme/web:1" || strings.Contains(imported.Spec, "oci://") {
			t.Errorf("reported: %s", imported.Spec)
		}
	}
	if !reflect.DeepEqual(lines, map[int]bool{18: true, 20: true}) {
		t.Errorf("busybox is base's on its image: line and child's on its extends: line: %v", lines)
	}
	langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, langtest.Write(t, composeProject))["svc/compose.yaml"],
		map[string]lang.Target{"image: nginx:1.27": image("nginx", "1.27")})
}

// Nothing an .env file sets is shown: not in the imports, the graph or the
// resolution report, even where it resolves a reference.
//
// Verifies: REQ-DOCKER-003
func TestEnvironmentValuesNeverShown(t *testing.T) {
	files := map[string]string{}
	for k, v := range composeProject {
		files[k] = v
	}
	files["compose.yaml"] += "  leaky:\n    image: ${DB_PASSWORD}/x:${DB_PASSWORD}\n  tagged:\n    image: app:${DB_PASSWORD}\n"
	root := langtest.Write(t, files)
	var out bytes.Buffer
	results := langtest.Analyze(t, Plugin{}, root)
	if err := json.NewEncoder(&out).Encode(results); err != nil {
		t.Fatal(err)
	}
	report := trace.New(-1, false, nil, nil)
	g, _, err := analysis.Run(context.Background(), root, analysis.Options{
		Plugins: []lang.Plugin{Plugin{}}, ResolveDepth: -1, Trace: report,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []any{g, report} {
		if err := json.NewEncoder(&out).Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := report.Text(&out); err != nil {
		t.Fatal(err)
	}
	if err := report.Markdown(&out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), secret) {
		t.Errorf("an .env value was shown:\n%s", out.String())
	}
	// A reference naming a credential is left as written: such values are not read.
	got := langtest.Imports(t, results["compose.yaml"])
	for spec, want := range map[string]lang.Target{
		"image: ${DB_PASSWORD}/x:${DB_PASSWORD}": {Ecosystem: "oci", Package: "${DB_PASSWORD}/x:${DB_PASSWORD}", Unresolved: true},
		"image: app:${DB_PASSWORD}":              image("app", "${DB_PASSWORD}"),
	} {
		if got[spec] != want {
			t.Errorf("%s: got %+v, want %+v", spec, got[spec], want)
		}
	}
	for name, want := range map[string]bool{
		"DB_PASSWORD": true, "GITHUB_TOKEN": true, "AWS_SECRET_ACCESS_KEY": true, "API_KEY": true, "apikey": true,
		"KEY": true, "APP_IMAGE": false, "KEYCLOAK_TAG": false, "TAG": false,
	} {
		if secretName(name) != want {
			t.Errorf("secretName(%s) = %v", name, !want)
		}
	}
}

// Garbage or missing files around a Compose file give no edges past the ones they
// are named by, and no panic.
//
// Verifies: REQ-DOCKER-009
func TestComposeGarbage(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"compose.yaml": "include:\n  - bad.yml\n  - {path: 7}\n  - path: {x: 1}\nservices:\n" +
			"  a:\n    extends: {file: bad.yml, service: x}\n  b:\n    extends: {file: /etc/compose.yml, service: x}\n" +
			"  c:\n    extends: c\n  d:\n    extends: {service: e}\n  e:\n    extends: {service: d}\n" +
			"  f:\n    image: nginx:${TAG}\n",
		".env":    "\x00\xff=\n==\n=x\nTAG\n'broken=\"x\nA=\"unterminated\nB='\n",
		"bad.yml": ":\n\t- [\x00",
	})
	result := langtest.Analyze(t, Plugin{}, root)["compose.yaml"]
	langtest.CheckImports(t, result, map[string]lang.Target{
		"include: bad.yml":    {Local: "bad.yml"},
		"extends: bad.yml":    {Local: "bad.yml"},
		"include: 7":          {}, // a file named 7, which the repository lacks
		"image: nginx:${TAG}": image("nginx", "${TAG}"),
	})
	for _, source := range []string{"\x00\xff", "include: 7\nservices: [1]\n", "include:\n  - [a]\n"} {
		extractCompose([]byte(source))
	}
	if extraction := extractCompose([]byte("include: base.yml\n")); len(extraction.Imports) != 1 || extraction.Imports[0].Module != "base.yml" {
		t.Errorf("an include: of one path: %+v", extraction.Imports)
	}
	if variables := readEnvironment([]byte("A=1\nB=${A}-x\nC='${A}'\nD=\"${A} #\" # c\nE=$$\nexport F = 2\nG=x # c\n")); variables["B"] != "1-x" || variables["G"] != "x" ||
		variables["C"] != "${A}" || variables["D"] != "1 #" || variables["E"] != "$" || variables["F"] != "2" {
		t.Errorf("readEnv: %v", variables)
	}
}
