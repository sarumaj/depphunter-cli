package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SaveUI writes ui into the `ui:` section of the YAML file, creating the file or the
// section as needed. Other keys, their order and comments are preserved.
//
// Implements: REQ-CFG-013, REQ-CFG-014
func SaveUI(file string, ui UI) error {
	if err := ui.Validate(); err != nil {
		return err
	}
	var doc yaml.Node
	data, err := os.ReadFile(file)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return err
	default:
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return err
		}
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return errors.New(file + ": top level is not a mapping")
	}

	var fresh yaml.Node
	if err := fresh.Encode(ui); err != nil {
		return err
	}
	section := lookup(root, "ui")
	if section == nil || section.Kind != yaml.MappingNode {
		section = &yaml.Node{Kind: yaml.MappingNode}
		set(root, "ui", section)
	}
	// Replace every UI key: an omitted (empty) filter removes a previously saved one.
	for _, key := range []string{"hide_languages", "hide_islands", "path_filter"} {
		remove(section, key)
	}
	for i := 0; i+1 < len(fresh.Content); i += 2 {
		set(section, fresh.Content[i].Value, fresh.Content[i+1])
	}

	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return err
	}
	out := buffer.Bytes()
	temporary, err := os.CreateTemp(filepath.Dir(file), ".depphunter-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(out); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if fileInfo, err := os.Stat(file); err == nil {
		os.Chmod(temporary.Name(), fileInfo.Mode().Perm())
	} else {
		os.Chmod(temporary.Name(), 0o644)
	}
	return os.Rename(temporary.Name(), file)
}

func lookup(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func set(m *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			// Keep comments attached to the old value.
			value.HeadComment, value.LineComment, value.FootComment = m.Content[i+1].HeadComment, m.Content[i+1].LineComment, m.Content[i+1].FootComment
			m.Content[i+1] = value
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func remove(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}
