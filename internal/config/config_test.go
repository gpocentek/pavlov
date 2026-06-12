package config

import "testing"

func TestEmptyRules(t *testing.T) {
	yaml := `rules: []`

	cfg, _ := LoadFromString([]byte(yaml))
	err := Validate(cfg)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "no rules found" {
		t.Fatalf("expected 'no rules found', got %v", err)
	}
}
