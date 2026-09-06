package config

import "os"

type Config struct {
	Addr                    string
	OpenAIKey               string
	OpenAIModel             string
	WorldDate               string
	DatabaseURL             string
	DebugResetToken         string
	MaterializationLabToken string
}

func Load() Config {
	return Config{
		Addr:                    env("ADDR", ":8080"),
		OpenAIKey:               os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:             env("OPENAI_MODEL", "gpt-5.6-luna"),
		WorldDate:               env("WORLD_DATE", "1996-08-26"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		DebugResetToken:         os.Getenv("DEBUG_RESET_TOKEN"),
		MaterializationLabToken: os.Getenv("MATERIALIZATION_LAB_TOKEN"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" { return v }
	return fallback
}
