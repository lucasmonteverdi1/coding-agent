package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func WebSearchTool() Tool {
	return Tool{
		Name:        "web_search",
		Description: "Search the web using Tavily and return relevant results.",
		Schema: []byte(`{
			"type": "object",
			"properties": {
				"query": { "type": "string", "description": "The search query" }
			},
			"required": ["query"]
		}`),
		Handler: func(args map[string]any) (string, error) {
			query, ok := args["query"].(string)
			if !ok {
				return "", fmt.Errorf("query must be a string")
			}

			apiKey := os.Getenv("TAVILY_API_KEY")
			if apiKey == "" {
				return "", fmt.Errorf("TAVILY_API_KEY not set")
			}

			body, _ := json.Marshal(map[string]any{
				"api_key":      apiKey,
				"query":        query,
				"search_depth": "basic",
				"max_results":  5,
			})

			resp, err := http.Post(
				"https://api.tavily.com/search",
				"application/json",
				strings.NewReader(string(body)),
			)
			if err != nil {
				return "", fmt.Errorf("web_search: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("web_search: API returned status %d", resp.StatusCode)
			}

			raw, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", fmt.Errorf("web_search reading body: %w", err)
			}

			var result struct {
				Results []struct {
					Title   string `json:"title"`
					URL     string `json:"url"`
					Content string `json:"content"`
				} `json:"results"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				return "", fmt.Errorf("web_search parsing: %w", err)
			}

			var lines []string
			for _, r := range result.Results {
				lines = append(lines, fmt.Sprintf("## %s\n%s\n%s", r.Title, r.URL, r.Content))
			}
			return strings.Join(lines, "\n\n"), nil
		},
	}
}
