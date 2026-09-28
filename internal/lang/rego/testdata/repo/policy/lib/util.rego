package lib.util

default allow := false

allow if input.user == "admin"

helper(x) if x.name != ""

ref.head.rule := 1
