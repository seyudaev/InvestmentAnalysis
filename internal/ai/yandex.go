package ai

import (
	"context"
	"fmt"
)

type Yandex struct {
	apiKey   string
	folderID string
	model    string
}

func NewYandex(apiKey, folderID, model string) *Yandex {
	return &Yandex{apiKey: apiKey, folderID: folderID, model: model}
}

func (y *Yandex) AnalyzeRebalance(ctx context.Context, data string) (string, error) {
	return y.chat(ctx, rebalanceSystemPrompt+"\n\n"+data)
}

func (y *Yandex) AnswerQuestion(ctx context.Context, portfolioData, question string) (string, error) {
	userMsg := fmt.Sprintf("%s\n\nДанные портфеля:\n%s\n\nВопрос: %s", questionSystemPrompt, portfolioData, question)
	return y.chat(ctx, userMsg)
}

func (y *Yandex) chat(ctx context.Context, text string) (string, error) {
	modelURI := fmt.Sprintf("gpt://%s/%s/latest", y.folderID, y.model)
	body := map[string]any{
		"modelUri": modelURI,
		"completionOptions": map[string]any{
			"stream":      false,
			"temperature": 0.3,
			"maxTokens":   2048,
		},
		"messages": []map[string]string{
			{"role": "user", "text": text},
		},
	}

	url := "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"
	respBody, err := doChatRequest(ctx, url,
		map[string]string{"Authorization": "Api-Key " + y.apiKey}, body)
	if err != nil {
		return "", err
	}

	content, err := extractYandexContent(respBody)
	if err != nil {
		return "", err
	}
	return content + disclaimer, nil
}
