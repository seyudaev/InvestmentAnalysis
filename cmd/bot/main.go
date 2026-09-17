package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/seyud/investment-analysis/internal/ai"
	"github.com/seyud/investment-analysis/internal/bot"
	"github.com/seyud/investment-analysis/internal/broker/tinkoff"
	"github.com/seyud/investment-analysis/internal/config"
	"github.com/seyud/investment-analysis/internal/proxy"
	"github.com/seyud/investment-analysis/internal/scheduler"
	"github.com/seyud/investment-analysis/internal/storage"
	tele "gopkg.in/telebot.v4"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	enc, err := storage.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("encryption: %v", err)
	}

	store, err := storage.NewStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	defer store.Close()

	httpClient := proxy.NewHTTPClient(cfg.ProxyURL, time.Minute)
	if cfg.ProxyURL != "" {
		log.Printf("Proxy enabled: %s", proxy.MaskProxyURL(cfg.ProxyURL))
		if err := proxy.ValidateProxy(httpClient); err != nil {
			log.Printf("Warning: %v", err)
		}
	}
	ai.SetHTTPClient(httpClient)

	tb, err := tele.NewBot(tele.Settings{
		Token:  cfg.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10},
		Client: httpClient,
	})
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}

	tinkoffClient := tinkoff.NewClient(cfg.TBankEndpoint, cfg.TBankTLSInsecure)
	if cfg.TBankTLSInsecure {
		log.Println("T-Bank TLS: InsecureSkipVerify enabled (set TBANK_TLS_INSECURE=false to enforce cert check)")
	}

	hasAI, aiProvider := initAI(cfg)

	appBot := bot.New(tb, store, enc, tinkoffClient, aiProvider, hasAI)
	appBot.Register()

	sched := scheduler.New(store, enc, tinkoffClient, tb)
	sched.Start(cfg.DigestHour)
	defer sched.Stop()

	go appBot.Start()

	log.Println("Investment Analysis Bot is running. Press Ctrl+C to stop.")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	tb.Stop()
}

func initAI(cfg *config.Config) (bool, ai.Provider) {
	providerCfg := ai.ProviderConfig{
		OpenAIKey:       cfg.OpenAIKey,
		OpenAIModel:     cfg.OpenAIModel,
		OpenRouterKey:   cfg.OpenRouterKey,
		OpenRouterModel: cfg.OpenRouterModel,
		AnthropicKey:    cfg.AnthropicKey,
		AnthropicModel:  cfg.AnthropicModel,
		YandexKey:       cfg.YandexKey,
		YandexFolderID:  cfg.YandexFolderID,
		YandexModel:     cfg.YandexModel,
	}

	hasKey := false
	switch cfg.AIProvider {
	case "openai":
		hasKey = cfg.OpenAIKey != ""
	case "openrouter":
		hasKey = cfg.OpenRouterKey != ""
	case "claude":
		hasKey = cfg.AnthropicKey != ""
	case "yandex":
		hasKey = cfg.YandexKey != "" && cfg.YandexFolderID != ""
	}

	if !hasKey {
		log.Println("AI provider not configured — /rebalance and /ask will work without AI")
		return false, nil
	}

	provider, err := ai.NewProvider(cfg.AIProvider, providerCfg)
	if err != nil {
		log.Printf("AI provider init failed: %v", err)
		return false, nil
	}

	log.Printf("AI provider: %s", cfg.AIProvider)
	return true, provider
}
