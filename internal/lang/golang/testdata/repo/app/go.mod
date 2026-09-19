module example.com/app

go 1.22

require (
	github.com/spf13/cobra v1.8.0
	example.com/lib v0.0.0
)

replace example.com/lib => ../lib
