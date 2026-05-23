package models

import "testing"

func TestIsValidModel_KnownModel(t *testing.T) {
	known := []string{"gpt-4o", "gpt-4o-mini", "o3", "o4-mini", "gpt-4.1"}
	for _, model := range known {
		if !IsValidModel(model) {
			t.Errorf("expected %q to be valid", model)
		}
	}
}

func TestIsValidModel_UnknownModel(t *testing.T) {
	unknown := []string{"gpt-3", "claude-3", "", "not-a-model", "gpt4o"}
	for _, model := range unknown {
		if IsValidModel(model) {
			t.Errorf("expected %q to be invalid", model)
		}
	}
}
