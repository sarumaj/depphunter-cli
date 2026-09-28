// The shop's configuration. import "fake-comment" is a comment.
@extern(embed)

package config

import (
	"strings"
	j "encoding/json"
	"list"
	"tool/exec"
	"example.com/shop/schema"
	"example.com/shop/schema:other"
	"example.com/shop/schema/sub"
	"example.com/shop/nothere"
	"k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	"net/http"
	"example.com/shop/api"
	"github.com/acme/schemas/k8s"
	"cue.dev/x/k8s.io/api/apps/v1:appsv1reg"
	"github.com/acme/chain/x"
	"github.com/acme/unversioned/y"
	"github.com/legacy/lib"
	"example.org/old/defs"
	"github.com/nobody/thing/pkg"
	"unknownstd"
)

import s "strings"

#Config: {
	name: string
}

_#Hidden: int
_hidden:  1

name: strings.ToUpper("shop") & s.ToLower("SHOP")
"quoted-field": j.Marshal({a: 1})
opt?:  int
req!:  string
a: b: c: 1
X=aliased: 3
[string]: _
(name): "dynamic"
let local = 1
name: string
import: "fake-field"
fake:   "import \"fake-string\""
multi: """
	import "fake-multi"
	"""
raw: #"import "fake-raw" \(ignored)"#
interp: "\(strings.Join(["import", "fake-interp"], " "))"
items: [...schema.#Product] & list.MinItems(0)
obj: {
	inner: 1
}
for k, v in {x: 1} {
	"gen-\(k)": v
}
if true {
	conditional: 1
}
cmd: exec.Run & {cmd: "true"}
pod:    v1.#Pod
deploy: appsv1.#Deployment
header: http.#Header
item:   api.#Item
other:  other.extra
sub:    sub.#Sub
