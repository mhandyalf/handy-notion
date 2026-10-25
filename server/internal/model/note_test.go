package model

import (
	"encoding/json"
	"testing"
)

func TestJSONValueAndMarshal(t *testing.T) {
	tests := []struct {
		name string
		data JSON
		want string
	}{
		{"nil defaults to array", nil, `[]`},
		{"empty defaults to array", JSON{}, `[]`},
		{"blocks", JSON(`[{"type":"paragraph","content":"hello"}]`), `[{"type":"paragraph","content":"hello"}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := tt.data.Value()
			if err != nil {
				t.Fatal(err)
			}
			data, ok := value.([]byte)
			if !ok || string(data) != tt.want {
				t.Errorf("Value() = %T(%v), want []byte(%q)", value, value, tt.want)
			}
			encoded, err := json.Marshal(struct {
				Content JSON `json:"content"`
			}{Content: tt.data})
			if err != nil {
				t.Fatal(err)
			}
			if want := `{"content":` + tt.want + `}`; string(encoded) != want {
				t.Errorf("Marshal() = %s, want %s", encoded, want)
			}
		})
	}
}

func TestJSONScan(t *testing.T) {
	for _, value := range []any{[]byte(`[{"id":"one"}]`), `[{"id":"one"}]`} {
		var got JSON = JSON(`previous content`)
		if err := got.Scan(value); err != nil {
			t.Fatalf("Scan(%T): %v", value, err)
		}
		if string(got) != `[{"id":"one"}]` {
			t.Errorf("Scan(%T) = %s", value, got)
		}
	}
	t.Run("copies database buffer", func(t *testing.T) {
		source := []byte(`[1]`)
		var got JSON
		if err := got.Scan(source); err != nil {
			t.Fatal(err)
		}
		source[1] = '2'
		if string(got) != `[1]` {
			t.Errorf("Scan retained database buffer: %s", got)
		}
	})
	t.Run("unsupported types leave value unchanged", func(t *testing.T) {
		for _, value := range []any{nil, 42, true} {
			got := JSON(`[1]`)
			if err := got.Scan(value); err == nil {
				t.Errorf("Scan(%T) succeeded, want error", value)
			}
			if string(got) != `[1]` {
				t.Errorf("Scan(%T) changed value to %s", value, got)
			}
		}
	})
}
