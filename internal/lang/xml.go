package lang

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// UnmarshalXML is xml.Unmarshal for documents that declare a Latin-1 encoding, as
// many POMs do (<?xml version="1.0" encoding="ISO-8859-1"?>): encoding/xml reads
// only UTF-8 and refuses any other declared encoding. ISO-8859-1, Latin-1 and
// Windows-1252 are read byte by byte as Latin-1 (Windows-1252's extra punctuation
// in 0x80-0x9F is not what a POM's markup is made of); US-ASCII and UTF-8 as they
// are.
//
// Implements: REQ-SUP-056
func UnmarshalXML(data []byte, v any) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(strings.TrimSpace(label)) {
		case "utf-8", "utf8", "us-ascii", "ascii":
			return input, nil
		case "iso-8859-1", "iso8859-1", "latin1", "latin-1", "l1", "windows-1252", "cp1252":
			raw, err := io.ReadAll(input)
			if err != nil {
				return nil, err
			}
			out := make([]byte, 0, len(raw)+len(raw)/8)
			for _, b := range raw {
				out = utf8.AppendRune(out, rune(b))
			}
			return bytes.NewReader(out), nil
		}
		return nil, fmt.Errorf("xml: unsupported encoding %q", label)
	}
	return d.Decode(v)
}
