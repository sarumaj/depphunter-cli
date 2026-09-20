package lang

import "testing"

func TestPinned(t *testing.T) {
	for _, spec := range []string{
		"1.2.3", "v1.2.3", "1.2", "1.2.3-rc.1", "1.2.3+build.5",
		"v0.0.0-20180428030007-95032a82bc51", // a Go pseudo-version
		"==1.2.3",                            // Python, PowerShell
		"[1.2.3]",                            // a NuGet or Maven single-version range
		"95032a82bc5195032a82bc5195032a82bc519503",                                     // a git commit
		"sha256:" + "ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12", // an OCI digest
	} {
		if !Pinned(spec) {
			t.Errorf("%q should pin", spec)
		}
	}
	for _, spec := range []string{
		"", "   ", "^1.2.3", "~1.2", ">=1.2", "<2", ">=1.2,<2", "1.2 - 1.8", "1.2.x", "1.*", "*",
		"latest", "next", "main", "HEAD", "RELEASE", "LATEST", "[1.0,2.0)", "(,1.0]", "1.0.0 || 2.0.0",
		"1.2.3.RELEASE", "workspace:*", "sha256:beef", "95032a82", // a short sha is not a commit
	} {
		if Pinned(spec) {
			t.Errorf("%q should float", spec)
		}
	}
}

func TestPinnedSemver(t *testing.T) {
	// npm reads a shortened version as a range, so it needs all three parts.
	for _, spec := range []string{"1.2.3", "v1.2.3", "1.2.3-rc.1"} {
		if !PinnedSemver(spec) {
			t.Errorf("%q should pin", spec)
		}
	}
	for _, spec := range []string{"1.2", "1", "^1.2.3", "latest"} {
		if PinnedSemver(spec) {
			t.Errorf("%q should float", spec)
		}
	}
	if !PinnedSemver("95032a82bc5195032a82bc5195032a82bc519503") {
		t.Error("a git commit should pin")
	}
}
