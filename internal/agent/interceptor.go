package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
)

var errPlanRejected = fmt.Errorf("plan rejected")

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
) (approved bool, revisedMessage string, err error) {
	systemPrompt := `You are a planning assistant. The user has given you a task.
	Your job is to explore the project (use list_files and read_file as needed) and produce a clear, numbered plan of steps to complete the task.
	Do not execute anything — only plan. End your response with the plan and nothing else.`

	planMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userMessage),
	}

	agent.planMode = false
	plan, err := agent.Run(ctx, planMessages, onEvent)
	agent.planMode = true

	if err != nil {
		return false, "", err
	}

	emitEvent(onEvent, AgentEvent{
		Type:    EventPlanReady,
		Message: plan,
	})

	agent.scanner.Scan()
	input := strings.TrimSpace(agent.scanner.Text())

	switch input {
	case "y", "Y":
		return true, "", nil
	case "n", "N":
		emitEvent(onEvent, AgentEvent{
			Type:    EventPlanRejected,
			Message: "Plan rejected",
		})
		return false, "", errPlanRejected
	default:
		return true, input, nil
	}
}

func (agent *Agent) askConfirmation(toolName string, args map[string]any) bool {
	emitEvent(nil, AgentEvent{
		Type:    EventSupervisionPrompt,
		Message: describeToolCall(toolName, args),
	})
	agent.scanner.Scan()
	input := strings.TrimSpace(agent.scanner.Text())
	return input == "y" || input == "Y"
}

func (agent *Agent) handleSupervision(
	tc openai.ChatCompletionMessageToolCall,
	args map[string]any,
	messages []openai.ChatCompletionMessageParamUnion,
	onEvent AgentEventHandler,
	iteration int,
) (rejected bool, updatedMessages []openai.ChatCompletionMessageParamUnion) {
	if !agent.supervision || !requiresSupervision(tc.Function.Name) {
		return false, messages
	}
	if agent.askConfirmation(tc.Function.Name, args) {
		return false, messages
	}
	emitEvent(onEvent, AgentEvent{
		Type:      EventSupervisionRejected,
		Message:   fmt.Sprintf("User rejected: %s", tc.Function.Name),
		ToolName:  tc.Function.Name,
		Iteration: iteration,
	})
	messages = append(messages, openai.ToolMessage("user rejected this action", tc.ID))
	return true, messages
}
