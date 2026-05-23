package guardrails

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DecisionType string

const (
	Allow           DecisionType = "allow"
	Deny            DecisionType = "deny"
	RequireApproval DecisionType = "require_approval"
)

type Decision struct {
	Type   DecisionType
	Reason string
}

type ToolCall struct {
	Name string
	Args map[string]any
}

type Config struct {
	WorkspaceRoot      string   `json:"workspace_root"`
	MaxToolOutputBytes int      `json:"max_tool_output_bytes"`
	DeniedPathPatterns []string `json:"denied_path_patterns"`
	DeniedCommands     []string `json:"denied_commands"`
	ApprovalCommands   []string `json:"approval_commands"`
}

type Policy struct {
	config        Config
	workspaceRoot string
}

func LoadPolicy(configPath string) (Policy, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return DefaultPolicy(), nil
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Policy{}, fmt.Errorf("guardrails: invalid config: %w", err)
	}
	root, err := filepath.Abs(config.WorkspaceRoot)
	if err != nil {
		return Policy{}, fmt.Errorf("guardrails: bad workspace_root: %w", err)
	}
	return Policy{config: config, workspaceRoot: root}, nil
}

func DefaultPolicy() Policy {
	root, _ := filepath.Abs(".")
	return Policy{
		workspaceRoot: root,
		config: Config{
			MaxToolOutputBytes: 65536,
			DeniedCommands:     []string{"rm -rf", "sudo", "shutdown", "reboot"},
		},
	}
}

func (p Policy) SanitizeOutput(result string) string {
	max := p.config.MaxToolOutputBytes
	if max > 0 && len(result) > max {
		return result[:max] + fmt.Sprintf("\n[output truncated to %d bytes]", max)
	}
	return result
}

func (p Policy) CheckToolCall(call ToolCall) Decision {
	switch call.Name {
	case "read_file", "write_file", "list_files":
		return p.checkPath(call)
	case "run_command":
		return p.checkCommand(call)
	}
	return Decision{Type: Allow}
}

func (p Policy) checkPath(call ToolCall) Decision {
	path, ok := call.Args["path"].(string)
	if !ok {
		return Decision{Type: Allow}
	}

	// Resolve to absolute path
	abs, err := filepath.Abs(path)
	if err != nil {
		return Decision{Type: Deny, Reason: "could not resolve path"}
	}

	// Must stay inside workspace
	rel, err := filepath.Rel(p.workspaceRoot, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return Decision{
			Type:   Deny,
			Reason: fmt.Sprintf("path %q is outside the workspace", path),
		}
	}

	// Check against denied patterns
	base := filepath.Base(abs)
	for _, pattern := range p.config.DeniedPathPatterns {
		matched, err := filepath.Match(pattern, base)
		if err == nil && matched {
			return Decision{
				Type:   Deny,
				Reason: fmt.Sprintf("path %q matches denied pattern %q", path, pattern),
			}
		}
	}

	return Decision{Type: Allow}
}

func (p Policy) checkCommand(call ToolCall) Decision {
	command, ok := call.Args["command"].(string)
	if !ok {
		return Decision{Type: Allow}
	}

	lower := strings.ToLower(command)

	for _, denied := range p.config.DeniedCommands {
		if strings.Contains(lower, strings.ToLower(denied)) {
			return Decision{
				Type:   Deny,
				Reason: fmt.Sprintf("command contains denied pattern %q", denied),
			}
		}
	}

	for _, approval := range p.config.ApprovalCommands {
		if strings.Contains(lower, strings.ToLower(approval)) {
			return Decision{
				Type:   RequireApproval,
				Reason: fmt.Sprintf("command contains %q which requires approval", approval),
			}
		}
	}

	return Decision{Type: Allow}
}
