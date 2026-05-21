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
	fmt.Println("Coding Agent initiated")
	fmt.Printf("Current model: %s\n", ag.client.GetModel())
	fmt.Println("Commands: /model <name>  /quit")
	fmt.Println("---")

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
			if _, ok := validModels[model]; !ok {
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

		reply, err := ag.Run(context.Background(), history)

		// Error rollback: if ag.Run fails, the user message that was just appended is removed
		if err != nil {
			fmt.Printf("error: %v\n", err)
			history = history[:len(history)-1]
			continue
		}

		history = append(history, openai.AssistantMessage(reply))
		fmt.Printf("\n%s\n\n", reply)
	}
}
