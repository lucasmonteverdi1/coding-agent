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
		Handler: func(args map[string]any) (string, error) {
			command, ok := args["command"].(string)
			if !ok {
				return "", fmt.Errorf("command must be a string")
			}
			// new context that auto-cancels after 30s, plus a cancel function.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			// releases resources even if the timeout doesn't fire (Idiomatic Go for cleanup)
			// 'defer' schedules a call to run when the surrounding function returns, no matter how
			defer cancel()

			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			// in-memory buffer. The command writes into them instead of the terminal.
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
