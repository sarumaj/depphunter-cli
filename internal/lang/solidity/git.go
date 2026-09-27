package solidity

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// gitTimeout bounds the one git call a resolver makes: a repository on a
// slow network share must not hold up the analysis for long.
const gitTimeout = 10 * time.Second

// gitlinks asks git which commit the index records for each of the submodule
// paths under dir (paths relative to dir): what `git submodule update`
// checks out. Without git, outside a repository, or for a path git does not
// record as a submodule the answer is simply missing.
//
// Implements: REQ-SOLIDITY-006
func gitlinks(dir string, paths []string) map[string]string {
	out := map[string]string{}
	if len(paths) == 0 {
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	args := append([]string{"-C", dir, "ls-files", "--stage", "-z", "--"}, paths...)
	data, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return out
	}
	for _, rec := range bytes.Split(data, []byte{0}) {
		// "160000 <sha> <stage>\t<path>": mode 160000 is a gitlink.
		meta, p, ok := strings.Cut(string(rec), "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) == 3 && f[0] == "160000" {
			out[p] = f[1]
		}
	}
	return out
}
