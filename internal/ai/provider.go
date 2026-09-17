package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/seyud/investment-analysis/internal/analytics"
)

type Provider interface {
	AnalyzeRebalance(ctx context.Context, data string) (string, error)
	AnswerQuestion(ctx context.Context, portfolioData, question string) (string, error)
	AnalyzeWatchlist(ctx context.Context, tickers []string, question string) (string, error)
}

func NewProvider(providerType string, cfg ProviderConfig) (Provider, error) {
	switch providerType {
	case "openai":
		return NewOpenAI(cfg.OpenAIKey, cfg.OpenAIModel), nil
	case "openrouter":
		return NewOpenRouter(cfg.OpenRouterKey, cfg.OpenRouterModel), nil
	case "claude":
		return NewClaude(cfg.AnthropicKey, cfg.AnthropicModel), nil
	case "yandex":
		return NewYandex(cfg.YandexKey, cfg.YandexFolderID, cfg.YandexModel), nil
	default:
		return nil, fmt.Errorf("unknown AI provider: %s (use openai, openrouter, claude, yandex)", providerType)
	}
}

type ProviderConfig struct {
	OpenAIKey        string
	OpenAIModel      string
	OpenRouterKey    string
	OpenRouterModel  string
	AnthropicKey     string
	AnthropicModel   string
	YandexKey        string
	YandexFolderID   string
	YandexModel      string
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

const watchlistSystemPrompt = `Ты — финансовый аналитик по российскому и международному рынку.
Пользователь задал список тикеров компаний (watchlist) без данных брокерского портфеля.
Дай практичные советы на русском языке:
1. Краткий обзор каждой компании (сектор, чем известна)
2. По каждой: сигнал buy / hold / sell / wait и краткое обоснование
3. Риски и на что смотреть дальше
4. Если данных мало или они устарели — честно укажи это
Не выдумывай точные котировки как факт; опирайся на общедоступные знания и логику анализа.
Это не индивидуальная инвестиционная рекомендация.`

func FormatWatchlistPrompt(tickers []string, question string) string {
	var b strings.Builder
	b.WriteString("Список компаний (тикеры):\n")
	for i, t := range tickers {
		fmt.Fprintf(&b, "%d. %s\n", i+1, t)
	}
	if strings.TrimSpace(question) != "" {
		fmt.Fprintf(&b, "\nДополнительный вопрос пользователя:\n%s\n", question)
	} else {
		b.WriteString("\nДай общий обзор и торговые/инвестиционные ориентиры по этому списку.\n")
	}
	return b.String()
}

func FormatPortfolioContext(a analytics.PortfolioAnalysis) string {
	return analytics.FormatRebalanceData(a)
}

