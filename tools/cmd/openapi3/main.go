// Command openapi3 converts a Swagger 2.0 YAML file into a deterministic OpenAPI 3.0 YAML file.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"gopkg.in/yaml.v3"
)

func main() {
	inPath := flag.String("in", "api/swagger.yaml", "input Swagger 2.0 YAML path")
	outPath := flag.String("out", "api/openapi.yaml", "output OpenAPI 3.0 YAML path")
	flag.Parse()

	if err := convert(*inPath, *outPath); err != nil {
		fmt.Fprintf(os.Stderr, "openapi3: %v\n", err)
		os.Exit(1)
	}
}

func convert(inPath, outPath string) error {
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}

	var parsed any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	jsonBytes, err := json.Marshal(normalizeYAML(parsed))
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	var doc2 openapi2.T
	if err := json.Unmarshal(jsonBytes, &doc2); err != nil {
		return fmt.Errorf("decode swagger 2.0: %w", err)
	}

	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return fmt.Errorf("convert to openapi 3.0: %w", err)
	}
	doc3.Servers = nil

	v3JSON, err := json.Marshal(doc3)
	if err != nil {
		return fmt.Errorf("marshal openapi 3.0 json: %w", err)
	}

	var canonical any
	if err := json.Unmarshal(v3JSON, &canonical); err != nil {
		return fmt.Errorf("unmarshal canonical: %w", err)
	}

	out, err := yaml.Marshal(canonical)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	if err := os.WriteFile(outPath, out, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

func normalizeYAML(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[k] = normalizeYAML(v)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, v := range typed {
			out[i] = normalizeYAML(v)
		}
		return out
	default:
		return value
	}
}
