package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/metrics"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/ci"
	"github.com/sarumaj/depphunter-cli/internal/lang/csharp"
	"github.com/sarumaj/depphunter-cli/internal/lang/golang"
	"github.com/sarumaj/depphunter-cli/internal/lang/java"
	"github.com/sarumaj/depphunter-cli/internal/lang/javascript"
	"github.com/sarumaj/depphunter-cli/internal/lang/markdown"
	"github.com/sarumaj/depphunter-cli/internal/lang/powershell"
	"github.com/sarumaj/depphunter-cli/internal/lang/python"
	"github.com/sarumaj/depphunter-cli/internal/lang/rust"
)

// referenceFiles is the size of the reference project REQ-LANG-029 is stated for.
const referenceFiles = 10_000

// writeReferenceProject generates the reference project: n source files of about
// 4 KB, a third each Go, TypeScript and Python, in 100 packages of 10 directories,
// each importing the standard library, an external package and a sibling package,
// with the manifests that resolve them. Everything is claimed by a tree-sitter or
// go/parser plugin, which makes it harder than a real repository of the same size,
// where documentation, data and configuration files are only counted.
func writeReferenceProject(tb testing.TB, root string, n int) {
	tb.Helper()
	write := func(p, s string) {
		abs := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(s), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	write("go.mod", "module example.com/big\n\ngo 1.22\n\nrequire github.com/pkg/errors v0.9.1\n")
	write("package.json", `{"name":"big","dependencies":{"react":"^18.2.0","lodash":"4.17.21"}}`+"\n")
	write("requirements.txt", "requests==2.31.0\nPyYAML==6.0\n")
	for i := range n {
		dir := fmt.Sprintf("pkg%d/sub%d", i%100, i/100%10)
		var b strings.Builder
		switch i % 3 {
		case 0:
			fmt.Fprintf(&b, "package sub%d\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\t\"github.com/pkg/errors\"\n\t\"example.com/big/pkg%d/sub0\"\n)\n\n", i/100%10, (i+1)%100)
			for j := range 18 {
				fmt.Fprintf(&b, "// F%d does things.\nfunc F%d_%d(a, b string) (string, error) {\n\tif a == \"\" {\n\t\treturn \"\", errors.New(\"empty\")\n\t}\n\treturn fmt.Sprintf(\"%%s-%%s\", strings.ToUpper(a), b), nil\n}\n\n", j, i, j)
			}
			write(fmt.Sprintf("%s/f%d.go", dir, i), b.String())
		case 1:
			fmt.Fprintf(&b, "import React from 'react';\nimport { map } from 'lodash';\nimport { x } from '../../pkg%d/sub0/t%d';\n\n", (i+1)%100, (i+1)/3*3+1)
			for j := range 16 {
				fmt.Fprintf(&b, "export function f%d(a: number, b: string): string {\n  const r = map([a, b], v => String(v));\n  if (a > %d) { return r.join(\",\") + b; }\n  return `${a}:${b}`;\n}\n\nexport class C%d { m(): number { return %d; } }\n\n", j, j, j, j)
			}
			write(fmt.Sprintf("%s/t%d.ts", dir, i), b.String())
		default:
			b.WriteString("import os\nimport requests\nimport yaml\nfrom . import helpers\n\n")
			for j := range 22 {
				fmt.Fprintf(&b, "def f%d(a, b):\n    \"\"\"Doc %d.\"\"\"\n    if a:\n        return os.path.join(a, b)\n    return requests.get(b).text\n\n\nclass K%d:\n    def m(self):\n        return %d\n\n", j, j, j, j)
			}
			write(fmt.Sprintf("%s/p%d.py", dir, i), b.String())
		}
	}
}

// cpuSeconds is the CPU time this process has spent running Go code so far.
func cpuSeconds() float64 {
	s := []metrics.Sample{{Name: "/cpu/classes/total:cpu-seconds"}, {Name: "/cpu/classes/idle:cpu-seconds"}}
	metrics.Read(s)
	return s[0].Value.Float64() - s[1].Value.Float64()
}

// BenchmarkColdAnalysis measures a cold analysis (no cache) of the reference project
// with every plugin the command runs. Besides the wall time it reports the CPU time,
// which does not depend on how many cores the machine has, and against the wall time
// shows how well the analysis uses them:
//
//	go test ./internal/analyze -run '^$' -bench ColdAnalysis -benchtime 3x -cpu 1,4
//
// Verifies: REQ-LANG-029
func BenchmarkColdAnalysis(b *testing.B) {
	root := b.TempDir()
	writeReferenceProject(b, root, referenceFiles)
	plugins := []lang.Plugin{
		golang.Plugin{}, javascript.Plugin{}, python.Plugin{}, rust.Plugin{}, java.Plugin{},
		csharp.Plugin{}, powershell.Plugin{}, ci.Plugin{}, markdown.Plugin{},
	}
	b.ResetTimer()
	cpu := cpuSeconds()
	for range b.N {
		_, st, err := Run(context.Background(), root, Options{Plugins: plugins})
		if err != nil {
			b.Fatal(err)
		}
		if st.Parsed != referenceFiles {
			b.Fatalf("parsed %d files, want %d", st.Parsed, referenceFiles)
		}
	}
	b.ReportMetric((cpuSeconds()-cpu)/float64(b.N), "cpu-s/op")
}
