package config

import (
	"os"
	"strings"
)

type Config struct {
	Addr                        string
	AzureOpenAIEndpoint         string
	AzureOpenAIKey              string
	AzureOpenAIModel            string
	AzureOpenAIVerbosity        string
	AzureOpenAIReasoningEffort  string
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
		AzureOpenAIVerbosity:        os.Getenv("AZURE_OPENAI_VERBOSITY"),
		AzureOpenAIReasoningEffort:  os.Getenv("AZURE_OPENAI_REASONING_EFFORT"),
		JevKey:                      jevKey,
		JevModel:                    env("JEV_MODEL", "jev-latest"),
		WorldDate:                   env("WORLD_DATE", "1996-08-26"),
		HistoricalReferencesEnabled:                envBool("HISTORICAL_REFERENCES_ENABLED", false),
		DebugLogBBSArticleDetails:                  envBool("DEBUG_LOG_BBS_ARTICLE_DETAILS", false),
		DebugLogGeneratedContent:                envBool("DEBUG_LOG_GENERATED_CONTENT", true),
		DebugGenerationTrace:                    envBool("DEBUG_GENERATION_TRACE", true),
		GenerationFreeformBody:                 envBool("GENERATION_FREEFORM_BODY", true),
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
