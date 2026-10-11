package llm

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	// Identical vectors -> 1.0
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{1.0, 0.0, 0.0}
	if sim := CosineSimilarity(a, b); math.Abs(sim-1.0) > 1e-5 {
		t.Errorf("expected 1.0, got %f", sim)
	}

	// Orthogonal vectors -> 0.0
	c := []float32{0.0, 1.0, 0.0}
	if sim := CosineSimilarity(a, c); math.Abs(sim-0.0) > 1e-5 {
		t.Errorf("expected 0.0, got %f", sim)
	}

	// Opposite vectors -> -1.0
	d := []float32{-1.0, 0.0, 0.0}
	if sim := CosineSimilarity(a, d); math.Abs(sim-(-1.0)) > 1e-5 {
		t.Errorf("expected -1.0, got %f", sim)
	}

	// Different lengths -> 0.0
	e := []float32{1.0, 0.0}
	if sim := CosineSimilarity(a, e); sim != 0.0 {
		t.Errorf("expected 0.0 for mismatched lengths, got %f", sim)
	}
}

func TestAzureOpenAIEmbeddingClient_Embed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "test-key" {
			t.Errorf("missing api-key header")
		}
		var req openAIEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "text-embedding-3-small" {
			t.Errorf("expected text-embedding-3-small, got %s", req.Model)
		}

		res := openAIEmbeddingResponse{
			Data: []openAIEmbeddingItem{
				{Index: 0, Embedding: []float32{0.1, 0.2, 0.3}},
				{Index: 1, Embedding: []float32{0.4, 0.5, 0.6}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer ts.Close()

	client := &AzureOpenAIEmbeddingClient{
		Endpoint: ts.URL,
		APIKey:   "test-key",
		Model:    "text-embedding-3-small",
		Client:   ts.Client(),
	}

	vectors, err := client.Embed(context.Background(), []string{"バーチャファイター", "PC-9821"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vectors) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vectors))
	}
	if len(vectors[0]) != 3 || vectors[0][0] != 0.1 {
		t.Errorf("unexpected vector 0: %v", vectors[0])
	}
	if len(vectors[1]) != 3 || vectors[1][0] != 0.4 {
		t.Errorf("unexpected vector 1: %v", vectors[1])
	}
}
