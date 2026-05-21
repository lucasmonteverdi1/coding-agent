# coding-agent
Implementation of a coding agent in Go, including a harness, conversation mode, plan mode, and supervision mode.

## Requirements
- Go 1.21+
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

The agent starts an interactive REPL. Type a task in natural language and press Enter.

## Commands

| Command | Description |
|---|---|
| `/model <name>` | Switch model (e.g. `/model gpt-4o-mini`) |
| `/quit` | Exit |

## Tools available
- `read_file` — read a file by path
- `write_file` — write or overwrite a file
- `list_files` — list contents of a directory
- `run_command` — execute a shell command (30s timeout)
- `web_search` — search the web via Tavily

## License
MIT — Lucas Monteverdi 2026
