package goai

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// EmitYAML serialises a Document into deterministic YAML bytes with two
// space indentation. Stable output is mandatory because both file diff
// tools and Merge3Way depend on byte-identical re-emission for unchanged
// inputs.
func EmitYAML(doc *Document) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("goai: emit nil document")
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		_ = encoder.Close()

		return nil, fmt.Errorf("goai: yaml encode: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("goai: yaml close: %w", err)
	}

	return buf.Bytes(), nil
}
