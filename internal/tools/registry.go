package tools

import "encoding/json"

type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage
	Handler     func(args map[string]any) (string, error)
}

type Registry map[string]Tool

func NewRegistry() Registry {
	return Registry{}
}

func (r Registry) Register(t Tool) {
	r[t.Name] = t
}
