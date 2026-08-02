package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func RunCommandTool() Tool {
	return Tool{
		Name:        "run_command",
		Description: "Run a shell command and return its stdout and stderr.",
		Schema: []byte(`{
			"type": "object",
			"properties": {
				"command": { "type": "string", "description": "The shell command to run" }
			},
			"required": ["command"]
		}`),
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			command, ok := args["command"].(string)
			if !ok {
				return "", fmt.Errorf("command must be a string")
			}
			// Derived from the agent's context, so upstream cancellation
			// propagates instead of the command running to its full timeout.
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			out := strings.TrimSpace(stdout.String())
			errOut := strings.TrimSpace(stderr.String())

			if errOut != "" {
				out += "\n[stderr]\n" + errOut
			}
			if err != nil {
				return out, fmt.Errorf("run_command: %w", err)
			}
			return out, nil
		},
	}
}
