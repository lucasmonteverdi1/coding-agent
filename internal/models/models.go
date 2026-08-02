package models

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// ValidModels is a lookup set for O(1) membership checks.
var ValidModels = map[string]struct{}{}

// ValidModelsOrder preserves the order of models as defined in pricing.config.json.
var ValidModelsOrder = []string{}

// findPricingConfigPath attempts to locate pricing.config.json by probing
// several common locations relative to the current working directory.
func findPricingConfigPath() string {
	// 1) current working directory
	if wd, err := os.Getwd(); err == nil {
		p := filepath.Join(wd, "pricing.config.json")
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	// 2) walk up the directory tree to find the config
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 10; i++ {
			p := filepath.Join(dir, "pricing.config.json")
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	// 3) fallback: try repository root if we can deduce it from this file's location
	// Best effort: assume this file is under internal/models, so go up two levels
	if thisFile, err := os.ReadFile("internal/models/.placeholder"); err == nil {
		_ = thisFile
	}
	// Not found
	return ""
}

// init loads pricing configuration if found, preserving the order of keys in the JSON file.
func init() {
	path := findPricingConfigPath()
	if path == "" {
		// If pricing file isn't found, leave the maps empty.
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	// Read opening '{'
	t, err := dec.Token()
	if err != nil {
		return
	}
	// Expect a json.Delim with '{'
	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return
	}
	for dec.More() {
		// Read the key
		k, err := dec.Token()
		if err != nil {
			return
		}
		key, ok := k.(string)
		if !ok {
			// Skip any non-string keys defensively
			var skip interface{}
			if err := dec.Decode(&skip); err != nil {
				return
			}
			continue
		}
		// Read the value object and ignore its contents; we only need the key order
		var val map[string]interface{}
		if err := dec.Decode(&val); err != nil {
			return
		}
		ValidModelsOrder = append(ValidModelsOrder, key)
		ValidModels[key] = struct{}{}
	}
	// Consume the closing '}' if present
	// _, _ = dec.Token()
}

func IsValidModel(model string) bool {
	_, ok := ValidModels[model]
	return ok
}
