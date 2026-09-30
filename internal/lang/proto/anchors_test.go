package proto

import (
	"reflect"
	"testing"
)

// An anchored list of deps is read as the list it names.
//
// Verifies: REQ-LANG-032
func TestBufAnchors(t *testing.T) {
	b := readBuf("", []byte("version: v1\nx-deps: &deps\n  - buf.build/acme/pay\ndeps: *deps\n"))
	var got []string
	for _, d := range b.dependencies {
		got = append(got, d.value)
	}
	if want := []string{"buf.build/acme/pay"}; !reflect.DeepEqual(got, want) {
		t.Errorf("dependencies %v, want %v", got, want)
	}
}
