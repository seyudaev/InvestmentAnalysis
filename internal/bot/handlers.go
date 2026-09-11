package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/seyud/investment-analysis/internal/ai"
	"github.com/seyud/investment-analysis/internal/analytics"
	"github.com/seyud/investment-analysis/internal/broker/tinkoff"
	"github.com/seyud/investment-analysis/internal/storage"
	tele "gopkg.in/telebot.v4"
)

const stateAwaitingToken = "awaiting_token"

type Bot struct {
	tb       *tele.Bot
	store    *storage.Store
	crypto   *storage.Encryptor
	tinkoff  *tinkoff.Client
	ai       ai.Provider
	hasAI    bool
}

func New(
	tb *tele.Bot,
	store *storage.Store,
	crypto *storage.Encryptor,
	tinkoffClient *tinkoff.Client,
	aiProvider ai.Provider,
	hasAI bool,
) *Bot {
	return &Bot{
		tb:      tb,
		store:   store,
		crypto:  crypto,
		tinkoff: tinkoffClient,
		ai:      aiProvider,
		hasAI:   hasAI,
	}
}

func (b *Bot) Register() {
	b.tb.Handle("/start", b.handleStart)
	b.tb.Handle("/help", b.handleHelp)
	b.tb.Handle("/connect", b.handleConnect)
	b.tb.Handle("/disconnect", b.handleDisconnect)
	b.tb.Handle("/portfolio", b.handlePortfolio)
	b.tb.Handle("/signals", b.handleSignals)
	b.tb.Handle("/rebalance", b.handleRebalance)
	b.tb.Handle("/settings", b.handleSettings)
	b.tb.Handle("/setallocation", b.handleSetAllocation)
	b.tb.Handle("/ask", b.handleAsk)

	b.tb.Handle(tele.OnText, b.handleText)
}

func (b *Bot) handleStart(c tele.Context) error {
	msg := `👋 *Investment Analysis Bot*

Я помогу анализировать ваш портфель T-Инвестиций и давать рекомендации по ребалансировке.

*Команды:*
/connect — подключить T-Bank API
/portfolio — текущий портфель
/signals — сигналы buy/hold/sell
/rebalance — рекомендации по ребалансировке
/settings — настройки аллокации
/setallocation — задать целевую аллокацию
/ask — задать вопрос AI
/help — справка

Начните с /connect`
	return c.Send(msg, tele.ModeMarkdown)
}

func (b *Bot) handleHelp(c tele.Context) error {
	return c.Send(`*Справка*

1. Получите API-токен T-Инвестиций (только чтение):
   Приложение T-Инвестиции → Настройки → API для инвестиций

2. /connect — введите токен

3. /setallocation share:50 bond:30 etf:10 currency:10
   Задайте целевую аллокацию (сумма = 100%)

4. /portfolio — просмотр портфеля
   /signals — торговые сигналы
   /rebalance — AI-рекомендации
   /ask Покупать ли Сбер? — вопрос AI`, tele.ModeMarkdown)
}

func (b *Bot) handleConnect(c tele.Context) error {
	user, err := b.store.GetOrCreateUser(c.Sender().ID)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	if b.store.HasBrokerToken(user.ID, "tinkoff") {
		return c.Send("T-Bank уже подключён. Используйте /disconnect для отключения.")
	}

	if err := b.store.SetUserState(user.ID, stateAwaitingToken); err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	return c.Send(`🔑 Отправьте API-токен T-Инвестиций.

⚠️ Используйте токен *только для чтения* (без права торговли).
Токен будет зашифрован и сохранён локально.

Отправьте /cancel для отмены.`, tele.ModeMarkdown)
}

func (b *Bot) handleDisconnect(c tele.Context) error {
	_, err := b.store.GetOrCreateUser(c.Sender().ID)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	return c.Send("Для отключения удалите файл базы данных или переподключите через /connect.")
}

