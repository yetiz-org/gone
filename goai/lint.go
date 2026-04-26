package goai

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// LintReport is the structured outcome of a LintYAML pass. Errors carries
// human-readable messages; Paths and Operations are headline counts that
// callers (CI badges, status lines) can surface verbatim.
type LintReport struct {
	Errors     []string
	Paths      int
	Operations int
}

// LintYAML performs a lightweight structural check on an OpenAPI 3.0.x
// yaml document. The check is intentionally tighter than "did it parse"
// but looser than a full Spectral / Redocly validation:
//
//   - The document MUST parse as yaml.
//   - The root MUST be a mapping.
//   - openapi MUST be present and start with "3.".
//   - info MUST be present, be a mapping, and carry a non-empty title.
//   - paths MUST be present and contain at least one entry.
//   - Every entry under paths MUST be a mapping.
//
// The intent is to catch the common "shipped a broken yaml" failure modes
// in CI without standing up a full validator pipeline. Returns a non-nil
// LintReport even when errors exist; the caller decides whether to fail.
func LintYAML(body []byte) (*LintReport, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty document")
	}

	var node yaml.Node
	if err := yaml.Unmarshal(body, &node); err != nil {
		return nil, err
	}

	report := &LintReport{}
	root := documentRoot(&node)
	if root == nil || root.Kind != yaml.MappingNode {
		report.Errors = append(report.Errors, "root must be a mapping (object)")

		return report, nil
	}

	openapi := mappingValue(root, "openapi")
	switch {
	case openapi == nil:
		report.Errors = append(report.Errors, "missing required key: openapi")
	case openapi.Kind != yaml.ScalarNode:
		report.Errors = append(report.Errors, "openapi must be a scalar string")
	case !strings.HasPrefix(openapi.Value, "3."):
		report.Errors = append(report.Errors, fmt.Sprintf("openapi must be 3.x; got %q", openapi.Value))
	}

	info := mappingValue(root, "info")
	switch {
	case info == nil:
		report.Errors = append(report.Errors, "missing required key: info")
	case info.Kind != yaml.MappingNode:
		report.Errors = append(report.Errors, "info must be a mapping")
	default:
		title := mappingValue(info, "title")
		if title == nil || title.Kind != yaml.ScalarNode || strings.TrimSpace(title.Value) == "" {
			report.Errors = append(report.Errors, "info.title must be a non-empty string")
		}

		version := mappingValue(info, "version")
		if version == nil || version.Kind != yaml.ScalarNode || strings.TrimSpace(version.Value) == "" {
			report.Errors = append(report.Errors, "info.version must be a non-empty string")
		}
	}

	paths := mappingValue(root, "paths")
	switch {
	case paths == nil:
		report.Errors = append(report.Errors, "missing required key: paths")
	case paths.Kind != yaml.MappingNode:
		report.Errors = append(report.Errors, "paths must be a mapping")
	case len(paths.Content) == 0:
		report.Errors = append(report.Errors, "paths must contain at least one entry")
	default:
		httpVerbs := map[string]bool{
			"get": true, "put": true, "post": true, "delete": true,
			"options": true, "head": true, "patch": true, "trace": true,
		}

		for i := 0; i+1 < len(paths.Content); i += 2 {
			report.Paths++
			pathName := paths.Content[i].Value
			item := paths.Content[i+1]
			if item.Kind != yaml.MappingNode {
				report.Errors = append(report.Errors,
					fmt.Sprintf("paths.%s must be a mapping", pathName))

				continue
			}

			for j := 0; j+1 < len(item.Content); j += 2 {
				if httpVerbs[item.Content[j].Value] {
					report.Operations++
				}
			}
		}
	}

	return report, nil
}
