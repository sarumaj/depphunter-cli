package main

import (
	"fmt"
	"net/http"

	"example.com/app/internal/util"
	"example.com/lib/sub"
	"github.com/spf13/cobra/doc"
	"github.com/undeclared/thing"
)

type Server struct{}

func (s *Server) Start() {}

func init() {}
func init() {}

const Version = "1"

func main() { fmt.Println(http.StatusOK, util.X, sub.Y, doc.Z, thing.W, cobra) }
