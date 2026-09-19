package powershell

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

func TestStatementLines(t *testing.T) {
	stmts, _ := scanStatements("function A {\n    Import-Module X\n}\n\n  Import-Module Y; Import-Module Z\n")
	lines := map[string]int{}
	for _, s := range stmts {
		lines[s.text] = s.line
	}
	want := map[string]int{"function A": 1, "Import-Module X": 2, "Import-Module Y": 5, "Import-Module Z": 5}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("got %v, want %v", lines, want)
	}
}

func extract(t *testing.T, name, src string) ([]string, map[string]string) {
	t.Helper()
	ex, err := Plugin{}.Extract(&scan.File{Path: name}, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var mods []string
	for _, im := range ex.Imports {
		mods = append(mods, im.Module)
	}
	syms := map[string]string{}
	for _, s := range ex.Symbols {
		syms[s.Name] = s.Kind
	}
	return mods, syms
}

func TestScannerIgnoresNonCode(t *testing.T) {
	src := "$doc = @\"\nImport-Module InHereString\n\"@\n" +
		"<#\nImport-Module InBlockComment\n#>\n" +
		"Write-Host 'not # a comment'; Import-Module A # trailing comment\n" +
		"Import-Module `\n  -Name B,\n  C\r\n" +
		"if ($x) { Import-Module D }\n"
	mods, _ := extract(t, "x.ps1", src)
	if want := []string{"A", "B", "C", "D"}; !reflect.DeepEqual(mods, want) {
		t.Errorf("got %v, want %v", mods, want)
	}
}

func TestScannerClasses(t *testing.T) {
	src := "class Shape {\n  [double] Area() { if ($true) { return 0 } }\n  static [Shape] New() { return $null }\n  hidden [void] reset() {}\n  [int] $Sides = @{ a = 1 }.a\n}\n" +
		"function script:Get-Shape { param($n) }\nfilter Only-Big { $_ }\n"
	_, syms := extract(t, "x.ps1", src)
	want := map[string]string{
		"Shape": "class", "Shape.Area": "method", "Shape.New": "method", "Shape.reset": "method",
		"Get-Shape": "func", "Only-Big": "func",
	}
	if !reflect.DeepEqual(syms, want) {
		t.Errorf("got %v, want %v", syms, want)
	}
}

func TestManifestWithComments(t *testing.T) {
	src := "@{\n  # RequiredModules = @('Commented')\n  RequiredModules = @(\n    'A' # first\n    @{ ModuleName = 'B'; RequiredVersion = '2.0' }\n  )\n}\n"
	mods, _ := extract(t, "m.psd1", src)
	if want := []string{"B", "A"}; !reflect.DeepEqual(mods, want) {
		t.Errorf("got %v, want %v", mods, want)
	}
}
