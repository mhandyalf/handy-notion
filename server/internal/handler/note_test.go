package handler

import (
	"encoding/json"
	"testing"
)

func TestValidBlocks(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"empty array", `[]`, true},
		{"blocks", `[{"id":"block-1","type":"paragraph","content":"hello"}]`, true},
		{"whitespace", " \n\t[ ]\r\n", true},
		{"nested values", `[[1],null,true,"text"]`, true},
		{"missing", ``, false},
		{"whitespace only", " \n\t", false},
		{"null", `null`, false},
		{"object", `{}`, false},
		{"string", `"[]"`, false},
		{"number", `42`, false},
		{"boolean", `true`, false},
		{"unclosed array", `[`, false},
		{"trailing comma", `[{},]`, false},
		{"trailing data", `[] {}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validBlocks(json.RawMessage(tt.raw)); got != tt.want {
				t.Errorf("validBlocks(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
