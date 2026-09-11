package ai

import (
	"context"

	"github.com/seyud/investment-analysis/internal/analytics"
)

type Provider interface {
	AnalyzeRebalance(ctx context.Context, data string) (string, error)
	AnswerQuestion(ctx context.Context, portfolioData, question string) (string, error)
}

func NewProvider(providerType string, cfg ProviderConfig) (Provider, error) {
	switch providerType {
	case "openai":
		return NewOpenAI(cfg.OpenAIKey, cfg.OpenAIModel), nil
	case "claude":
		return NewClaude(cfg.AnthropicKey, cfg.AnthropicModel), nil
	case "yandex":
		return NewYandex(cfg.YandexKey, cfg.YandexFolderID, cfg.YandexModel), nil
	default:
		return NewOpenAI(cfg.OpenAIKey, cfg.OpenAIModel), nil
	}
}

type ProviderConfig struct {
	OpenAIKey      string
	OpenAIModel    string
	AnthropicKey   string
	AnthropicModel string
	YandexKey      string
	YandexFolderID string
	YandexModel    string
}

const disclaimer = "\n\n_⚠️ Это не является индивидуальной инвестиционной рекомендацией. Принимайте решения самостоятельно._"

const rebalanceSystemPrompt = `Ты — финансовый аналитик. На основе данных портфеля предложи план ребалансировки.
Отвечай на русском языке. Структура ответа:
1. Краткая оценка текущего состояния
2. Что продать / уменьшить
3. Что купить / увеличить
4. Риски и замечания
Будь конкретен, указывай суммы и проценты где возможно.`

const questionSystemPrompt = `Ты — финансовый аналитик. Отвечай на вопросы пользователя о его портфеле на русском языке.
Используй только предоставленные данные портфеля. Если данных недостаточно — скажи об этом.`

func FormatPortfolioContext(a analytics.PortfolioAnalysis) string {
	return analytics.FormatRebalanceData(a)
}