func (b *Bot) handlePortfolio(c tele.Context) error {
	analysis, err := b.fetchAnalysis(c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	return c.Send(analytics.FormatPortfolioReport(*analysis), tele.ModeMarkdown)
}

func (b *Bot) handleSignals(c tele.Context) error {
	analysis, err := b.fetchAnalysis(c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	return c.Send(analytics.FormatSignalsReport(analysis.Signals), tele.ModeMarkdown)
}

func (b *Bot) handleRebalance(c tele.Context) error {
	analysis, err := b.fetchAnalysis(c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	// Send numeric report first
	report := analytics.FormatPortfolioReport(*analysis)
	signals := analytics.FormatSignalsReport(analysis.Signals)
	if err := c.Send(report+"\n"+signals, tele.ModeMarkdown); err != nil {
		return err
	}

	if !b.hasAI {
		return c.Send("AI-провайдер не настроен. Добавьте API-ключ в .env для AI-рекомендаций.")
	}

	if err := c.Send("🤖 Анализирую портфель..."); err != nil {
		return err
	}

	data := analytics.FormatRebalanceData(*analysis)
	aiResp, err := b.ai.AnalyzeRebalance(context.Background(), data)
	if err != nil {
		return c.Send("Ошибка AI: " + err.Error())
	}

	return b.sendLongMessage(c, "📋 *Рекомендации AI:*\n\n"+aiResp)
}

func (b *Bot) handleSettings(c tele.Context) error {
	user, err := b.store.GetOrCreateUser(c.Sender().ID)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	allocs, err := b.store.GetTargetAllocations(user.ID)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	if len(allocs) == 0 {
		return c.Send(`*Целевая аллокация не задана*

Используйте:
/setallocation share:50 bond:30 etf:10 currency:10

Типы: share, bond, etf, currency`, tele.ModeMarkdown)
	}

	var sb strings.Builder
	sb.WriteString("*Текущая целевая аллокация:*\n")
	total := 0.0
	for _, a := range allocs {
		label := analytics.AssetType(a.AssetType)
		fmt.Fprintf(&sb, "• %s: %.1f%%\n", assetTypeRu(label), a.TargetPct)
		total += a.TargetPct
	}
	fmt.Fprintf(&sb, "\nСумма: %.1f%%", total)

	return c.Send(sb.String(), tele.ModeMarkdown)
}

func (b *Bot) handleSetAllocation(c tele.Context) error {
	user, err := b.store.GetOrCreateUser(c.Sender().ID)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	args := strings.TrimSpace(c.Message().Payload)
	if args == "" {
		return c.Send("Использование: /setallocation share:50 bond:30 etf:10 currency:10")
	}

	pairs := strings.Fields(args)
	total := 0.0
	parsed := make(map[string]float64)

	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return c.Send("Неверный формат: " + pair + "\nПример: share:50 bond:30")
		}
		pct, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || pct < 0 || pct > 100 {
			return c.Send("Неверное значение процента: " + parts[1])
		}
		parsed[parts[0]] = pct
		total += pct
	}

	if total < 99.9 || total > 100.1 {
		return c.Send(fmt.Sprintf("Сумма должна быть 100%%, сейчас: %.1f%%", total))
	}

	if err := b.store.DeleteTargetAllocations(user.ID); err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	for assetType, pct := range parsed {
		if err := b.store.SaveTargetAllocation(user.ID, assetType, pct); err != nil {
			return c.Send("Ошибка сохранения: " + err.Error())
		}
	}

	return c.Send("✅ Целевая аллокация сохранена. Проверьте: /settings")
}

func (b *Bot) handleAsk(c tele.Context) error {
	if !b.hasAI {
		return c.Send("AI-провайдер не настроен. Добавьте API-ключ в .env")
	}

	question := strings.TrimSpace(c.Message().Payload)
	if question == "" {
		return c.Send("Использование: /ask Стоит ли докупать Сбер?")
	}

	analysis, err := b.fetchAnalysis(c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	if err := c.Send("🤖 Думаю..."); err != nil {
		return err
	}

	data := analytics.FormatRebalanceData(*analysis)
	answer, err := b.ai.AnswerQuestion(context.Background(), data, question)
	if err != nil {
		return c.Send("Ошибка AI: " + err.Error())
	}

	return b.sendLongMessage(c, answer)
}

func (b *Bot) handleText(c tele.Context) error {
	text := strings.TrimSpace(c.Text())
	if text == "/cancel" {
		user, err := b.store.GetOrCreateUser(c.Sender().ID)
		if err != nil {
			return nil
		}
		_ = b.store.ClearUserState(user.ID)
		return c.Send("Отменено.")
	}

	user, err := b.store.GetOrCreateUser(c.Sender().ID)
	if err != nil {
		return nil
	}

	state, err := b.store.GetUserState(user.ID)
	if err != nil || state != stateAwaitingToken {
		return nil
	}

	token := strings.TrimSpace(text)
	if len(token) < 10 {
		return c.Send("Токен слишком короткий. Попробуйте ещё раз или /cancel")
	}

	// Validate token by fetching accounts
	accounts, err := b.tinkoff.GetAccounts(context.Background(), token)
	if err != nil {
		return c.Send("❌ Не удалось проверить токен: " + err.Error() + "\n\nПопробуйте ещё раз или /cancel")
	}

	tokenEnc, err := b.crypto.Encrypt(token)
	if err != nil {
		return c.Send("Ошибка шифрования: " + err.Error())
	}

	accountID := ""
	if len(accounts) > 0 {
		accountID = accounts[0].ID
	}

	if err := b.store.SaveBrokerToken(user.ID, "tinkoff", tokenEnc, accountID); err != nil {
		return c.Send("Ошибка сохранения: " + err.Error())
	}

	_ = b.store.ClearUserState(user.ID)

	accInfo := ""
	if len(accounts) > 0 {
		accInfo = fmt.Sprintf("\n\nСчёт: %s (%s)", accounts[0].Name, accounts[0].Type)
	}

	return c.Send("✅ T-Bank подключён!" + accInfo + "\n\nТеперь: /setallocation share:50 bond:30 etf:10 currency:10")
}

func (b *Bot) fetchAnalysis(c tele.Context) (*analytics.PortfolioAnalysis, error) {
	user, err := b.store.GetOrCreateUser(c.Sender().ID)
	if err != nil {
		return nil, fmt.Errorf("ошибка пользователя: %w", err)
	}

	bt, err := b.store.GetBrokerToken(user.ID, "tinkoff")
	if err != nil {
		return nil, fmt.Errorf("брокер не подключён. Используйте /connect")
	}

	token, err := b.crypto.Decrypt(bt.TokenEnc)
	if err != nil {
		return nil, fmt.Errorf("ошибка расшифровки токена")
	}

	snapshot, err := b.tinkoff.GetPortfolio(context.Background(), token, bt.AccountID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения портфеля: %w", err)
	}

	targets := b.loadTargets(user.ID)
	analysis := analytics.AnalyzePortfolio(*snapshot, targets)
	return &analysis, nil
}

func (b *Bot) loadTargets(userID int64) map[analytics.AssetType]float64 {
	allocs, err := b.store.GetTargetAllocations(userID)
	if err != nil || len(allocs) == 0 {
		return analytics.DefaultTargets()
	}

	targets := make(map[analytics.AssetType]float64)
	for _, a := range allocs {
		targets[analytics.AssetType(a.AssetType)] = a.TargetPct
	}
	return targets
}

func (b *Bot) sendLongMessage(c tele.Context, text string) error {
	const maxLen = 4000
	if len(text) <= maxLen {
		return c.Send(text, tele.ModeMarkdown)
	}

	parts := splitMessage(text, maxLen)
	for _, part := range parts {
		if err := c.Send(part, tele.ModeMarkdown); err != nil {
			return err
		}
	}
	return nil
}

func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var parts []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			parts = append(parts, text)
			break
		}
		cut := maxLen
		if idx := strings.LastIndex(text[:cut], "\n"); idx > maxLen/2 {
			cut = idx
		}
		parts = append(parts, text[:cut])
		text = text[cut:]
	}
	return parts
}

func assetTypeRu(at analytics.AssetType) string {
	switch at {
	case analytics.AssetShare:
		return "Акции"
	case analytics.AssetBond:
		return "Облигации"
	case analytics.AssetETF:
		return "ETF/Фонды"
	case analytics.AssetCurrency:
		return "Валюта"
	default:
		return string(at)
	}
}

func (b *Bot) Start() {
	log.Println("Bot started")
	b.tb.Start()
}
