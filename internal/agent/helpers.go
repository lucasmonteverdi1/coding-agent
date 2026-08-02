package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
)

// isZeroMessage reports whether a result slot was never filled in. Every tool
// message the loop produces sets OfTool.
func isZeroMessage(m openai.ChatCompletionMessageParamUnion) bool {
	return m.OfTool == nil
}

// errorType gives error.type a bounded-cardinality value, never the error's
// message. Concrete types are only useful when they carry meaning; the generic
// stdlib wrappers collapse to "_OTHER" per the OTel convention.
func errorType(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	}
	switch fmt.Sprintf("%T", err) {
	case "*errors.errorString", "*fmt.wrapError":
		return "_OTHER"
	default:
		return fmt.Sprintf("%T", err)
	}
}

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
