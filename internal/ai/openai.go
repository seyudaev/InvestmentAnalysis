package ai

import (
	"context"
	"fmt"
)

type OpenAICompatible struct {
	apiKey  string
	model   string
	baseURL string
	headers map[string]string
}

func NewOpenAI(apiKey, model string) *OpenAICompatible {
	return &OpenAICompatible{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://api.openai.com/v1",
	}
}

func NewOpenRouter(apiKey, model string) *OpenAICompatible {
	if model == "" {
		model = "openai/gpt-4o"
	}
	return &OpenAICompatible{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://openrouter.ai/api/v1",
		headers: map[string]string{
			"HTTP-Referer": "https://github.com/seyudaev/InvestmentAnalysis",
			"X-Title":      "Investment Analysis Bot",
		},
	}
}

func (o *OpenAICompatible) AnalyzeRebalance(ctx context.Context, data string) (string, error) {
	return o.chat(ctx, rebalanceSystemPrompt, data)
}

func (o *OpenAICompatible) AnswerQuestion(ctx context.Context, portfolioData, question string) (string, error) {
	userMsg := fmt.Sprintf("Данные портфеля:\n%s\n\nВопрос: %s", portfolioData, question)
	return o.chat(ctx, questionSystemPrompt, userMsg)
}

func (o *OpenAICompatible) AnalyzeWatchlist(ctx context.Context, tickers []string, question string) (string, error) {
	return o.chat(ctx, watchlistSystemPrompt, FormatWatchlistPrompt(tickers, question))
}

func (o *OpenAICompatible) chat(ctx context.Context, system, user string) (string, error) {
	body := map[string]any{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.3,
	}

	headers := map[string]string{
		"Authorization": "Bearer " + o.apiKey,
	}
	for k, v := range o.headers {
		headers[k] = v
	}

	respBody, err := doChatRequest(ctx, o.baseURL+"/chat/completions", headers, body)
	if err != nil {
		return "", err
	}

	content, err := extractOpenAIContent(respBody)
	if err != nil {
		return "", err
	}
	return content + disclaimer, nil
}
