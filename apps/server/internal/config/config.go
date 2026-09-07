package config

import (
	"os"
	"strings"
)

type Config struct {
	Addr                        string
	OpenAIKey                   string
	OpenAIModel                 string
	WorldDate                   string
	HistoricalReferencesEnabled bool
	DatabaseURL                 string
	DebugResetToken             string
	MaterializationLabToken     string
}

func Load() Config {
	return Config{
		Addr:                        env("ADDR", ":8080"),
		OpenAIKey:                   os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:                 env("OPENAI_MODEL", "gpt-5.6-luna"),
		WorldDate:                   env("WORLD_DATE", "1996-08-26"),
		HistoricalReferencesEnabled: envBool("HISTORICAL_REFERENCES_ENABLED", false),
		DatabaseURL:                 os.Getenv("DATABASE_URL"),
		DebugResetToken:             os.Getenv("DEBUG_RESET_TOKEN"),
		MaterializationLabToken:     os.Getenv("MATERIALIZATION_LAB_TOKEN"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	case "":
		return fallback
	default:
		return fallback
	}
}
