package agent

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/lucasmonteverdi1/coding-agent/internal/models"
	"github.com/openai/openai-go"
)

// Package-level greeting and next-prompt messages to vary the interaction
var greetingMessages = []string{
	"Hi! What would you like me to do?",
	"Hello! What task should I tackle next?",
	"Hey there! What would you like me to work on?",
	"Greetings! What's the next coding task?",
}

var nextPrompts = []string{
	"What would you like me to do next?",
	"Anything else you'd like me to tackle?",
	"What should I work on next?",
	"What's next?",
}

func pickGreeting() string {
	if len(greetingMessages) == 0 {
		return "Hi! What would you like me to do?"
	}
	return greetingMessages[rand.Intn(len(greetingMessages))]
}

func pickNextPrompt() string {
	if len(nextPrompts) == 0 {
		return "Anything else?"
	}
	return nextPrompts[rand.Intn(len(nextPrompts))]
}

func RunREPL(ag *Agent) {
	scanner := bufio.NewScanner(os.Stdin)

	// Seed RNG for greeting/next prompt variation
	rand.Seed(time.Now().UnixNano())

	printBanner(ag)

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
			handleModelCommand(ag, line)
			continue
		}

		if strings.HasPrefix(line, "/plan") {
			handleToggleCommand(line, "/plan", ag.SetPlanMode, "Plan mode")
			continue
		}

		if strings.HasPrefix(line, "/supervision") {
			handleToggleCommand(line, "/supervision", ag.SetSupervision, "Supervision")
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
		fmt.Println(pickNextPrompt())
		fmt.Println()
	}
}

func printBanner(ag *Agent) {
	fmt.Println("==========================================================")
	fmt.Println("  Coding Agent")
	fmt.Printf("  Model:       %s\n", ag.client.GetModel())
	fmt.Printf("  Plan mode:   %v\n", ag.planMode)
	fmt.Printf("  Supervision: %v\n", ag.supervision)
	fmt.Println("  Commands: /model <name>  /plan on|off  /supervision on|off  /quit")
	fmt.Println("==========================================================")
	fmt.Println()
	fmt.Println(pickGreeting())
	fmt.Println()
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
		// intentionally silent
	case EventFinalizing:
		fmt.Println("  Done. Writing answer...")
	case EventPlanReady:
		fmt.Println("\n--- PLAN ---")
		fmt.Println(event.Message)
		fmt.Println("------------")
		fmt.Println("[y] approve  [n] reject  or type a revised instruction")
		fmt.Print("> ")
	case EventPlanRejected:
		fmt.Println(event.Message)
	case EventSupervisionRejected:
		fmt.Printf("  ✗ %s\n", event.Message)
	case EventGuardrailBlocked:
		fmt.Printf("  ✗ Blocked: %s\n", event.Message)
	case EventGuardrailApproval:
		fmt.Printf("  ⚠ Approval required: %s\n", event.Message)
	}

}

func handleModelCommand(ag *Agent, line string) {
	model := strings.TrimSpace(strings.TrimPrefix(line, "/model"))
	if model == "" {
		fmt.Printf("current model: %s\n", ag.client.GetModel())
		return
	}
	if !models.IsValidModel(model) {
		fmt.Println("invalid model. available models:")
		for m := range models.ValidModels {
			fmt.Printf("  - %s\n", m)
		}
		return
	}
	ag.client.SetModel(model)
	fmt.Printf("switched to %s\n", model)
}

func handleToggleCommand(line, prefix string, setter func(bool), label string) {
	val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	switch val {
	case "on":
		setter(true)
		fmt.Printf("%s enabled\n", label)
	case "off":
		setter(false)
		fmt.Printf("%s disabled\n", label)
	default:
		fmt.Printf("usage: %s on|off\n", prefix)
	}
}
