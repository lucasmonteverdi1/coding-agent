package tools

import (
	"fmt"
	"os"
)

func ReadFileTool() Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read the contents of a file in the given path",
		Schema: []byte(`{
			"type": "object",
			"properties": {
				"path": { "type": "string", "description": "Path to the file to read" }
			},
			"required": ["path"]
		}`),
		Handler: func(args map[string]any) (string, error) {
			// returns 2 values: a string with the result or the error. one is always empty.
			path, ok := args["path"].(string)
			if !ok {
				return "", fmt.Errorf("path argument missing or is not a string")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("read_file: %w", err)
			}
			return string(data), nil
		},
	}
}
