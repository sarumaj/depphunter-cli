package main_test

import data.main

test_deny if {
	count(data.main.deny) == 0 with input as {}
	data.lib.util.allow
}
