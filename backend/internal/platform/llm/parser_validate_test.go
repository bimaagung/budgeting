package llm

import (
	"strings"
	"testing"
)

func TestValidate_AcceptsConformantPayloads(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"intent":"expense","amount":45000,"category":"Makan & Minum","note":"makan siang","goal_name":"","confidence":0.95}`),
		[]byte(`{"intent":"income","amount":5000000,"category":"Gaji","note":"","goal_name":"","confidence":0.99}`),
		[]byte(`{"intent":"balance","amount":0,"category":"","note":"","goal_name":"","confidence":0.9}`),
		[]byte(`{"intent":"unknown","amount":0,"category":"","note":"","goal_name":"","confidence":0}`),
	}
	for _, raw := range cases {
		t.Run(string(raw), func(t *testing.T) {
			p, err := Validate(raw)
			if err != nil {
				t.Fatalf("Validate err: %v", err)
			}
			if p.Intent == "" {
				t.Errorf("intent empty")
			}
		})
	}
}

func TestValidate_RejectsMalformed(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		msg  string
	}{
		{"invalid JSON", `{not json`, "invalid JSON"},
		{"unknown intent", `{"intent":"transfer","amount":0,"category":"","note":"","goal_name":"","confidence":0.5}`, "unknown intent"},
		{"negative amount", `{"intent":"expense","amount":-5,"category":"","note":"","goal_name":"","confidence":0.5}`, "negative amount"},
		{"confidence > 1", `{"intent":"expense","amount":5,"category":"","note":"","goal_name":"","confidence":1.5}`, "confidence out of range"},
		{"confidence < 0", `{"intent":"expense","amount":5,"category":"","note":"","goal_name":"","confidence":-0.1}`, "confidence out of range"},
		{"missing intent", `{"amount":0,"confidence":0.5}`, "unknown intent"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Validate([]byte(tt.raw))
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tt.msg) {
				t.Errorf("err = %v, want contains %q", err, tt.msg)
			}
		})
	}
}
