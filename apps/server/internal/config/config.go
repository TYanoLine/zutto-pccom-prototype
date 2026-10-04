package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	Addr                        string
	AzureOpenAIEndpoint         string
	AzureOpenAIKey              string
	AzureOpenAIModel            string
	JevKey                      string
	JevModel                    string
	WorldDate                   string
	HistoricalReferencesEnabled                  bool
	DebugLogBBSArticleDetails                    bool
	DebugLogGeneratedContent                   bool
	DebugGenerationTrace                       bool
	GenerationFreeformBody                    bool
	DatabaseURL                                  string
	DebugResetToken             string
}

func Load() Config {
	jevKey := os.Getenv("JEV_APIKEY")
	if jevKey == "" {
		jevKey = os.Getenv("TYPESAFE_API_KEY")
	}
	return Config{
		Addr:                        env("ADDR", ":8080"),
		AzureOpenAIEndpoint:         os.Getenv("AZURE_OPENAI_ENDPOINT"),
		AzureOpenAIKey:              os.Getenv("AZURE_OPENAI_API_KEY"),
		AzureOpenAIModel:            env("AZURE_OPENAI_MODEL", "zutto-pccom-gpt-6-luna"),
		JevKey:                      jevKey,
		JevModel:                    env("JEV_MODEL", "jev-latest"),
		WorldDate:                   env("WORLD_DATE", "1996-08-26"),
		HistoricalReferencesEnabled:                envBool("HISTORICAL_REFERENCES_ENABLED", false),
		DebugLogBBSArticleDetails:                  envBool("DEBUG_LOG_BBS_ARTICLE_DETAILS", false),
		DebugLogGeneratedContent:                envBoolWithLegacy("DEBUG_LOG_GENERATED_CONTENT", "DEBUG_LOG_HAKATA_GENERATED", true),
		DebugGenerationTrace:                    envBoolWithLegacy("DEBUG_GENERATION_TRACE", "DEBUG_HAKATA_LLM_TRACE", true),
		GenerationFreeformBody:                 envBoolWithLegacy("GENERATION_FREEFORM_BODY", "HAKATA_FREEFORM_BODY", true),
		DatabaseURL:                                 os.Getenv("DATABASE_URL"),
		DebugResetToken:             os.Getenv("DEBUG_RESET_TOKEN"),
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

// envBoolWithLegacy reads name; if it is unset (or empty) it falls back to the
// deprecated legacy name, and finally to def. Using the legacy name is logged once.
func envBoolWithLegacy(name, legacy string, def bool) bool {
	if os.Getenv(name) != "" {
		return envBool(name, def)
	}
	if os.Getenv(legacy) != "" {
		log.Printf("config: %s is deprecated; use %s", legacy, name)
		return envBool(legacy, def)
	}
	return def
}
