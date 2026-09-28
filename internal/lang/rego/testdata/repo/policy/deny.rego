# METADATA
# description: import data.fake.comment
package main

import rego.v1
import future.keywords.in
import input
import data.lib.kubernetes
import data.lib.util as u
import data.lib
import data.external.inventory
import data.rules.bugs["constant-condition"] as cc

deny contains msg if {
	some c in kubernetes.containers
	not u.allow
	lib.util.helper(c)
	data.lib.kubernetes.is_deployment
	x := data.inventory.hosts
	cc.report
	msg := sprintf("%s: data.fake.string", [c.name])
}

deny contains msg if {
	msg := "second"
}

warn[msg] {
	msg := `raw
data.fake.raw`
}

f(x) := y if {
	y := x
}

default allow := false
