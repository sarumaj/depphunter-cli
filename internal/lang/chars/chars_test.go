package chars

import "testing"

// Every test against what it is documented to accept, for every byte.
func TestEveryByte(t *testing.T) {
	in := func(c byte, sets ...string) bool {
		for _, s := range sets {
			for i := 0; i+2 < len(s)+1; i += 3 {
				if c >= s[i] && c <= s[i+2] {
					return true
				}
			}
		}
		return false
	}
	for b := 0; b < 256; b++ {
		c := byte(b)
		for _, check := range []struct {
			name string
			got  bool
			want bool
		}{
			{"IsDigit", IsDigit(c), in(c, "0-9")},
			{"IsLower", IsLower(c), in(c, "a-z")},
			{"IsUpper", IsUpper(c), in(c, "A-Z")},
			{"IsLetter", IsLetter(c), in(c, "a-z", "A-Z")},
			{"IsAlnum", IsAlnum(c), in(c, "a-z", "A-Z", "0-9")},
			{"IsWord", IsWord(c), in(c, "a-z", "A-Z", "0-9", "___")},
			{"IsIdentStart", IsIdentStart(c), in(c, "a-z", "A-Z", "___")},
			{"IsIdentStartUTF8", IsIdentStartUTF8(c), in(c, "a-z", "A-Z", "___") || c >= 0x80},
			{"IsIdentUTF8", IsIdentUTF8(c), in(c, "a-z", "A-Z", "0-9", "___") || c >= 0x80},
		} {
			if check.got != check.want {
				t.Errorf("%s(%q) = %v", check.name, c, check.got)
			}
		}
	}
	if At("ab", 1) != 'b' || At("ab", 2) != 0 {
		t.Error("At")
	}
}
