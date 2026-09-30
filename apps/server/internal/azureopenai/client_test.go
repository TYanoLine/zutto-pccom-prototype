package azureopenai

import (
	"net/http"
	"testing"
)

func TestURLAcceptsResourceOrV1Endpoint(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		want     string
	}{
		{"https://example.openai.azure.com", "https://example.openai.azure.com/openai/v1/responses"},
		{"https://example.openai.azure.com/", "https://example.openai.azure.com/openai/v1/responses"},
		{"https://example.openai.azure.com/openai/v1/", "https://example.openai.azure.com/openai/v1/responses"},
	} {
		got, err := URL(tc.endpoint, "responses")
		if err != nil { t.Fatal(err) }
		if got != tc.want { t.Fatalf("URL(%q)=%q want %q", tc.endpoint, got, tc.want) }
	}
}

func TestApplyAPIKeyUsesAzureHeader(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://example.openai.azure.com/openai/v1/responses", nil)
	if err := ApplyAPIKey(req, "secret"); err != nil { t.Fatal(err) }
	if got := req.Header.Get("api-key"); got != "secret" { t.Fatalf("api-key=%q", got) }
	if got := req.Header.Get("Authorization"); got != "" { t.Fatalf("unexpected Authorization header %q", got) }
}
