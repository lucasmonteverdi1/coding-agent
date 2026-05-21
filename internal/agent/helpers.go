package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

func parseToolArguments(raw string) (map[string]any, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, err
	}
	return args, nil
}

func describeToolCall(name string, args map[string]any) string {
	switch name {
	case "read_file":
		return fmt.Sprintf("Reading %s", stringArg(args, "path"))
	case "write_file":
		return fmt.Sprintf("Writing %s", stringArg(args, "path"))
	case "list_files":
		return fmt.Sprintf("Listing %s", stringArg(args, "path"))
	case "run_command":
		return fmt.Sprintf("Running command: %s", stringArg(args, "command"))
	case "web_search":
		return fmt.Sprintf("Searching web: %s", stringArg(args, "query"))
	default:
		return fmt.Sprintf("Using tool: %s", name)
	}
}

func stringArg(args map[string]any, key string) string {
	value, ok := args[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "(missing argument)"
	}
	return value
}

func summarizeToolResult(result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		return "Tool finished with no output."
	}
	return fmt.Sprintf("Tool finished: %s", truncate(strings.Join(strings.Fields(result), " "), 120))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
