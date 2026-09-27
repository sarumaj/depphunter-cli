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
	line  int
	obj   []jentry
	arr   []*jvalue
	str   *string
	other string // a number, true, false or null
}

type jentry struct {
	key  string
	line int
	val  *jvalue
}

// readJSON decodes a JSON document keeping the line of every key and value.
func readJSON(src []byte) *jvalue {
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()
	lines := newLineIndex(src)
	var value func() *jvalue
	value = func() *jvalue {
		off := int(dec.InputOffset())
		t, err := dec.Token()
		if err != nil {
			return nil
		}
		v := &jvalue{line: lines.at(src, off)}
		switch x := t.(type) {
		case json.Delim:
			switch x {
			case '{':
				for dec.More() {
					off := int(dec.InputOffset())
					k, err := dec.Token()
					if err != nil {
						return v
					}
					key, _ := k.(string)
					e := jentry{key: key, line: lines.at(src, off)}
					if e.val = value(); e.val == nil {
						return v
					}
					v.obj = append(v.obj, e)
				}
				dec.Token()
			case '[':
				for dec.More() {
					item := value()
					if item == nil {
						return v
					}
					v.arr = append(v.arr, item)
				}
				dec.Token()
			}
		case string:
			v.str = &x
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

func newLineIndex(src []byte) lineIndex {
	idx := lineIndex{0}
	for i, c := range src {
		if c == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

// at is the line of the first token at or after off (skipping the separators the
// decoder has not consumed yet).
func (idx lineIndex) at(src []byte, off int) int {
	for off < len(src) && strings.IndexByte(" \t\r\n,:", src[off]) >= 0 {
		off++
	}
	lo, hi := 0, len(idx)
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if idx[mid] <= off {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo + 1
}

// tokens renders a JSON value as the native-syntax expression it stands for.
func (v *jvalue) tokens() []tok {
	switch {
	case v.str != nil:
		l := &lexer{src: []byte(*v.str), line: v.line}
		t := l.template(false)
		t.line = v.line
		return []tok{t}
	case v.obj != nil:
		out := []tok{{kind: tPunct, text: "{", line: v.line}}
		for _, e := range v.obj {
			out = append(out, tok{kind: tString, text: e.key, lit: true, line: e.line}, tok{kind: tPunct, text: "=", line: e.line})
			out = append(out, e.val.tokens()...)
			out = append(out, tok{kind: tPunct, text: ",", line: e.line})
		}
		return append(out, tok{kind: tPunct, text: "}", line: v.line})
	case v.arr != nil:
		out := []tok{{kind: tPunct, text: "[", line: v.line}}
		for _, item := range v.arr {
			out = append(out, item.tokens()...)
			out = append(out, tok{kind: tPunct, text: ",", line: item.line})
		}
		return append(out, tok{kind: tPunct, text: "]", line: v.line})
	case v.other != "":
		return []tok{{kind: tIdent, text: v.other, line: v.line}}
	}
	return []tok{{kind: tPunct, text: "{", line: v.line}, {kind: tPunct, text: "}", line: v.line}}
}

// bodies are the objects a block value holds: one object, or an array of them.
func (v *jvalue) bodies() []*jvalue {
	if v.arr != nil {
		return v.arr
	}
	return []*jvalue{v}
}

// jsonBody turns an object into a block of attributes; the keys nested names
// makes blocks of their own (required_providers inside terraform).
func jsonBody(b *block, v *jvalue, nested map[string]bool) {
	for _, e := range v.obj {
		if nested[e.key] {
			for _, body := range e.val.bodies() {
				child := &block{typ: e.key, line: e.line}
				jsonBody(child, body, nil)
				b.blocks = append(b.blocks, child)
			}
			continue
		}
		b.attrs = append(b.attrs, attr{name: e.key, line: e.line, expr: e.val.tokens()})
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
func parseJSON(src []byte) *block {
	root := &block{line: 1}
	doc := readJSON(src)
	if doc == nil {
		return root
	}
	var label func(typ string, v *jvalue, labels []string, line int, n int)
	label = func(typ string, v *jvalue, labels []string, line, n int) {
		if n == 0 {
			for _, body := range v.bodies() {
				b := &block{typ: typ, labels: labels, line: line}
				jsonBody(b, body, map[string]bool{"required_providers": typ == "terraform"})
				root.blocks = append(root.blocks, b)
			}
			return
		}
		for _, e := range v.obj {
			label(typ, e.val, append(append([]string(nil), labels...), e.key), e.line, n-1)
		}
	}
	for _, e := range doc.obj {
		n, ok := jsonLabels[e.key]
		if !ok {
			continue
		}
		for _, body := range e.val.bodies() {
			label(e.key, body, nil, e.line, n)
		}
	}
	return root
}

// parseJSONVars reads a .tfvars.json file: its top-level keys.
func parseJSONVars(src []byte) *block {
	root := &block{line: 1}
	if doc := readJSON(src); doc != nil {
		for _, e := range doc.obj {
			root.attrs = append(root.attrs, attr{name: e.key, line: e.line})
		}
	}
	return root
}
