package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/lucasmonteverdi1/coding-agent/internal/agent"
	"github.com/lucasmonteverdi1/coding-agent/internal/guardrails"
	"github.com/lucasmonteverdi1/coding-agent/internal/llm"
	"github.com/lucasmonteverdi1/coding-agent/internal/tools"
)

func main() {
	godotenv.Load()

	policy, err := guardrails.LoadPolicy("agent.config.json")
	if err != nil {
		log.Fatal(err)
	}

	client := llm.NewClient()

	registry := tools.NewRegistry()
	registry.Register(tools.ReadFileTool())
	registry.Register(tools.WriteFileTool())
	registry.Register(tools.RunCommandTool())
	registry.Register(tools.ListFilesTool())
	registry.Register(tools.WebSearchTool())

	agent.RunREPL(agent.New(client, registry, policy))
}
