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
	AzureOpenAIImageModel       string
	JevKey                      string
	JevModel                    string
	WorldDate                   string
	HistoricalReferencesEnabled                  bool
	DebugDisableBBSTitleHistoricalVerification   bool
	DebugLogBBSArticleDetails                    bool
	DebugAutoRunMaterializationAudit             bool
	DatabaseURL                                  string
	DebugResetToken             string
	MaterializationLabToken     string
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
		AzureOpenAIImageModel:       env("AZURE_OPENAI_IMAGE_MODEL", "gpt-image-1"),
		JevKey:                      jevKey,
		JevModel:                    env("JEV_MODEL", "jev-latest"),
		WorldDate:                   env("WORLD_DATE", "1996-08-26"),
		HistoricalReferencesEnabled:                envBool("HISTORICAL_REFERENCES_ENABLED", false),
		DebugDisableBBSTitleHistoricalVerification: envBool("DEBUG_DISABLE_BBS_TITLE_HISTORICAL_VERIFICATION", false),
		DebugLogBBSArticleDetails:                  envBool("DEBUG_LOG_BBS_ARTICLE_DETAILS", false),
		DebugAutoRunMaterializationAudit:           envBool("DEBUG_AUTORUN_MATERIALIZATION_AUDIT", false),
		DatabaseURL:                                 os.Getenv("DATABASE_URL"),
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
