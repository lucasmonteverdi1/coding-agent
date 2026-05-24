# Architecture

A coding agent built from scratch in Go without frameworks. A harness that connects an LLM to real tools via two nested loops, with a layered interception stack between the LLM's requests and their execution.

## Package layout

```
coding-agent/
├── main.go                        — entry point: flags, wiring, starts the REPL
│
└── internal/
    ├── llm/
    │   └── client.go              — thin wrapper over the OpenAI SDK
    │
    ├── models/
    │   └── models.go              — set of valid OpenAI model names (O(1) lookup)
    │
    ├── tools/
    │   ├── registry.go            — Tool struct + Registry type (map[string]Tool)
    │   ├── read_file.go
    │   ├── write_file.go
    │   ├── list_files.go
    │   ├── run_command.go         — os/exec + context.WithTimeout
    │   └── web_search.go          — Tavily API
    │
    ├── guardrails/
    │   └── guardrails.go          — policy loaded from agent.config.json
    │
    └── agent/
        ├── agent.go               — Agent struct + inner loop (Run)
        ├── repl.go                — outer loop (REPL), slash commands, rendering
        ├── interceptor.go         — plan mode + supervision confirmation
        ├── events.go              — AgentEvent types + AgentEventHandler
        ├── helpers.go             — parsing and formatting utilities
        └── prompt.go              — system prompt constant
```

## The three core contracts

```go
// A tool: self-contained unit of capability
type Tool struct {
    Name        string
    Description string
    Schema      json.RawMessage          // JSON Schema sent to the LLM
    Handler     func(map[string]any) (string, error)
}

// The registry: name → tool
type Registry map[string]Tool

// The event system: decouples the loop from the UI
type AgentEventHandler func(AgentEvent)
```

## Data flow

```
User types input
       │
       ▼
┌─────────────────────────────────────────┐
│  OUTER LOOP — repl.go                   │
│  • maintains message history            │
│  • handles slash commands               │
│  • renders events via AgentEventHandler │
└──────────────────┬──────────────────────┘
                   │ ag.Run(ctx, history, onEvent)
                   ▼
┌─────────────────────────────────────────┐
│  INNER LOOP — agent.go                  │
│  • injects system prompt                │
│  • calls llm.Client.Send()              │
│  • if LLM responds with text → return   │
│  • if LLM requests tools → execute them │
│  • repeats up to maxIterations          │
└──────────────────┬──────────────────────┘
                   │ for each tool call:
                   ▼
┌─────────────────────────────────────────┐
│  INTERCEPTION STACK                     │
│                                         │
│  1. Supervision (optional, user toggle) │
│     write_file / run_command → confirm  │
│                                         │
│  2. Guardrails (always active)          │
│     • Deny  → block, return error msg   │
│     • RequireApproval → confirm         │
│     • Allow → proceed                   │
└──────────────────┬──────────────────────┘
                   │
                   ▼
            tool.Handler(args)
                   │
                   ▼
         result → SanitizeOutput
                   │
                   ▼
      appended to messages as tool_result
                   │
                   └──────────► (repeat inner loop)
```

## Plan mode

When enabled, `Run` is called twice per user turn:

1. **Planning call** — a separate `Run` invocation with a planning system prompt. The LLM explores the project (via `list_files` / `read_file`) and returns a numbered plan. Plan mode is temporarily disabled for this call to prevent infinite recursion.
2. **User approval** — the plan is shown; the user types `y`, `n`, or a revised instruction.
3. **Execution call** — the normal `Run` proceeds with the original (or revised) message.

## Guardrails

Loaded at startup from `agent.config.json`. Three decision types:

| Decision | Effect |
|---|---|
| `Allow` | Tool executes immediately |
| `Deny` | Blocked; LLM receives an error string as the tool result |
| `RequireApproval` | User prompted; if supervision is already on, skips the second prompt |

Path checks use `filepath.Abs` + `filepath.Rel` to detect workspace escapes. Command checks use case-insensitive substring matching against denied and approval lists.

## Event system

The inner loop never calls `fmt.Print` directly. Instead it emits `AgentEvent` values via an `AgentEventHandler` callback. `repl.go` provides `renderAgentEvent` as the handler, which owns all terminal output. This makes the loop independently testable and the UI fully swappable.

## Key design decisions

**Errors as tool results:** when a tool fails, the error string is returned to the LLM as the tool result rather than crashing the loop. The LLM can read the error and self-correct.

**History ownership:** the REPL owns the outer history slice and appends only user messages and final assistant replies. Tool-call round trips (intermediate assistant messages + tool results) live inside `Run`'s local copy and are discarded after each turn. This keeps the conversation context clean across turns.

**Single stdin reader:** `bufio.NewReader(os.Stdin)` is created once in `New` and stored on the `Agent` struct. The REPL uses its own `bufio.Scanner` for the main prompt loop. The agent's reader is used only by the interceptor (plan approval, supervision confirmation), avoiding conflicts over the shared stdin stream.
