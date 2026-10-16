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
	AzureOpenAIEmbeddingModel   string
	AzureOpenAIVerbosity        string
	AzureOpenAIReasoningEffort  string
	AzureOpenAISchemaCompat     bool
	AzureOpenAIAPI              string
	JevKey                      string
	JevModel                    string
	ReactionAdvisorEnabled      bool
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
		AzureOpenAIEmbeddingModel:   env("AZURE_OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),
		AzureOpenAIVerbosity:        os.Getenv("AZURE_OPENAI_VERBOSITY"),
		AzureOpenAIReasoningEffort:  os.Getenv("AZURE_OPENAI_REASONING_EFFORT"),
		AzureOpenAISchemaCompat:     envBool("AZURE_OPENAI_SCHEMA_COMPAT", false),
		AzureOpenAIAPI:              os.Getenv("AZURE_OPENAI_API"),
		JevKey:                      jevKey,
		JevModel:                    env("JEV_MODEL", "jev-latest"),
		ReactionAdvisorEnabled:      envBool("REACTION_ADVISOR_ENABLED", false),
		WorldDate:                   strings.TrimSpace(os.Getenv("WORLD_DATE")),
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
