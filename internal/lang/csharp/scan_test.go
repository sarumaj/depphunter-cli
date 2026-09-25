package csharp

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Verifies: REQ-CS-005
func TestScannerEdgeCases(t *testing.T) {
	src := `// using Commented;
/* using BlockCommented; */
#if DEBUG
using System.Diagnostics;
#endif
using System.Text;

namespace Outer
{
    using Inner.Stuff;

    public sealed class Widget<T> : IDisposable where T : class, new()
    {
        private const string Raw = """
            using NotReal;
            { unbalanced
            """;
        private string verbatim = @"C:\path ""quoted"" { ";
        private string interpolated = $"{Raw} using Nope;";
        private char brace = '{';

        public Widget() { }
        public static IEnumerable<T> Items<TKey>(Func<T, TKey> key) where TKey : notnull => null;
        protected virtual async Task<int> RunAsync(CancellationToken ct = default)
        {
            int Local() => 1;
            if (ct.IsCancellationRequested) { return 0; }
            return Local();
        }
        public int Count { get; set; }

        private class Nested
        {
            internal void Deep() { }
        }
    }

    public interface IThing { void Run(); }
}
`
	ex, err := Plugin{}.Extract(&scan.File{Path: "x.cs"}, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var usings []string
	for _, im := range ex.Imports {
		usings = append(usings, im.Module)
	}
	if want := []string{"System.Diagnostics", "System.Text", "Inner.Stuff"}; !reflect.DeepEqual(usings, want) {
		t.Errorf("usings %v, want %v", usings, want)
	}
	symbols := map[string]string{}
	lines := map[string]int{}
	for _, s := range ex.Symbols {
		symbols[s.Name], lines[s.Name] = s.Kind, s.Line
	}
	want := map[string]string{
		"Widget": "class", "Widget.Items": "method", "Widget.RunAsync": "method",
		"Nested": "class", "Nested.Deep": "method", "IThing": "interface",
	}
	if !reflect.DeepEqual(symbols, want) {
		t.Errorf("symbols %v, want %v", symbols, want)
	}
	if lines["Widget.RunAsync"] != 24 {
		t.Errorf("RunAsync on line %d, want 24", lines["Widget.RunAsync"])
	}
}

// Verifies: REQ-CS-006
func TestInterpolatedStrings(t *testing.T) {
	src := "class A\n{\n" +
		"    string a = $\"{(ok ? \"x;{\" : \"b\")} {{not a hole}} {d:yyyy-MM-dd} {'{'}\";\n" +
		"    string b = $@\"C:\\{name}\\\"\"x\"\" {(y ? \"}\" : \"{\")}\";\n" +
		"    string c = $$\"\"\"{{x}} \"quoted\" \"\"\";\n" +
		"    public void AfterStrings() { }\n" +
		"}\n"
	ex, err := Plugin{}.Extract(&scan.File{Path: "a.cs"}, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, s := range ex.Symbols {
		got[s.Name] = s.Line
	}
	if want := map[string]int{"A": 1, "A.AfterStrings": 6}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
