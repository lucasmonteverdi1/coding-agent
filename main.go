package main

import (
	"github.com/joho/godotenv"
	"github.com/lucasmonteverdi1/coding-agent/internal/agent"
	"github.com/lucasmonteverdi1/coding-agent/internal/llm"
	"github.com/lucasmonteverdi1/coding-agent/internal/tools"
)

func main() {
	godotenv.Load()

	client := llm.NewClient()

	registry := tools.NewRegistry()
	registry.Register(tools.ReadFileTool())
	registry.Register(tools.WriteFileTool())
	registry.Register(tools.RunCommandTool())
	registry.Register(tools.ListFilesTool())
	registry.Register(tools.WebSearchTool())

	agent.RunREPL(agent.New(client, registry))
}
