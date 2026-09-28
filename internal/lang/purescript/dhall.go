package purescript

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/dhall"
)

// Dhall is read by the evaluator in internal/lang/dhall, which the dhall
// plugin shares.

// setName names a package set by the import it starts from: the release a
// package-sets URL downloads (psc-0.15.0-20220507), else the URL's repository.
func setName(v *dhall.Value) string {
	for k := 0; v != nil && k < 64; k++ {
		switch v.Kind {
		case dhall.KindImport:
			if !strings.Contains(v.Location, "://") {
				return ""
			}
			u := strings.TrimSuffix(v.Location, "/packages.dhall")
			if u != v.Location {
				return u[strings.LastIndexByte(u, '/')+1:]
			}
			return lang.RepositoryName(v.Location)
		case dhall.KindRecord:
			v = v.Base
		default:
			return ""
		}
	}
	return ""
}
