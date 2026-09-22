package graph

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The graph document crosses three languages: it is declared here, served as JSON,
// read by the browser (which needs no declaration) and read by the VS Code extension
// (which does). That last copy used to be typed out by hand in api.ts, and a hand
// copy of a struct is a hand copy: a field added here and forgotten there does not
// fail to compile, it simply stops being read, and nothing says so.
//
// So it is generated. This test writes extension/src/graph.ts from the declarations
// in graph.go - fields, JSON names, whether they may be absent, and the doc comments
// that explain them - and, on an ordinary run, checks that what is committed is what
// those declarations say. `go test ./internal/graph -update` rewrites it.
//
// A generator rather than a schema language because there is one file to read and
// three types in it, and because the alternative is a toolchain: the repository
// builds with `go build` and the UI has no build step at all.

var update = flag.Bool("update", false, "rewrite extension/src/graph.ts from graph.go")

// generated is where the TypeScript lands, relative to the repository root.
const generated = "extension/src/graph.ts"

// exported are the types the extension reads, in the order they are written out.
var exported = []string{"Graph", "Node", "Edge"}

func TestGeneratedTypeScriptMatchesTheGoDeclarations(t *testing.T) {
	root := repoRoot(t)
	want, err := renderTypeScript(filepath.Join(root, "graph.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "..", "..", generated)
	if *update {
		if err := os.WriteFile(out, want, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", generated)
		return
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("%s: %v (run: go test ./internal/graph -update)", generated, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is not what graph.go declares. Run:\n\tgo test ./internal/graph -update\n\n%s",
			generated, firstDifference(got, want))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// renderTypeScript reads the declarations out of graph.go and writes the interfaces
// the extension imports.
func renderTypeScript(path string) ([]byte, error) {
	fSet := token.NewFileSet()
	file, err := parser.ParseFile(fSet, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	types, unions := declarations(file)

	var b bytes.Buffer
	b.WriteString(`// The graph document, as internal/graph declares it.
//
// Generated from internal/graph/graph.go - do not edit. To change it, change the Go
// declaration and run:
//
//	go test ./internal/graph -update
//
// The check that this file still says what Go says runs with the ordinary tests.
`)
	for _, name := range exported {
		spec, ok := types[name]
		if !ok {
			return nil, fmt.Errorf("graph.go declares no type %s", name)
		}
		b.WriteString("\n")
		b.WriteString(comment(spec.doc, ""))
		fmt.Fprintf(&b, "export interface %s {\n", tsName(name))
		fields, err := fieldsOf(spec.node, types, unions)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for i, f := range fields {
			if i > 0 && f.doc != "" {
				b.WriteString("\n")
			}
			b.WriteString(comment(f.doc, "  "))
			optional := ""
			if f.optional {
				optional = "?"
			}
			fmt.Fprintf(&b, "  %s%s: %s;\n", f.json, optional, f.ts)
		}
		b.WriteString("}\n")
	}
	return b.Bytes(), nil
}

type decl struct {
	node *ast.StructType
	doc  string
}

// declarations collects the struct types and the string-constant groups, which is
// where the unions come from: NodeKind's constants are what a node's kind may be.
func declarations(file *ast.File) (map[string]decl, map[string][]string) {
	types := map[string]decl{}
	unions := map[string][]string{}
	for _, d := range file.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if st, ok := s.Type.(*ast.StructType); ok {
					doc := s.Doc
					if doc == nil {
						doc = gen.Doc
					}
					types[s.Name.Name] = decl{node: st, doc: text(doc)}
				}
			case *ast.ValueSpec:
				// `KindDir NodeKind = "dir"` in a const block: the named type gets
				// the literal as one of its allowed values.
				named, ok := s.Type.(*ast.Ident)
				if !ok || len(s.Values) != 1 {
					continue
				}
				if lit, ok := s.Values[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					unions[named.Name] = append(unions[named.Name], strings.Trim(lit.Value, `"`))
				}
			}
		}
	}
	return types, unions
}

type field struct {
	json     string
	ts       string
	optional bool
	doc      string
}

func fieldsOf(st *ast.StructType, types map[string]decl, unions map[string][]string) ([]field, error) {
	var out []field
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) != 1 || !f.Names[0].IsExported() {
			continue
		}
		name, opts := jsonTag(strings.Trim(f.Tag.Value, "`"))
		if name == "-" || name == "" {
			continue
		}
		ts, err := tsType(f.Type, types, unions)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Names[0].Name, err)
		}
		out = append(out, field{json: name, ts: ts, optional: opts, doc: text(f.Doc)})
	}
	return out, nil
}

// jsonTag reads the encoding/json tag: the name it serializes under, and whether it
// may be absent from the document (omitempty, which is what makes a field optional
// on the other side).
func jsonTag(tag string) (string, bool) {
	value := ""
	for _, part := range strings.Fields(tag) {
		if rest, ok := strings.CutPrefix(part, `json:"`); ok {
			value = strings.TrimSuffix(rest, `"`)
		}
	}
	name, opts, _ := strings.Cut(value, ",")
	return name, strings.Contains(opts, "omitempty")
}

func tsType(expr ast.Expr, types map[string]decl, unions map[string][]string) (string, error) {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "string", nil
		case "bool":
			return "boolean", nil
		case "int", "int32", "int64", "float64":
			return "number", nil
		}
		// A struct declared in the same file is one of the document's own types.
		if _, ok := types[t.Name]; ok {
			return tsName(t.Name), nil
		}
		if values, ok := unions[t.Name]; ok {
			quoted := make([]string, len(values))
			for i, v := range values {
				quoted[i] = `'` + v + `'`
			}
			return strings.Join(quoted, " | "), nil
		}
		return "", fmt.Errorf("no TypeScript for %s", t.Name)
	case *ast.SelectorExpr:
		// time.Time serializes as an RFC 3339 string, and nothing else from another
		// package is in this document.
		if pkg, ok := t.X.(*ast.Ident); ok && pkg.Name == "time" && t.Sel.Name == "Time" {
			return "string", nil
		}
		return "", fmt.Errorf("no TypeScript for %s.%s", t.X, t.Sel.Name)
	case *ast.StarExpr:
		return tsType(t.X, types, unions)
	case *ast.ArrayType:
		elem, err := tsType(t.Elt, types, unions)
		if err != nil {
			return "", err
		}
		if strings.Contains(elem, " | ") { // a union needs holding together
			elem = "(" + elem + ")"
		}
		return elem + "[]", nil
	}
	return "", fmt.Errorf("no TypeScript for %T", expr)
}

// tsName is what a Go type is called on the other side. Node and Edge are the graph's
// own, and "Node" alone would be a word the editor's API already uses.
func tsName(name string) string {
	switch name {
	case "Node", "Edge":
		return "Graph" + name
	}
	return name
}

func text(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	return strings.TrimSpace(doc.Text())
}

func comment(doc, indent string) string {
	if doc == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(indent + "/**\n")
	for _, line := range strings.Split(doc, "\n") {
		b.WriteString(strings.TrimRight(indent+" * "+line, " ") + "\n")
	}
	b.WriteString(indent + " */\n")
	return b.String()
}

// firstDifference points at where the committed file and the declarations part ways,
// so the failure names the field rather than printing two files.
func firstDifference(got, want []byte) string {
	a, b := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := range max(len(a), len(b)) {
		x, y := at(a, i), at(b, i)
		if x != y {
			return fmt.Sprintf("line %d:\n\tcommitted: %s\n\tdeclared:  %s", i+1, x, y)
		}
	}
	return ""
}

func at(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "(end of file)"
}
