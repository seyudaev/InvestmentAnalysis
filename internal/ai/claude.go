package ai

import (
	"context"
	"fmt"
)

type Claude struct {
	apiKey string
	model  string
}

func NewClaude(apiKey, model string) *Claude {
	return &Claude{apiKey: apiKey, model: model}
}

func (c *Claude) AnalyzeRebalance(ctx context.Context, data string) (string, error) {
	return c.chat(ctx, rebalanceSystemPrompt, data)
}

func (c *Claude) AnswerQuestion(ctx context.Context, portfolioData, question string) (string, error) {
	userMsg := fmt.Sprintf("Данные портфеля:\n%s\n\nВопрос: %s", portfolioData, question)
	return c.chat(ctx, questionSystemPrompt, userMsg)
}

func (c *Claude) AnalyzeWatchlist(ctx context.Context, tickers []string, question string) (string, error) {
	return c.chat(ctx, watchlistSystemPrompt, FormatWatchlistPrompt(tickers, question))
}

func (c *Claude) chat(ctx context.Context, system, user string) (string, error) {
	body := map[string]any{
		"model":      c.model,
		"max_tokens": 2048,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": user},
		},
	}

	respBody, err := doChatRequest(ctx, "https://api.anthropic.com/v1/messages",
		map[string]string{
			"x-api-key":         c.apiKey,
			"anthropic-version": "2023-06-01",
		}, body)
	if err != nil {
		return "", err
	}

	content, err := extractClaudeContent(respBody)
	if err != nil {
		return "", err
	}
	return content + disclaimer, nil
}
