package config

import "os"

type Config struct {
	Addr        string
	OpenAIKey   string
	OpenAIModel string
	WorldDate   string
}

func Load() Config {
	return Config{
		Addr:        env("ADDR", ":8080"),
		OpenAIKey:   os.Getenv("OPENAI_API_KEY"),
		OpenAIModel: env("OPENAI_MODEL", "gpt-5.2"),
		WorldDate:   env("WORLD_DATE", "1996-08-26"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
