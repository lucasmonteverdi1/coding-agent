package tools

import (
	"context"
	"encoding/json"
)

type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage
	Handler     func(ctx context.Context, args map[string]any) (string, error)
}

type Registry map[string]Tool

func NewRegistry() Registry {
	return Registry{}
}

func (r Registry) Register(t Tool) {
	r[t.Name] = t
}
