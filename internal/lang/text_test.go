package lang

import "testing"

func TestLineOfFindsTheFirstNeedleAtOrAfterAnOffset(t *testing.T) {
	duplicates := "\"alpha\" = 1\n\"alpha\" = 2\n"
	for _, testCase := range []struct {
		name, source, needle string
		from, want           int
	}{
		{"quoted key", "{\n  \"left-pad\": \"1.3.0\"\n}", `"left-pad"`, 0, 2},
		{"inline table", "[dependencies]\nbeta = { \"gamma\" = \"1\" }\n", `"gamma"`, 0, 2},
		{"first of duplicate keys", duplicates, `"alpha"`, 0, 1},
		{"duplicate key after the first", duplicates, `"alpha"`, 1, 2},
		{"comment counts as text", "# \"alpha\" is pinned\n\"alpha\" = 1\n", `"alpha"`, 0, 1},
		{"CRLF", "[a]\r\nb = 1\r\n\"c\" = 2\r\n", `"c"`, 0, 3},
		{"key in a string value", "note = \"see alpha\"\nalpha = 1\n", "alpha", 0, 1},
		{"missing key", "alpha = 1\n", `"beta"`, 0, 0},
		{"negative offset", "\nalpha = 1\n", "alpha", -5, 2},
		{"offset past the end", "alpha = 1\n", "alpha", 100, 0},
	} {
		if got := LineOf([]byte(testCase.source), testCase.needle, testCase.from); got != testCase.want {
			t.Errorf("%s: LineOf(%q, %q, %d) = %d, want %d", testCase.name, testCase.source, testCase.needle, testCase.from, got, testCase.want)
		}
	}
}
