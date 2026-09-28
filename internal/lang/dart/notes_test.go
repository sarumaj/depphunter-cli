package dart

import (
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// pubspec.lock pins without edges, which is noted, as is one read from disk that
// the scan left out.
//
// Verifies: REQ-DART-007, REQ-TRC-017
func TestPubspecLockNotes(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"pubspec.yaml": "name: app\ndependencies:\n  http: ^1.2.0\n",
		"pubspec.lock": "packages:\n  http:\n    dependency: \"direct main\"\n    description:\n      name: http\n" +
			"      url: \"https://pub.dev\"\n    source: hosted\n    version: \"1.2.0\"\n",
		"lib/main.dart": "import 'package:http/http.dart';\n",
	})
	files := langtest.Files(t, root)
	if got, want := langtest.Notes(newResolver(root, files)), []string{"pubspec.lock lock-flat"}; !slices.Equal(got, want) {
		t.Errorf("notes %q, want %q", got, want)
	}
	got := langtest.Notes(newResolver(root, langtest.Without(files, "pubspec.lock")))
	if want := []string{"pubspec.lock lock-flat", "pubspec.lock lock-ignored"}; !slices.Equal(got, want) {
		t.Errorf("ignored: notes %q, want %q", got, want)
	}
}
