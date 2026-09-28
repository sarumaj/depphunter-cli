module: "example.com/shop@v0"
language: {
	version: "v0.9.0"
}
deps: {
	"github.com/acme/schemas@v0": {
		v: "v0.3.1"
	}
	"cue.dev/x/k8s.io@v0": {
		v:       "v0.5.0"
		default: true
	}
	"github.com/acme/unversioned@v0": {}
}
deps: "github.com/acme/chain@v1": v: "v1.2.0"
