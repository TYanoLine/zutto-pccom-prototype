package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/azureopenai"
)

// EmbeddingClient generates vector embeddings for given input texts.
type EmbeddingClient interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// AzureOpenAIEmbeddingClient calls the Azure OpenAI Embeddings REST API.
type AzureOpenAIEmbeddingClient struct {
	Endpoint string
	APIKey   string
	Model    string
	Client   *http.Client
}

type openAIEmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model,omitempty"`
}

type openAIEmbeddingItem struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

type openAIEmbeddingResponse struct {
	Data []openAIEmbeddingItem `json:"data"`
}

func (c *AzureOpenAIEmbeddingClient) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// Embed generates vector embeddings for a slice of texts using the configured deployment.
func (c *AzureOpenAIEmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(c.Endpoint) == "" {
		return nil, errors.New("Azure OpenAI endpoint is empty")
	}
	model := strings.TrimSpace(c.Model)
	if model == "" {
		model = "text-embedding-3-small"
	}

	reqBody, err := json.Marshal(openAIEmbeddingRequest{
		Input: texts,
		Model: model,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	url, err := azureopenai.URL(c.Endpoint, "embeddings")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	if err := azureopenai.ApplyAPIKey(req, c.APIKey); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBuf bytes.Buffer
		_, _ = errBuf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("azure openai embeddings status %d: %s", resp.StatusCode, errBuf.String())
	}

	var res openAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	out := make([][]float32, len(texts))
	for _, item := range res.Data {
		if item.Index >= 0 && item.Index < len(out) {
			out[item.Index] = item.Embedding
		}
	}
	return out, nil
}

// CosineSimilarity calculates the cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0.0
	}
	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		va := float64(a[i])
		vb := float64(b[i])
		dot += va * vb
		normA += va * va
		normB += vb * vb
	}
	if normA == 0 || normB == 0 {
		return 0.0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// MockEmbeddingClient is a test double returning deterministic mock embeddings.
type MockEmbeddingClient struct {
	Vectors map[string][]float32
}

func (m *MockEmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if v, ok := m.Vectors[t]; ok {
			out[i] = v
		} else {
			// Pseudo-embedding based on string length and ascii
			vec := make([]float32, 4)
			vec[0] = float32(len(t))
			if len(t) > 0 {
				vec[1] = float32(t[0])
			}
			out[i] = vec
		}
	}
	return out, nil
}
