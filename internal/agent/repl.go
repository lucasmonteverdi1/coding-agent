package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go"
)

func RunREPL(ag *Agent) {
	fmt.Println("==========================================================")
	fmt.Println("  Coding Agent")
	fmt.Printf("  Model: %s\n", ag.client.GetModel())
	fmt.Println("  Type /model <name> to switch models, /quit to exit.")
	fmt.Println("==========================================================")
	fmt.Println()
	fmt.Println("Hi! What would you like me to do?")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	var history []openai.ChatCompletionMessageParamUnion

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line == "/quit" {
			fmt.Println("Bye!")
			return
		}

		if strings.HasPrefix(line, "/model") {
			model := strings.TrimSpace(strings.TrimPrefix(line, "/model"))
			if model == "" {
				fmt.Printf("current model: %s\n", ag.client.GetModel())
				continue
			}
			if !isValidModel(model) {
				fmt.Println("invalid model. available models:")
				for m := range validModels {
					fmt.Printf("  - %s\n", m)
				}
				continue
			}
			ag.client.SetModel(model)
			fmt.Printf("switched to %s\n", model)
			continue
		}

		history = append(history, openai.UserMessage(line))

		reply, err := ag.Run(context.Background(), history, renderAgentEvent)

		if err != nil {
			fmt.Printf("error: %v\n", err)
			history = history[:len(history)-1]
			continue
		}

		history = append(history, openai.AssistantMessage(reply))
		fmt.Printf("\nAssistant:\n%s\n\n", reply)
		fmt.Println("Anything else?")
		fmt.Println()
	}
}

func renderAgentEvent(event AgentEvent) {
	switch event.Type {
	case EventThinking:
		fmt.Println("\nWorking...")
	case EventToolCall:
		fmt.Printf("  → %s\n", event.Message)
	case EventToolArgsError:
		fmt.Printf("  ✗ %s\n", event.Message)
	case EventToolDone:
		// intentionally silent, the tool call line is enough
	case EventFinalizing:
		fmt.Println("  Done. Writing answer...")
	}
}
