package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/lucasmonteverdi1/coding-agent/internal/telemetry"
	"github.com/openai/openai-go"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

func readLine(agent *Agent) string {
	input, _ := agent.reader.ReadString('\n')
	return strings.TrimSpace(input)
}

var supervisedTools = map[string]struct{}{
	"write_file":  {},
	"run_command": {},
}

func requiresSupervision(toolName string) bool {
	_, ok := supervisedTools[toolName]
	return ok
}

func (agent *Agent) runPlanMode(
	ctx context.Context,
	userMessage string,
	onEvent AgentEventHandler,
	stats *turnStats,
) (approved bool, revisedMessage string, err error) {
	systemPrompt := `You are a planning assistant. The user has given you a task.
	Your job is to explore the project (use list_files and read_file as needed) and produce a clear, numbered plan of steps to complete the task.
	Do not execute anything — only plan. End your response with the plan and nothing else.`

	planMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userMessage),
	}

	// Tag the nested run so its gen_ai.chat spans are distinguishable from the
	// execution pass, whose iteration indexes would otherwise collide.
	planCtx := context.WithValue(ctx, phaseKey{}, telemetry.PhasePlan)

	agent.planMode = false
	plan, err := agent.run(planCtx, planMessages, onEvent, stats)
	agent.planMode = true

	if err != nil {
		return false, "", err
	}

	emitEvent(onEvent, AgentEvent{
		Type:    EventPlanReady,
		Message: plan,
	})

	input := readLine(agent)

	switch input {
	case "y", "Y":
		return true, "", nil
	case "n", "N":
		emitEvent(onEvent, AgentEvent{
			Type:    EventPlanRejected,
			Message: "Plan rejected",
		})
		return false, "", nil
	default:
		return true, input, nil
	}
}

// askConfirmation blocks on stdin. It gets its own span so human think-time
// does not inflate tool execution latency.
func (agent *Agent) askConfirmation(ctx context.Context, toolName string, args map[string]any) bool {
	_, span := tracer().Start(ctx, "agent.human_approval",
		trace.WithAttributes(semconv.GenAIToolName(toolName)))
	defer span.End()

	fmt.Printf("\n  ⚠ Supervision: %s\n", describeToolCall(toolName, args))
	fmt.Print("  Proceed? [y/n] > ")
	input := readLine(agent)

	granted := input == "y" || input == "Y"
	span.SetAttributes(telemetry.AttrToolApprovalGranted.Bool(granted))
	return granted
}

// handleSupervision returns the rejection tool message rather than appending to
// the history, so callers can slot it into a position-indexed result set.
func (agent *Agent) handleSupervision(
	ctx context.Context,
	tc openai.ChatCompletionMessageToolCall,
	args map[string]any,
	onEvent AgentEventHandler,
	iteration int,
) (rejected bool, message openai.ChatCompletionMessageParamUnion) {
	if !agent.supervision || !requiresSupervision(tc.Function.Name) {
		return false, message
	}
	if agent.askConfirmation(ctx, tc.Function.Name, args) {
		return false, message
	}
	emitEvent(onEvent, AgentEvent{
		Type:      EventSupervisionRejected,
		Message:   fmt.Sprintf("User rejected: %s", tc.Function.Name),
		ToolName:  tc.Function.Name,
		Iteration: iteration,
	})
	return true, openai.ToolMessage("user rejected this action", tc.ID)
}
