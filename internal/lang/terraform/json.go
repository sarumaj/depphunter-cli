package terraform

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Terraform's JSON syntax (.tf.json) says the same as the native syntax, with the
// block structure implied by the keys. It is read best effort: the top-level block
// types are known and turned into the blocks readModule takes, every other value
// becomes an attribute whose expression is made of tokens again (a string is a
// template, so its interpolations are references), and lines are those of the keys.

type jvalue struct {
	line   int
	object []jentry
	array  []*jvalue
	text   *string
	other  string // a number, true, false or null
}

type jentry struct {
	key   string
	line  int
	value *jvalue
}

// readJSON decodes a JSON document keeping the line of every key and value.
func readJSON(source []byte) *jvalue {
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.UseNumber()
	lines := newLineIndex(source)
	var value func() *jvalue
	value = func() *jvalue {
		off := int(decoder.InputOffset())
		t, err := decoder.Token()
		if err != nil {
			return nil
		}
		v := &jvalue{line: lines.at(source, off)}
		switch x := t.(type) {
		case json.Delim:
			switch x {
			case '{':
				for decoder.More() {
					off := int(decoder.InputOffset())
					k, err := decoder.Token()
					if err != nil {
						return v
					}
					key, _ := k.(string)
					e := jentry{key: key, line: lines.at(source, off)}
					if e.value = value(); e.value == nil {
						return v
					}
					v.object = append(v.object, e)
				}
				decoder.Token()
			case '[':
				for decoder.More() {
					item := value()
					if item == nil {
						return v
					}
					v.array = append(v.array, item)
				}
				decoder.Token()
			}
		case string:
			v.text = &x
		case json.Number:
			v.other = string(x)
		case bool:
			v.other = "false"
			if x {
				v.other = "true"
			}
		default:
			v.other = "null"
		}
		return v
	}
	return value()
}

type lineIndex []int // offsets of line starts

func newLineIndex(source []byte) lineIndex {
	index := lineIndex{0}
	for i, c := range source {
		if c == '\n' {
			index = append(index, i+1)
		}
	}
	return index
}

// at is the line of the first token at or after off (skipping the separators the
// decoder has not consumed yet).
func (index lineIndex) at(source []byte, off int) int {
	for off < len(source) && strings.IndexByte(" \t\r\n,:", source[off]) >= 0 {
		off++
	}
	low, high := 0, len(index)
	for low+1 < high {
		mid := (low + high) / 2
		if index[mid] <= off {
			low = mid
		} else {
			high = mid
		}
	}
	return low + 1
}

// tokens renders a JSON value as the native-syntax expression it stands for.
func (v *jvalue) tokens() []token {
	switch {
	case v.text != nil:
		l := &lexer{source: []byte(*v.text), line: v.line}
		t := l.template(false)
		t.line = v.line
		return []token{t}
	case v.object != nil:
		out := []token{{kind: tPunctuation, text: "{", line: v.line}}
		for _, e := range v.object {
			out = append(out, token{kind: tString, text: e.key, literal: true, line: e.line}, token{kind: tPunctuation, text: "=", line: e.line})
			out = append(out, e.value.tokens()...)
			out = append(out, token{kind: tPunctuation, text: ",", line: e.line})
		}
		return append(out, token{kind: tPunctuation, text: "}", line: v.line})
	case v.array != nil:
		out := []token{{kind: tPunctuation, text: "[", line: v.line}}
		for _, item := range v.array {
			out = append(out, item.tokens()...)
			out = append(out, token{kind: tPunctuation, text: ",", line: item.line})
		}
		return append(out, token{kind: tPunctuation, text: "]", line: v.line})
	case v.other != "":
		return []token{{kind: tIdentifier, text: v.other, line: v.line}}
	}
	return []token{{kind: tPunctuation, text: "{", line: v.line}, {kind: tPunctuation, text: "}", line: v.line}}
}

// bodies are the objects a block value holds: one object, or an array of them.
func (v *jvalue) bodies() []*jvalue {
	if v.array != nil {
		return v.array
	}
	return []*jvalue{v}
}

// jsonBody turns an object into a block of attributes; the keys nested names
// makes blocks of their own (required_providers inside terraform).
func jsonBody(b *block, v *jvalue, nested map[string]bool) {
	for _, e := range v.object {
		if nested[e.key] {
			for _, body := range e.value.bodies() {
				child := &block{typeName: e.key, line: e.line}
				jsonBody(child, body, nil)
				b.blocks = append(b.blocks, child)
			}
			continue
		}
		b.attributes = append(b.attributes, attribute{name: e.key, line: e.line, expression: e.value.tokens()})
	}
}

// jsonLabels is how many labels each top-level block type has.
var jsonLabels = map[string]int{
	"resource": 2, "data": 2, "ephemeral": 2, "module": 1, "variable": 1, "output": 1,
	"provider": 1, "check": 1, "locals": 0, "terraform": 0,
}

// parseJSON reads a .tf.json file into the blocks of the native syntax.
//
// Implements: REQ-TERRAFORM-001
func parseJSON(source []byte) *block {
	root := &block{line: 1}
	doc := readJSON(source)
	if doc == nil {
		return root
	}
	var label func(typeName string, v *jvalue, labels []string, line int, n int)
	label = func(typeName string, v *jvalue, labels []string, line, n int) {
		if n == 0 {
			for _, body := range v.bodies() {
				b := &block{typeName: typeName, labels: labels, line: line}
				jsonBody(b, body, map[string]bool{"required_providers": typeName == "terraform"})
				root.blocks = append(root.blocks, b)
			}
			return
		}
		for _, e := range v.object {
			label(typeName, e.value, append(append([]string(nil), labels...), e.key), e.line, n-1)
		}
	}
	for _, e := range doc.object {
		n, ok := jsonLabels[e.key]
		if !ok {
			continue
		}
		for _, body := range e.value.bodies() {
			label(e.key, body, nil, e.line, n)
		}
	}
	return root
}

// parseJSONVariables reads a .tfvars.json file: its top-level keys.
func parseJSONVariables(source []byte) *block {
	root := &block{line: 1}
	if doc := readJSON(source); doc != nil {
		for _, e := range doc.object {
			root.attributes = append(root.attributes, attribute{name: e.key, line: e.line})
		}
	}
	return root
}
