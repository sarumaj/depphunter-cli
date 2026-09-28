package powershell

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Verifies: REQ-PS-009
func TestStatementLines(t *testing.T) {
	statements, _ := scanStatements("function A {\n    Import-Module X\n}\n\n  Import-Module Y; Import-Module Z\n")
	lines := map[string]int{}
	for _, s := range statements {
		lines[s.text] = s.line
	}
	want := map[string]int{"function A": 1, "Import-Module X": 2, "Import-Module Y": 5, "Import-Module Z": 5}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("got %v, want %v", lines, want)
	}
}

func extract(t *testing.T, name, source string) ([]string, map[string]string) {
	t.Helper()
	extraction, err := Plugin{}.Extract(&scan.File{Path: name}, []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	var modules []string
	for _, rawImport := range extraction.Imports {
		modules = append(modules, rawImport.Module)
	}
	symbols := map[string]string{}
	for _, s := range extraction.Symbols {
		symbols[s.Name] = s.Kind
	}
	return modules, symbols
}

// Verifies: REQ-PS-002, REQ-PS-009
func TestScannerIgnoresNonCode(t *testing.T) {
	source := "$doc = @\"\nImport-Module InHereString\n\"@\n" +
		"<#\nImport-Module InBlockComment\n#>\n" +
		"Write-Host 'not # a comment'; Import-Module A # trailing comment\n" +
		"Import-Module `\n  -Name B,\n  C\r\n" +
		"if ($x) { Import-Module D }\n"
	modules, _ := extract(t, "x.ps1", source)
	if want := []string{"A", "B", "C", "D"}; !reflect.DeepEqual(modules, want) {
		t.Errorf("got %v, want %v", modules, want)
	}
}

// Verifies: REQ-PS-009
func TestScannerClasses(t *testing.T) {
	source := "class Shape {\n  [double] Area() { if ($true) { return 0 } }\n  static [Shape] New() { return $null }\n  hidden [void] reset() {}\n  [int] $Sides = @{ a = 1 }.a\n}\n" +
		"function script:Get-Shape { param($n) }\nfilter Only-Big { $_ }\n"
	_, symbols := extract(t, "x.ps1", source)
	want := map[string]string{
		"Shape": "class", "Shape.Area": "method", "Shape.New": "method", "Shape.reset": "method",
		"Get-Shape": "func", "Only-Big": "func",
	}
	if !reflect.DeepEqual(symbols, want) {
		t.Errorf("got %v, want %v", symbols, want)
	}
}

// Verifies: REQ-PS-006
func TestManifestWithComments(t *testing.T) {
	source := "@{\n  # RequiredModules = @('Commented')\n  RequiredModules = @(\n    'A' # first\n    @{ ModuleName = 'B'; RequiredVersion = '2.0' }\n  )\n}\n"
	modules, _ := extract(t, "m.psd1", source)
	if want := []string{"B", "A"}; !reflect.DeepEqual(modules, want) {
		t.Errorf("got %v, want %v", modules, want)
	}
}
