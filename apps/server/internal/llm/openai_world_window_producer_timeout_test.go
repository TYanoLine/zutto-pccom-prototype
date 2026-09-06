package llm

import (
	"net/http"
	"testing"
	"time"
)

func TestWorldWindowProducerExtendsShortSharedHTTPTimeout(t *testing.T) {
	originalClient := &http.Client{Timeout: 90 * time.Second}
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{Client: originalClient}}

	producer := provider.withWorldWindowHTTPTimeout()
	if producer.Client == originalClient {
		t.Fatal("producer must clone the shared client instead of mutating it")
	}
	if got := producer.Client.Timeout; got != worldWindowProducerHTTPTimeout {
		t.Fatalf("producer timeout = %s, want %s", got, worldWindowProducerHTTPTimeout)
	}
	if got := originalClient.Timeout; got != 90*time.Second {
		t.Fatalf("shared client timeout mutated to %s", got)
	}
}

func TestWorldWindowProducerCreatesLongClientWhenMissing(t *testing.T) {
	provider := StructuredOpenAIProvider{}
	producer := provider.withWorldWindowHTTPTimeout()
	if producer.Client == nil {
		t.Fatal("producer client is nil")
	}
	if got := producer.Client.Timeout; got != worldWindowProducerHTTPTimeout {
		t.Fatalf("producer timeout = %s, want %s", got, worldWindowProducerHTTPTimeout)
	}
}

func TestWorldWindowProducerPreservesLongerClientTimeout(t *testing.T) {
	originalClient := &http.Client{Timeout: 300 * time.Second}
	provider := StructuredOpenAIProvider{OpenAIProvider: OpenAIProvider{Client: originalClient}}
	producer := provider.withWorldWindowHTTPTimeout()
	if got := producer.Client.Timeout; got != 300*time.Second {
		t.Fatalf("producer timeout = %s, want 5m", got)
	}
}
