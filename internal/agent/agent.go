package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lucasmonteverdi1/coding-agent/internal/llm"
	"github.com/lucasmonteverdi1/coding-agent/internal/tools"
	"github.com/openai/openai-go"
)

const maxIterations = 25

type Agent struct {
	client   *llm.Client
	registry tools.Registry
}

func New(client *llm.Client, registry tools.Registry) *Agent {
	return &Agent{
		client:   client,
		registry: registry,
	}
}

func (agent *Agent) Run(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (string, error) {
	toolParams := buildToolParams(agent.registry)

	// INNER LOOP
	for i := 0; i < maxIterations; i++ {
		fmt.Printf("[iter %d] calling LLM\n", i+1)

		resp, err := agent.client.Send(ctx, messages, toolParams)
		if err != nil {
			return "", fmt.Errorf("LLM: %w", err)
		}

		choice := resp.Choices[0]

		// No tool calls — LLM is done, return its text
		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		// Append assistant's message with tool calls.
		messages = append(messages, choice.Message.ToParam())

		// Execute call tool and append results
		for _, tc := range choice.Message.ToolCalls {
			result := agent.executeTool(tc)
			messages = append(messages, openai.ToolMessage(result, tc.ID))
			fmt.Printf("[iter %d] tool=%s result=%q\n", i+1, tc.Function.Name, truncate(result, 80))
		}
	}
	return "", fmt.Errorf("reached max iterations (%d)", maxIterations)
}

func (agent *Agent) executeTool(tc openai.ChatCompletionMessageToolCall) string {
	tool, ok := agent.registry[tc.Function.Name]
	if !ok {
		return fmt.Sprintf("error: unknown tool %q", tc.Function.Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("error: could not parse args: %v", err)
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
