package lang

import (
	"slices"
	"testing"
)

// Verifies: REQ-LANG-033
func TestTOMLKeyLines(t *testing.T) {
	for _, testCase := range []struct {
		name, source, table, key string
		want                     int
	}{
		{"top-level key", "a = 1\nb = 2\n", "", "b", 2},
		{"key in a table", "a = 1\n[t]\na = 2\n", "t", "a", 3},
		{"same key in another table", "[s]\na = 1\n[t]\na = 2\n", "t", "a", 4},
		{"table header defines its key", "[a.b]\nc = 1\n", "a", "b", 1},
		{"sub-table header defines its parents", "x = 1\n[a.b.c]\n", "a", "b", 2},
		{"spaces in a header", "\n[ a . b ]\nc = 1\n", "a.b", "c", 3},
		{"quoted header", "[ \"a.b\" ]\nc = 1\n", "a.b", "c", 2},
		{"quoted header is one key", "[ \"a.b\" ]\nc = 1\n", "", "a.b", 1},
		{"comment after a header", "[t] # note\na = 1\n", "t", "a", 2},
		{"comment after an array header", "[[t]]   # first\na = 1\n", "t", "a", 2},
		{"array of tables", "[[t]]\na = 1\n[[t]]\nb = 2\n", "t", "b", 4},
		{"basic-quoted key", "[t]\n\"a b\" = 1\n", "t", "a b", 2},
		{"escaped basic-quoted key", "[t]\n\"a\\u0062\" = 1\n", "t", "ab", 2},
		{"literal-quoted key", "[t]\n'a\\b' = 1\n", "t", `a\b`, 2},
		{"tab before =", "[t]\na\t= 1\n", "t", "a", 2},
		{"tabs and no spaces", "[t]\n\ta\t=\t1\n", "t", "a", 2},
		{"no space before =", "[t]\na=1\n", "t", "a", 2},
		{"top-level dotted key", "x = 1\nt.a = 1\n", "t", "a", 2},
		{"top-level dotted key names its first part", "x = 1\nt.a = 1\n", "", "t", 2},
		{"deeper dotted key", "t.a.git = \"u\"\nt.b.git = \"v\"\n", "t", "b", 2},
		{"dotted key is not a prefix", "t.ab.git = \"u\"\nt.a.git = \"v\"\n", "t", "a", 2},
		{"dotted key in a table", "[t]\na.b = 1\n", "t.a", "b", 2},
		{"spaces around dots", "t . \"a\" . b = 1\n", "t.a", "b", 1},
		{"first definition wins", "[t]\na.x = 1\na.y = 2\n", "t", "a", 2},
		{"header after a dotted key", "t.a = 1\n[t.b]\n", "t", "b", 2},
		{"comment line", "# a = 1\na = 2\n", "", "a", 2},
		{"comment after a value", "b = 1 # a = 2\na = 3\n", "", "a", 2},
		{"hash in a string", "b = \"#\"\na = 1\n", "", "a", 2},
		{"multi-line basic string", "b = \"\"\"\na = 1\n[t]\n\"\"\"\na = 2\n", "", "a", 5},
		{"multi-line basic string keeps the table", "b = \"\"\"\n[t]\n\"\"\"\na = 2\n", "t", "a", 0},
		{"escaped quote in a multi-line string", "b = \"\"\"\\\"\"\"\na = 1\n\"\"\"\na = 2\n", "", "a", 4},
		{"line-ending backslash", "b = \"\"\"x \\\na = 1\"\"\"\na = 2\n", "", "a", 3},
		{"quotes before the closing ones", "b = [\"\"\"x\"\"\"\"]\na = 1\n", "", "a", 2},
		{"multi-line literal string", "b = '''\na = 1\n'''\na = 2\n", "", "a", 4},
		{"inline table", "[t]\nx = { a = 1 }\na = 2\n", "t", "a", 3},
		{"inline table keys are not the table's", "[t]\nx = { a = 1 }\n", "t", "a", 0},
		{"inline table keys are its own", "[t]\nx = { a = 1, b = { c = 2 } }\n", "t.x", "b", 2},
		{"nested inline table", "[t]\nx = { a = 1, b = { c = 2 } }\n", "t.x.b", "c", 2},
		{"dotted key in an inline table", "x = { a.b = 1 }\n", "x.a", "b", 1},
		{"inline table after an array", "x = { a = [1, 2], b = 3 }\n", "x", "b", 1},
		{"key in a string value", "[t]\nx = \"y, a = 1\"\na = 2\n", "t", "a", 3},
		{"multi-line array", "a = [\n  1, # b = 2\n  \"]\",\n]\nb = 3\n", "", "b", 5},
		{"array of inline tables", "a = [\n  { b = 1 },\n]\nb = 2\n", "", "b", 4},
		{"array of inline tables defines their keys", "a = [\n  { b = 1 },\n  { c = 2 },\n]\n", "a", "c", 3},
		{"CRLF", "[t]\r\na = 1\r\n\"b\" = 2\r\n", "t", "b", 3},
		{"CRLF header", "[t]\r\na = 1\r\n", "t", "a", 2},
		{"byte order mark", "\ufeffa = 1\n", "", "a", 1},
		{"missing key", "a = 1\n", "", "b", 0},
		{"missing table", "a = 1\n", "t", "a", 0},
	} {
		if got := TOMLKeyLines([]byte(testCase.source)).Line(testCase.table, testCase.key); got != testCase.want {
			t.Errorf("%s: Line(%q, %q) of %q = %d, want %d", testCase.name, testCase.table, testCase.key, testCase.source, got, testCase.want)
		}
	}
}

// Verifies: REQ-LANG-033
func TestTOMLKeyLinesWithinAndElements(t *testing.T) {
	lines := TOMLKeyLines([]byte("name = \"a\"\n[[t]]\nx = 1\n[t.'case(os)'.linux]\nb = 1\n[[t]]\nb = 2\n[u]\nc = 1\n[tt]\nc = 2\n"))
	for _, testCase := range []struct {
		table, key string
		want       int
	}{
		{"t", "b", 5},
		{"t", "x", 3},
		{"", "c", 9},
		{"", "name", 1},
		{"t", "c", 0},
		{"t.case(os)", "b", 5},
		{"tt", "c", 11},
	} {
		if got := lines.Within(testCase.table, testCase.key); got != testCase.want {
			t.Errorf("Within(%q, %q) = %d, want %d", testCase.table, testCase.key, got, testCase.want)
		}
	}
	if got := lines.Elements("t"); !slices.Equal(got, []int{2, 6}) {
		t.Errorf("Elements(t) = %v, want [2 6]", got)
	}
	if got := lines.Elements("u"); got != nil {
		t.Errorf("Elements(u) = %v, want none", got)
	}
}
