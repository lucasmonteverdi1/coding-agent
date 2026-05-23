package agent

import (
	"strings"
	"testing"
)

// --- truncate ---

func TestTruncate_ShortString(t *testing.T) {
	result := truncate("hello", 10)
	if result != "hello" {
		t.Errorf("expected %q, got %q", "hello", result)
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	result := truncate("hello", 5)
	if result != "hello" {
		t.Errorf("expected %q, got %q", "hello", result)
	}
}

func TestTruncate_LongString(t *testing.T) {
	result := truncate("hello world", 5)
	if result != "hello..." {
		t.Errorf("expected %q, got %q", "hello...", result)
	}
}

// --- summarizeToolResult ---

func TestSummarizeToolResult_EmptyResult(t *testing.T) {
	result := summarizeToolResult("")
	if result != "Tool finished with no output." {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestSummarizeToolResult_WhitespaceOnly(t *testing.T) {
	result := summarizeToolResult("   \n  ")
	if result != "Tool finished with no output." {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestSummarizeToolResult_ShortOutput(t *testing.T) {
	result := summarizeToolResult("ok")
	if !strings.HasPrefix(result, "Tool finished:") {
		t.Errorf("expected prefix 'Tool finished:', got %q", result)
	}
}

func TestSummarizeToolResult_LongOutputTruncated(t *testing.T) {
	long := strings.Repeat("a", 200)
	result := summarizeToolResult(long)
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected truncation with '...', got %q", result)
	}
}

// --- parseToolArguments ---

func TestParseToolArguments_ValidJSON(t *testing.T) {
	args, err := parseToolArguments(`{"path": "main.go"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args["path"] != "main.go" {
		t.Errorf("expected path=main.go, got %v", args["path"])
	}
}

func TestParseToolArguments_InvalidJSON(t *testing.T) {
	_, err := parseToolArguments(`not json`)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseToolArguments_EmptyObject(t *testing.T) {
	args, err := parseToolArguments(`{}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 0 {
		t.Errorf("expected empty map, got %v", args)
	}
}

// --- stringArg ---

func TestStringArg_PresentKey(t *testing.T) {
	args := map[string]any{"path": "main.go"}
	result := stringArg(args, "path")
	if result != "main.go" {
		t.Errorf("expected %q, got %q", "main.go", result)
	}
}

func TestStringArg_MissingKey(t *testing.T) {
	args := map[string]any{}
	result := stringArg(args, "path")
	if result != "(missing argument)" {
		t.Errorf("expected '(missing argument)', got %q", result)
	}
}

func TestStringArg_WhitespaceValue(t *testing.T) {
	args := map[string]any{"path": "   "}
	result := stringArg(args, "path")
	if result != "(missing argument)" {
		t.Errorf("expected '(missing argument)' for whitespace, got %q", result)
	}
}

func TestStringArg_WrongType(t *testing.T) {
	args := map[string]any{"path": 42}
	result := stringArg(args, "path")
	if result != "(missing argument)" {
		t.Errorf("expected '(missing argument)' for non-string, got %q", result)
	}
}
