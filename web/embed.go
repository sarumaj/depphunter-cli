// Package web embeds the browser UI. It is plain ES modules with vendored libraries,
// so building the binary needs no JavaScript toolchain.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var static embed.FS

func Assets() fs.FS {
	sub, err := fs.Sub(static, "static")
	if err != nil {
		panic(err) // unreachable: the directory is embedded at compile time
	}
	return sub
}
