// Package chars holds the byte tests the hand-written lexers share. They look at
// bytes, not runes: a byte at or above 0x80 is part of a UTF-8 sequence, which
// the lexers of languages that allow non-ASCII names take as part of a name.
package chars

// At is s[i], or 0 past the end of s.
func At(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func IsDigit(c byte) bool  { return c >= '0' && c <= '9' }
func IsLower(c byte) bool  { return c >= 'a' && c <= 'z' }
func IsUpper(c byte) bool  { return c >= 'A' && c <= 'Z' }
func IsLetter(c byte) bool { return IsLower(c) || IsUpper(c) }
func IsAlnum(c byte) bool  { return IsLetter(c) || IsDigit(c) }

// IsWord reports whether c can be part of an ASCII name: a letter, a digit or _.
func IsWord(c byte) bool { return IsAlnum(c) || c == '_' }

// IsIdentStart reports whether c can start an ASCII name: a letter or _.
func IsIdentStart(c byte) bool { return IsLetter(c) || c == '_' }

// IsIdentStartUTF8 is IsIdentStart for a name that may be written in any script.
func IsIdentStartUTF8(c byte) bool { return IsIdentStart(c) || c >= 0x80 }

// IsIdentUTF8 is IsWord for a name that may be written in any script.
func IsIdentUTF8(c byte) bool { return IsWord(c) || c >= 0x80 }
