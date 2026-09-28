package lib.kubernetes

import rego.v1

containers contains c if {
	some c in input.spec.containers
}

is_deployment if input.kind == "Deployment"

name(obj) := obj.metadata.name
