# coding-agent
Implementation of a coding agent in Go, including a harness, conversation mode, plan mode, and supervision mode.

## Requirements
- Go 1.26+
- An [OpenAI API key](https://platform.openai.com/api-keys)
- A [Tavily API key](https://tavily.com) (for web search)

## Setup

1. Clone the repo and install dependencies:
   ```bash
   git clone https://github.com/lucasmonteverdi1/coding-agent
   cd coding-agent
   go mod download
   ```

2. Copy the env template and fill in your API keys:
   ```bash
   cp .env.example .env
   ```
   ```
   OPENAI_API_KEY=sk-...
   TAVILY_API_KEY=tvly-...
   ```

## Run

```bash
go run .
```

For tasks that require reading many files, increase the iteration cap:

```bash
go run . --max-iterations 100
```

The agent starts an interactive REPL. Type a task in natural language and press Enter.

## Using outside this repo

### Download a prebuilt binary

Grab the archive for your platform from the [Releases](https://github.com/lucasmonteverdi1/coding-agent/releases) page, then:

```bash
tar -xzf coding-agent_*.tar.gz
mv coding-agent /usr/local/bin/
```

### Or build from source

```bash
go build -o coding-agent .
mv coding-agent /usr/local/bin/
```

Then launch it from any directory:

```bash
cd ~/your-project
coding-agent
```

The agent sandboxes itself to the directory it was launched from. Since `.env` files are project-local, set your API keys in your shell profile instead:

```bash
export OPENAI_API_KEY=sk-...
export TAVILY_API_KEY=tvly-...
export OPENAI_MODEL=gpt-4o  # optional, defaults to gpt-4o
```

## Commands

| Command                 | Description                              |
|-------------------------|------------------------------------------|
| `/model <name>`         | Switch model (e.g. `/model gpt-4o-mini`) |
| `/plan [on/off]`        | Plan mode on/off                         |
| `/supervision [on/off]` | Supervision mode on/off                  |
| `/quit`                 | Exit                                     |

## Features

- **Conversational REPL** — persistent message history across turns; the agent remembers context from previous messages
- **Tool loop** — autonomously chains tool calls until the task is complete, with a configurable iteration cap
- **Plan mode** — before acting, the agent explores the project and proposes a numbered plan for approval; supports approval, rejection, or inline revision
- **Supervision mode** — intercepts destructive actions (`write_file`, `run_command`) and asks for confirmation before executing
- **Guardrails** — always-on safety layer that blocks dangerous paths and commands, requires approval for risky ones (e.g. `git push`), and truncates oversized tool output
- **Model switching** — swap OpenAI models at runtime without restarting
- **Configurable** — `agent.config.json` defines blocked paths, denied commands, and approval-required commands; default model set via `OPENAI_MODEL` env var

## Tools

| Tool | Description | Requires confirmation |
|---|---|---|
| `read_file` | Read a file by path | No |
| `write_file` | Write or overwrite a file | Supervision mode |
| `list_files` | List contents of a directory | No |
| `run_command` | Execute a shell command (30s timeout) | Supervision mode |
| `web_search` | Search the web via Tavily | No |

## Sample run

### Run 1 — multi-step file edit (7 iterations)

The agent receives a task, explores the project, reads the target file, writes the change, and reads it back to verify.

```
Hi! What would you like me to do?

> add a comment to client.go explaining what it does

Working...
  → Listing .
Working...
  → Listing ./internal
Working...
  → Listing ./internal/agent
Working...
  → Reading ./internal/agent/agent.go
Working...
  → Writing ./internal/agent/agent.go
Working...
  → Reading ./internal/agent/agent.go
Working...
  Done. Writing answer...
Assistant:
I added a package-level comment at the top of agent.go explaining its purpose.
The comment describes how the package manages interactions with a language model,
including message handling, tool call orchestration, and supervision based on plans
and policies. The change has been verified successfully.
```

### Run 2 — plan mode (17 iterations)

With plan mode on, the agent reads the entire codebase before proposing a numbered
plan. The user can approve, reject, or revise it before any changes are made.

```
> /plan on
Plan mode enabled
> /model gpt-5-nano
switched to gpt-5-nano
> refactor repl.go so that there are a few different messages for the agent to greet the user

Working...
  → Listing .
Working...
  → Listing internal/agent
Working...
  → Reading internal/agent/repl.go
  → Reading internal/agent/prompt.go
  → Reading internal/agent/events.go
  → Reading internal/agent/helpers.go
  → Reading internal/agent/agent.go
Working...
  → Listing internal/llm
  → Reading internal/llm/client.go
  → Listing internal/tools
  → Reading internal/tools/registry.go
  ...
Working...
  Done. Writing answer...

--- PLAN ---
1) Locate where the greeting and the next-task prompt are printed (internal/agent/repl.go).
2) Introduce two slices at package scope: greetingMessages and nextPrompts.
3) Add math/rand import and seed the RNG at RunREPL startup.
4) Modify printBanner to pick a random greeting from greetingMessages.
5) Replace the static "Anything else?" prompt with a random entry from nextPrompts.
6) Add helper functions pickGreeting() and pickNextPrompt().
7) Build and verify with go build ./...
------------
[y] approve  [n] reject  or type a revised instruction
> y
```

## License
MIT - Lucas Monteverdi 2026
