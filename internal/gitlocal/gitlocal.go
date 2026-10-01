// Package gitlocal runs git the way depphunter needs it: reading the repository on
// disk, and never anything else.
//
// A clone made with --filter (a partial clone) fetches what it is missing from its
// remote on demand, and so does any git command that needs it - a log that counts
// changed lines reads every blob it touches. That fetch runs the user's credential
// helper, which asks for a password: under --watch, once for every analysis. So every
// way out is closed: no transport may be used, no credential helper is consulted, no
// lazy fetch is made, and nothing prompts. What is not on disk is an error, which the
// caller works around.
package gitlocal

import (
	"context"
	"os"
	"os/exec"
)

// Command is git with arguments, run in directory and kept to what is on disk.
//
// Implements: REQ-HIST-016
func Command(ctx context.Context, directory string, arguments ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, "git", append([]string{
		"-C", directory,
		// Every transport refused, so a fetch fails before it connects; and the list
		// of credential helpers emptied, so none would be asked even if one did not.
		"-c", "protocol.allow=never",
		"-c", "credential.helper=",
	}, arguments...)...)
	command.Env = append(os.Environ(),
		"GIT_NO_LAZY_FETCH=1",   // git 2.44 and later: a missing object is not fetched
		"GIT_TERMINAL_PROMPT=0", // no user name or password asked on the terminal
		"GCM_INTERACTIVE=never", // nor in a window, by Git Credential Manager
		"GIT_ASKPASS=",          // nor by an askpass program
		"SSH_ASKPASS_REQUIRE=never",
	)
	return command
}
