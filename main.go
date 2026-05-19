package main

import (
	"context"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/lucasmonteverdi1/coding-agent/internal/llm"
	"github.com/openai/openai-go"
)

func main() {
	godotenv.Load()

	client := llm.NewClient()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Say hello in one sentence"),
	}

	resp, err := client.Send(context.Background(), messages, nil)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Choices[0].Message.Content)
}
