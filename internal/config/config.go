package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken  string
	EncryptionKey  string
	AIProvider     string
	OpenAIKey      string
	OpenAIModel    string
	AnthropicKey   string
	AnthropicModel string
	YandexKey      string
	YandexFolderID string
	YandexModel    string
	DBPath         string
	TBankEndpoint  string
	TBankTLSInsecure bool
	DigestHour     int
	ProxyURL       string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		TelegramToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		EncryptionKey:  os.Getenv("ENCRYPTION_KEY"),
		AIProvider:     envOr("AI_PROVIDER", "openai"),
		OpenAIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:    envOr("OPENAI_MODEL", "gpt-4o"),
		AnthropicKey:   os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel: envOr("ANTHROPIC_MODEL", "claude-sonnet-4-20250514"),
		YandexKey:      os.Getenv("YANDEX_API_KEY"),
		YandexFolderID: os.Getenv("YANDEX_FOLDER_ID"),
		YandexModel:    envOr("YANDEX_MODEL", "yandexgpt"),
		DBPath:           envOr("DB_PATH", "./data/investment.db"),
		TBankEndpoint:    envOr("TBANK_ENDPOINT", "invest-public-api.tinkoff.ru:443"),
		TBankTLSInsecure: envBool("TBANK_TLS_INSECURE", true),
		DigestHour:       envInt("DIGEST_HOUR", 9),
		ProxyURL:         envOr("PROXY_URL", os.Getenv("ALL_PROXY")),
	}

	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}
