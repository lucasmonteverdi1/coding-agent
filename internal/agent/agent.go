package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lucasmonteverdi1/coding-agent/internal/guardrails"
	"github.com/lucasmonteverdi1/coding-agent/internal/llm"
	"github.com/lucasmonteverdi1/coding-agent/internal/tools"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

const maxIterations = 25

type Agent struct {
	client      *llm.Client
	registry    tools.Registry
	guardrails  guardrails.Policy
	planMode    bool
	supervision bool
}

func New(client *llm.Client, registry tools.Registry, policy guardrails.Policy) *Agent {
	return &Agent{
		client:     client,
		registry:   registry,
		guardrails: policy,
	}
}

func (agent *Agent) SetPlanMode(on bool) { agent.planMode = on }

func (agent *Agent) SetSupervision(on bool) { agent.supervision = on }

func (agent *Agent) Run(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	onEvent AgentEventHandler,
) (string, error) {
	toolParams := buildToolParams(agent.registry)

	// Inject system prompt if not present
	if len(messages) == 0 || messages[0].OfSystem == nil {
		messages = append([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
		}, messages...)
	}

	// Plan mode: generate and approve a plan before executing anything
	if agent.planMode {
		userMessage := extractLastUserMessage(messages)
		approved, revised, err := agent.runPlanMode(ctx, userMessage, onEvent)
		if err != nil || !approved {
			return "Plan rejected. Send a new prompt to try again.", nil
		}
		if revised != "" {
			original := extractLastUserMessage(messages)
			messages[len(messages)-1] = openai.UserMessage(original + "\n\nRevision: " + revised)
		}
	}

	// INNER LOOP
	for i := 0; i < maxIterations; i++ {
		emitEvent(onEvent, AgentEvent{
			Type:      EventThinking,
			Message:   "Thinking...",
			Iteration: i + 1,
		})

		resp, err := agent.client.Send(ctx, messages, toolParams)
		if err != nil {
			return "", fmt.Errorf("LLM: %w", err)
		}

		choice := resp.Choices[0]

		// No tool calls — LLM is done, return its text
		if len(choice.Message.ToolCalls) == 0 {
			emitEvent(onEvent, AgentEvent{
				Type:      EventFinalizing,
				Message:   "Writing answer...",
				Iteration: i + 1,
			})
			return choice.Message.Content, nil
		}

		// Append assistant's message with tool calls.
		messages = append(messages, choice.Message.ToParam())

		// Execute call tool and append results
		for _, tc := range choice.Message.ToolCalls {
			args, err := parseToolArguments(tc.Function.Arguments)
			if err != nil {
				emitEvent(onEvent, AgentEvent{
					Type:      EventToolArgsError,
					Message:   fmt.Sprintf("Could not parse arguments for tool: %s", tc.Function.Name),
					ToolName:  tc.Function.Name,
					Iteration: i + 1,
				})
				messages = append(messages, openai.ToolMessage("error: could not parse tool arguments", tc.ID))
				continue
			} else {
				emitEvent(onEvent, AgentEvent{
					Type:      EventToolCall,
					Message:   describeToolCall(tc.Function.Name, args),
					ToolName:  tc.Function.Name,
					Iteration: i + 1,
				})
			}

			var rejected bool
			rejected, messages = agent.handleSupervision(tc, args, messages, onEvent, i+1)
			if rejected {
				continue
			}

			decision := agent.guardrails.CheckToolCall(guardrails.ToolCall{
				Name: tc.Function.Name,
				Args: args,
			})

			switch decision.Type {
			case guardrails.Deny:
				emitEvent(onEvent, AgentEvent{
					Type:      EventGuardrailBlocked,
					Message:   decision.Reason,
					ToolName:  tc.Function.Name,
					Iteration: i + 1,
				})
				messages = append(messages, openai.ToolMessage("action blocked by guardrail: "+decision.Reason, tc.ID))
				continue
			case guardrails.RequireApproval:
				emitEvent(onEvent, AgentEvent{
					Type:      EventGuardrailApproval,
					Message:   decision.Reason,
					ToolName:  tc.Function.Name,
					Iteration: i + 1,
				})
				if !agent.supervision && !agent.askConfirmation(tc.Function.Name, args) {
					messages = append(messages, openai.ToolMessage("user rejected action requiring approval", tc.ID))
					continue
				}
			}

			result := agent.executeTool(tc, args)
			result = agent.guardrails.SanitizeOutput(result)
			messages = append(messages, openai.ToolMessage(result, tc.ID))
			emitEvent(onEvent, AgentEvent{
				Type:      EventToolDone,
				Message:   summarizeToolResult(result),
				ToolName:  tc.Function.Name,
				Iteration: i + 1,
			})
		}
	}
	return "", fmt.Errorf("reached max iterations (%d)", maxIterations)
}

func (agent *Agent) executeTool(tc openai.ChatCompletionMessageToolCall, args map[string]any) string {
	tool, ok := agent.registry[tc.Function.Name]
	if !ok {
		return fmt.Sprintf("error: unknown tool %q", tc.Function.Name)
	}

	result, err := tool.Handler(args)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

func buildToolParams(registry tools.Registry) []openai.ChatCompletionToolParam {
	var params []openai.ChatCompletionToolParam
	for _, tool := range registry {
		var schema openai.FunctionParameters
		json.Unmarshal(tool.Schema, &schema)

		params = append(params, openai.ChatCompletionToolParam{
			Type: "function",
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  schema,
			},
		})
	}
	return params
}

func extractLastUserMessage(messages []openai.ChatCompletionMessageParamUnion) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if msg := messages[i].OfUser; msg != nil {
			if !param.IsOmitted(msg.Content.OfString) {
				return msg.Content.OfString.Value
			}
		}
	}
	return ""
}
