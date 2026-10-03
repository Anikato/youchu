package mcp

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestParentIDInputSchema(t *testing.T) {
	payloads := []string{
		`{"id":1,"version":1,"parent_id":2}`,
		`{"id":1,"version":1,"parent_id":null}`,
		`{"id":1,"version":1}`,
	}
	for name, schema := range map[string]*jsonschema.Schema{
		"category": parentIDInputSchema[updateCategoryArgs](),
		"location": locationUpdateInputSchema(),
	} {
		resolved, err := schema.Resolve(nil)
		if err != nil {
			t.Fatalf("%s resolve: %v", name, err)
		}
		for _, raw := range payloads {
			var instance any
			if err := json.Unmarshal([]byte(raw), &instance); err != nil {
				t.Fatal(err)
			}
			if err := resolved.Validate(instance); err != nil {
				t.Fatalf("%s %s: %v", name, raw, err)
			}
		}
	}
}
