package azureopenai

import (
	"errors"
	"net/http"
	"strings"
)

func URL(endpoint, path string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if base == "" {
		return "", errors.New("AZURE_OPENAI_ENDPOINT is not set")
	}
	path = strings.TrimLeft(strings.TrimSpace(path), "/")
	if path == "" {
		return "", errors.New("Azure OpenAI API path is empty")
	}
	if strings.HasSuffix(base, "/openai/v1") {
		return base + "/" + path, nil
	}
	return base + "/openai/v1/" + path, nil
}

func ApplyAPIKey(req *http.Request, apiKey string) error {
	if req == nil {
		return errors.New("Azure OpenAI request is nil")
	}
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("AZURE_OPENAI_API_KEY is not set")
	}
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	return nil
}
