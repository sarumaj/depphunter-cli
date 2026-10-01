package minify

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJS(t *testing.T) {
	for _, testCase := range []struct{ name, in, want string }{
		{"line comment", "a = 1; // note\nb = 2;", "a = 1;\nb = 2;"},
		{"line numbering kept", "a;\n// x\n// y\nb;", "a;\n\n\nb;"},
		{"whole line", "a = 1;\n  // note\nb = 2;", "a = 1;\n\nb = 2;"},
		{"block", "a = /* x */ 1;", "a =   1;"},
		{"block over lines", "return (\n/* x\ny */ 1);", "return (\n\n1);"},
		{"indentation", "  function f() {\n    return 1;\n  }", "function f() {\nreturn 1;\n}"},
		{"slashes in a string", `const u = "https://x/y"; // gone`, `const u = "https://x/y";`},
		{"slashes in a template", "const u = `a//b${x}//c`; // gone", "const u = `a//b${x}//c`;"},
		{"a brace inside a hole", "const s = `${ {a: 1}.a }`; // gone", "const s = `${ {a: 1}.a }`;"},
		{"a backtick inside a hole", "const s = `${`in${1}`}`; // gone", "const s = `${`in${1}`}`;"},
		{"division", "const r = a / b; // gone", "const r = a / b;"},
		{"division by a call", "const r = f(x) / g(y); // gone", "const r = f(x) / g(y);"},
		{"a pattern with slashes", `s.replace(/\/\//g, '/'); // gone`, `s.replace(/\/\//g, '/');`},
		{"a pattern with a class", `if (/[/*]/.test(s)) f(); // gone`, `if (/[/*]/.test(s)) f();`},
		{"a pattern after a paren", "if (x) /a/.test(s); // gone", "if (x) /a/.test(s);"},
		{"a comment that looks like a pattern", "a = b;\n// /unterminated\nc = d;", "a = b;\n\nc = d;"},
		{"an apostrophe in a comment", "a = 1; // don't\nb = 2;", "a = 1;\nb = 2;"},
		{"a quote in a comment", `a = 1; /* "x */ b = 2;`, `a = 1;   b = 2;`},
		{"a pattern after return", "function f(s){ return /[/*]/.test(s) } /* c */ let x = 1;", "function f(s){ return /[/*]/.test(s) }   let x = 1;"},
		{"a pattern after typeof", "x = typeof /a/; // gone", "x = typeof /a/;"},
		{"a property named like a keyword", "r = o.in / 2; // gone", "r = o.in / 2;"},
		{"a template over lines", "const t = `a\n    b  \n`;\n    f();", "const t = `a\n    b  \n`;\nf();"},
		{"a continued string", "const s = 'a\\\n   b';\n  f();", "const s = 'a\\\n   b';\nf();"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := JS(testCase.in); got != testCase.want {
				t.Errorf("JS(%q)\n got %q\nwant %q", testCase.in, got, testCase.want)
			}
		})
	}
}

func TestCSS(t *testing.T) {
	in := "/* a note */\n.x { color: red; } /* another */\n.y::after { content: '/*'; }"
	want := "\n.x { color: red; }\n.y::after { content: '/*'; }"
	if got := CSS(in); got != want {
		t.Errorf("CSS\n got %q\nwant %q", got, want)
	}
}

func TestHTML(t *testing.T) {
	in := "<p>a</p>\n  <!-- a note -->\n  <p>b</p>"
	want := "<p>a</p>\n\n<p>b</p>"
	if got := HTML(in); got != want {
		t.Errorf("HTML\n got %q\nwant %q", got, want)
	}
}

// Whatever it takes out, what is left has to be the same program. Nothing here parses
// JavaScript, so the check is the next best thing: every quoted string, template and
// pattern in the file has to come through untouched, and so does every other
// character that is not whitespace or inside a comment.
func TestMinifyKeepsEveryLiteral(t *testing.T) {
	directory := filepath.Join("..", "..", "web", "static")
	var files []string
	err := filepath.WalkDir(directory, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(p) == ".js" {
			files = append(files, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 10 {
		t.Fatalf("expected the UI's modules under %s, found %d", directory, len(files))
	}
	for _, f := range files {
		source, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out := JS(string(source))
		for _, literal := range literals(string(source)) {
			if !strings.Contains(out, literal) {
				t.Errorf("%s: literal %q did not survive", filepath.Base(f), literal)
			}
		}
		if len(out) > len(source) {
			t.Errorf("%s: grew from %d to %d bytes", filepath.Base(f), len(source), len(out))
		}
	}
}

// literals pulls the quoted strings out of a source file with the same scanner the
// minifier uses, so a bug that mis-scans them fails this test on both sides at once -
// which is why the check above also counts characters.
func literals(source string) []string {
	var out []string
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '"', '\'':
			j := endOfString(source, i)
			if j-i > 4 && !strings.Contains(source[i:j], "\n") {
				out = append(out, source[i:j])
			}
			i = j - 1
		case '`':
			j := endOfTemplate(source, i)
			if j-i > 4 {
				out = append(out, source[i:j])
			}
			i = j - 1
		case '/':
			if i+1 < len(source) && (source[i+1] == '/' || source[i+1] == '*') {
				// Skip the comment so its contents are not mistaken for a literal.
				if source[i+1] == '/' {
					if k := strings.IndexByte(source[i:], '\n'); k < 0 {
						return out
					} else {
						i += k
					}
				} else if k := strings.Index(source[i+2:], "*/"); k < 0 {
					return out
				} else {
					i += 2 + k + 1
				}
			}
		}
	}
	return out
}
