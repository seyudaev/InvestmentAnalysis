package ai

import (
	"context"
	"fmt"
)

type OpenAI struct {
	apiKey string
	model  string
}

func NewOpenAI(apiKey, model string) *OpenAI {
	return &OpenAI{apiKey: apiKey, model: model}
}

func (o *OpenAI) AnalyzeRebalance(ctx context.Context, data string) (string, error) {
	return o.chat(ctx, rebalanceSystemPrompt, data)
}

func (o *OpenAI) AnswerQuestion(ctx context.Context, portfolioData, question string) (string, error) {
	userMsg := fmt.Sprintf("Данные портфеля:\n%s\n\nВопрос: %s", portfolioData, question)
	return o.chat(ctx, questionSystemPrompt, userMsg)
}

func (o *OpenAI) chat(ctx context.Context, system, user string) (string, error) {
	body := map[string]any{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.3,
	}

	respBody, err := doChatRequest(ctx, "https://api.openai.com/v1/chat/completions",
		map[string]string{"Authorization": "Bearer " + o.apiKey}, body)
	if err != nil {
		return "", err
	}

	content, err := extractOpenAIContent(respBody)
	if err != nil {
		return "", err
	}
	return content + disclaimer, nil
}
