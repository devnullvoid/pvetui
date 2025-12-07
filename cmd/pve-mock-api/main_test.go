package main

import (
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
)

func TestGenerateMockData(t *testing.T) {
    // Helper to create schema from JSON
    createSchema := func(jsonSchema string) *openapi3.Schema {
        schema := &openapi3.Schema{}
        err := json.Unmarshal([]byte(jsonSchema), schema)
        if err != nil {
            t.Fatalf("Failed to unmarshal schema: %v", err)
        }
        return schema
    }

    t.Run("string", func(t *testing.T) {
        s := createSchema(`{"type": "string"}`)
        res := generateMockData(s, 0)
        assert.Equal(t, "mock_string", res)
    })

    t.Run("integer", func(t *testing.T) {
        s := createSchema(`{"type": "integer"}`)
        res := generateMockData(s, 0)
        assert.Equal(t, 1, res)
    })

    t.Run("oneOf", func(t *testing.T) {
        s := createSchema(`{"oneOf": [{"type": "integer"}, {"type": "string"}]}`)
        res := generateMockData(s, 0)
        assert.Equal(t, 1, res)
    })

    t.Run("allOf objects", func(t *testing.T) {
        s := createSchema(`{"allOf": [
            {"type": "object", "properties": {"a": {"type": "integer"}}},
            {"type": "object", "properties": {"b": {"type": "string"}}}
        ]}`)
        res := generateMockData(s, 0)
        obj, ok := res.(map[string]interface{})
        assert.True(t, ok)
        assert.Equal(t, 1, obj["a"])
        assert.Equal(t, "mock_string", obj["b"])
    })

    t.Run("recursion limit", func(t *testing.T) {
        s := &openapi3.Schema{
            Type: &openapi3.Types{"array"},
        }
        // Infinite recursion simulation
        s.Items = &openapi3.SchemaRef{Value: s}

        // Should not panic/hang
        res := generateMockData(s, 0)
        assert.NotNil(t, res)
    })
}
