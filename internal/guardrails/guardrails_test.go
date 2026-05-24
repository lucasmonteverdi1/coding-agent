package guardrails

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPolicy(t *testing.T) Policy {
	t.Helper()
	root, _ := filepath.Abs(".")
	return Policy{
		workspaceRoot: root,
		config: Config{
			MaxToolOutputBytes: 100,
			DeniedPathPatterns: []string{".env", ".env.*", "*.key", "id_rsa"},
			DeniedCommands:     []string{"rm -rf", "sudo", "shutdown"},
			ApprovalCommands:   []string{"git push", "npm install"},
		},
	}
}

// --- checkPath ---

func TestCheckPath_AllowsFileInsideWorkspace(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "read_file",
		Args: map[string]any{"path": "README.md"},
	})
	if decision.Type != Allow {
		t.Errorf("expected Allow, got %s: %s", decision.Type, decision.Reason)
	}
}

func TestCheckPath_DeniesPathOutsideWorkspace(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "read_file",
		Args: map[string]any{"path": "../../../etc/passwd"},
	})
	if decision.Type != Deny {
		t.Errorf("expected Deny, got %s", decision.Type)
	}
}

func TestCheckPath_DeniesEnvFile(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "read_file",
		Args: map[string]any{"path": ".env"},
	})
	if decision.Type != Deny {
		t.Errorf("expected Deny, got %s", decision.Type)
	}
}

func TestCheckPath_DeniesEnvWildcard(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "read_file",
		Args: map[string]any{"path": ".env.production"},
	})
	if decision.Type != Deny {
		t.Errorf("expected Deny, got %s", decision.Type)
	}
}

func TestCheckPath_DeniesKeyFile(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "read_file",
		Args: map[string]any{"path": "secret.key"},
	})
	if decision.Type != Deny {
		t.Errorf("expected Deny, got %s", decision.Type)
	}
}

func TestCheckPath_AllowsNestedFileInsideWorkspace(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "read_file",
		Args: map[string]any{"path": "internal/agent/agent.go"},
	})
	if decision.Type != Allow {
		t.Errorf("expected Allow, got %s: %s", decision.Type, decision.Reason)
	}
}

// --- checkCommand ---

func TestCheckCommand_AllowsSafeCommand(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "run_command",
		Args: map[string]any{"command": "go build ./..."},
	})
	if decision.Type != Allow {
		t.Errorf("expected Allow, got %s: %s", decision.Type, decision.Reason)
	}
}

func TestCheckCommand_DeniesRmRf(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "run_command",
		Args: map[string]any{"command": "rm -rf ./tmp"},
	})
	if decision.Type != Deny {
		t.Errorf("expected Deny, got %s", decision.Type)
	}
}

func TestCheckCommand_DeniesSudo(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "run_command",
		Args: map[string]any{"command": "sudo apt-get install curl"},
	})
	if decision.Type != Deny {
		t.Errorf("expected Deny, got %s", decision.Type)
	}
}

func TestCheckCommand_IsCaseInsensitive(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "run_command",
		Args: map[string]any{"command": "RM -RF ./tmp"},
	})
	if decision.Type != Deny {
		t.Errorf("expected Deny for uppercase denied command, got %s", decision.Type)
	}
}

func TestCheckCommand_RequiresApprovalForGitPush(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "run_command",
		Args: map[string]any{"command": "git push origin main"},
	})
	if decision.Type != RequireApproval {
		t.Errorf("expected RequireApproval, got %s", decision.Type)
	}
}

func TestCheckCommand_RequiresApprovalForNpmInstall(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "run_command",
		Args: map[string]any{"command": "npm install express"},
	})
	if decision.Type != RequireApproval {
		t.Errorf("expected RequireApproval, got %s", decision.Type)
	}
}

// --- SanitizeOutput ---

func TestSanitizeOutput_PassesThroughShortOutput(t *testing.T) {
	p := testPolicy(t)
	result := p.SanitizeOutput("hello")
	if result != "hello" {
		t.Errorf("expected %q, got %q", "hello", result)
	}
}

func TestSanitizeOutput_TruncatesLongOutput(t *testing.T) {
	p := testPolicy(t)
	long := strings.Repeat("a", 200)
	result := p.SanitizeOutput(long)
	if len(result) <= 100 {
		t.Error("expected truncated output to still have content")
	}
	if !strings.Contains(result, "[output truncated to 100 bytes]") {
		t.Errorf("expected truncation message, got: %s", result)
	}
}

func TestSanitizeOutput_NoLimitWhenZero(t *testing.T) {
	p := Policy{config: Config{MaxToolOutputBytes: 0}}
	long := strings.Repeat("a", 200)
	result := p.SanitizeOutput(long)
	if result != long {
		t.Error("expected no truncation when MaxToolOutputBytes is 0")
	}
}

// --- LoadPolicy ---

func TestLoadPolicy_FileNotFound_ReturnsDefault(t *testing.T) {
	policy, err := LoadPolicy("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("expected no error when file not found, got: %v", err)
	}
	if policy.config.MaxToolOutputBytes != 65536 {
		t.Errorf("expected default MaxToolOutputBytes=65536, got %d", policy.config.MaxToolOutputBytes)
	}
}

func TestLoadPolicy_ValidFile_LoadsConfig(t *testing.T) {
	f, err := os.CreateTemp("", "agent-config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(`{
		"workspace_root": ".",
		"max_tool_output_bytes": 1024,
		"denied_commands": ["sudo"],
		"approval_commands": ["git push"]
	}`)
	f.Close()

	policy, err := LoadPolicy(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.config.MaxToolOutputBytes != 1024 {
		t.Errorf("expected MaxToolOutputBytes=1024, got %d", policy.config.MaxToolOutputBytes)
	}
	if len(policy.config.DeniedCommands) != 1 || policy.config.DeniedCommands[0] != "sudo" {
		t.Errorf("expected DeniedCommands=[sudo], got %v", policy.config.DeniedCommands)
	}
}

func TestLoadPolicy_MalformedJSON_ReturnsError(t *testing.T) {
	f, err := os.CreateTemp("", "agent-config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(`{ this is not valid json }`)
	f.Close()

	_, err = LoadPolicy(f.Name())
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

// --- CheckToolCall unknown tool ---

func TestCheckToolCall_AllowsUnknownTool(t *testing.T) {
	p := testPolicy(t)
	decision := p.CheckToolCall(ToolCall{
		Name: "web_search",
		Args: map[string]any{"query": "go testing"},
	})
	if decision.Type != Allow {
		t.Errorf("expected Allow for web_search, got %s", decision.Type)
	}
}
