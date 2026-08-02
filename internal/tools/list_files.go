package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func ListFilesTool() Tool {
	return Tool{
		Name:        "list_files",
		Description: "Lists all files and directories in a given path",
		Schema: []byte(`{
			"type": "object",
			"properties": {
				"path": { "type": "string", "description": "Directory path to list" }
			},
			"required": ["path"]
		}`),
		Handler: func(_ context.Context, args map[string]any) (string, error) {
			path, ok := args["path"].(string)
			if !ok {
				return "", fmt.Errorf("path must be a string")
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return "", fmt.Errorf("list_files: %w", err)
			}
			var lines []string
			for _, entry := range entries {
				lines = append(lines, entry.Name())
			}
			return strings.Join(lines, "\n"), nil
		},
	}
}
