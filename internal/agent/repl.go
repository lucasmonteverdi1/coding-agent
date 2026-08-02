package agent

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"

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

	printBanner(ag)

	var history []openai.ChatCompletionMessageParamUnion
	var session sessionUsage
	onEvent := newEventRenderer(&session)

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
			session.printTotal()
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

		reply, err := ag.Run(context.Background(), history, onEvent)

		if err != nil {
			fmt.Printf("error: %v\n", err)
			history = history[:len(history)-1]
			continue
		}

		history = append(history, openai.AssistantMessage(reply))
		fmt.Printf("%s\n%s\n\n", bold("Assistant:"), reply)
		fmt.Println(dim(pickNextPrompt()))
		fmt.Println()
	}

	// Reached on Ctrl-D; /quit returns earlier and prints its own total.
	session.printTotal()
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

// newEventRenderer returns the REPL's event handler, closing over the session
// totals so no package-level state is needed.
func newEventRenderer(session *sessionUsage) AgentEventHandler {
	return func(event AgentEvent) {
		renderAgentEvent(event, session)
	}
}

func renderAgentEvent(event AgentEvent, session *sessionUsage) {
	switch event.Type {
	case EventThinking:
		fmt.Println(dim("\nWorking..."))
	case EventToolCall:
		fmt.Println(dim("  → " + event.Message))
	case EventToolArgsError:
		fmt.Printf("  ✗ %s\n", event.Message)
	case EventToolDone:
		// intentionally silent
	case EventFinalizing:
		fmt.Println(dim("  Done. Writing answer..."))
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
	case EventTurnUsage:
		session.add(event.Usage)
	}

}

// sessionUsage accumulates across turns so the REPL can show a running total.
type sessionUsage struct {
	turns        int
	inputTokens  int64
	outputTokens int64
	costUSD      float64
	costKnown    bool
}

// add folds one turn into the session and renders both, from the OpenAI
// response metadata. Independent of telemetry: no collector involved.
func (s *sessionUsage) add(u *TurnUsage) {
	if u == nil {
		return
	}

	s.turns++
	s.inputTokens += u.InputTokens
	s.outputTokens += u.OutputTokens
	s.costUSD += u.CostUSD
	s.costKnown = s.costKnown || u.CostKnown

	cost := "cost " + formatUSD(u.CostUSD)
	if !u.CostKnown {
		// No price for this model: say so rather than implying it was free.
		cost = "cost unknown"
	}

	// Dimmed and rule-separated so usage metadata never reads as agent output.
	fmt.Println()
	fmt.Println(dim("  ── usage ─────────────────-"))
	fmt.Println(dim(fmt.Sprintf("  %s · %s · %s · %d↑ %d↓ tokens · %s",
		u.Model, plural(u.Iterations, "iteration"), plural(u.ToolCalls, "tool"),
		u.InputTokens, u.OutputTokens, cost)))

	if s.turns > 1 {
		total := formatUSD(s.costUSD)
		if !s.costKnown {
			total = "cost unknown"
		}
		fmt.Println(dim(fmt.Sprintf("  session: %d turns · %d↑ %d↓ · %s",
			s.turns, s.inputTokens, s.outputTokens, total)))
	}
	fmt.Println()
}

// printTotal reports the session's consumption on the way out.
func (s *sessionUsage) printTotal() {
	if s.turns == 0 {
		return
	}
	line := fmt.Sprintf("Session: %s · %d↑ %d↓ tokens",
		plural(s.turns, "turn"), s.inputTokens, s.outputTokens)
	if s.costKnown {
		line += " · " + formatUSD(s.costUSD)
	}
	fmt.Println(dim(line))
}

// plural renders "1 tool" but "2 tools".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// formatUSD keeps sub-cent costs readable instead of flattening them to $0.00.
func formatUSD(usd float64) string {
	if usd > 0 && usd < 0.01 {
		return fmt.Sprintf("$%.6f", usd)
	}
	return fmt.Sprintf("$%.4f", usd)
}

func handleModelCommand(ag *Agent, line string) {
	model := strings.TrimSpace(strings.TrimPrefix(line, "/model"))
	if model == "" {
		fmt.Printf("current model: %s\n", ag.client.GetModel())
		return
	}
	if !models.IsValidModel(model) {
		fmt.Println("invalid model. available models:")
		for _, m := range models.ValidModelsOrder {
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
