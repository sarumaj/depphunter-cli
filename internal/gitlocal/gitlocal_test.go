package gitlocal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git runs a git command for the test's own setup, failing the test on error.
func git(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir())
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, out)
	}
	return string(out)
}

// A partial clone missing a blob does not go back to its remote for it, and does not
// ask anybody for a credential to: the command fails, at once.
//
// Verifies: REQ-HIST-016
func TestNothingIsFetched(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	origin, clone := t.TempDir(), filepath.Join(t.TempDir(), "clone")
	git(t, origin, "init", "-q")
	git(t, origin, "config", "uploadpack.allowFilter", "true")
	os.WriteFile(filepath.Join(origin, "a.txt"), []byte("one\n"), 0o644)
	git(t, origin, "add", ".")
	git(t, origin, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "-qm", "one")
	os.WriteFile(filepath.Join(origin, "a.txt"), []byte("one\ntwo\n"), 0o644)
	git(t, origin, "-c", "user.name=a", "-c", "user.email=a@b", "commit", "-qam", "two")
	git(t, filepath.Dir(clone), "clone", "-q", "--filter=blob:none", "--no-checkout", "file://"+origin, clone)

	// A credential helper that leaves a mark when it is run.
	marker := filepath.Join(t.TempDir(), "asked")
	helper := filepath.Join(t.TempDir(), "helper.sh")
	os.WriteFile(helper, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755)
	git(t, clone, "config", "credential.helper", helper)

	out, err := Command(context.Background(), clone, "log", "--numstat", "--format=%H").CombinedOutput()
	if err == nil {
		t.Fatalf("the missing blobs were fetched:\n%s", out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the credential helper was run")
	}
	// What is on disk is still read.
	if out, err := Command(context.Background(), clone, "log", "--name-only", "--no-renames", "--format=%H").Output(); err != nil || !strings.Contains(string(out), "a.txt") {
		t.Errorf("reading what is on disk: %v\n%s", err, out)
	}
}
