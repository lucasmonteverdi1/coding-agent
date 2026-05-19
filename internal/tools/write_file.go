package tools

import (
	"fmt"
	"os"
)

func WriteFileTool() Tool {
	return Tool{
		Name:        "write_file",
		Description: "Write content to a file at the given path. Creates the file if it doesn't exist.",
		Schema: []byte(`{
			"type": "object",
			"properties": {
				"path":    { "type": "string", "description": "Path to write the file" },
				"content": { "type": "string", "description": "Content to write" }
			},
			"required": ["path", "content"]
		}`),
		Handler: func(args map[string]any) (string, error) {
			path, ok := args["path"].(string)
			if !ok {
				return "", fmt.Errorf("path must be a string")
			}
			content, ok := args["content"].(string)
			if !ok {
				return "", fmt.Errorf("content must be a string")
			}
			// converting a string to []byte.os.WriteFile needs bytes, not a string.
			err := os.WriteFile(path, []byte(content), 0644)
			if err != nil {
				return "", fmt.Errorf("write_file: %w", err)
			}
			return fmt.Sprintf("wrote %s", path), nil
		},
	}
}
