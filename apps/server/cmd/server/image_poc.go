package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const openAIImageGenerationURL = "https://api.openai.com/v1/images/generations"

type imagePocRequest struct { Prompt string `json:"prompt"` }
type imagePocOpenAIResponse struct {
	Data []struct { B64JSON string `json:"b64_json"`; URL string `json:"url"` } `json:"data"`
	Error *struct { Message string `json:"message"` } `json:"error,omitempty"`
}

func newImagePocHandler(apiKey string) http.HandlerFunc {
	client := &http.Client{Timeout: 90 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); _ = json.NewEncoder(w).Encode(map[string]string{"error":"POST only"}); return }
		if apiKey == "" { w.WriteHeader(http.StatusServiceUnavailable); _ = json.NewEncoder(w).Encode(map[string]string{"error":"OPENAI_API_KEY is not configured on the Render backend"}); return }
		var in imagePocRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil { w.WriteHeader(http.StatusBadRequest); _ = json.NewEncoder(w).Encode(map[string]string{"error":"invalid JSON request"}); return }
		raw := strings.TrimSpace(in.Prompt)
		if raw == "" || len([]rune(raw)) > 1000 { w.WriteHeader(http.StatusBadRequest); _ = json.NewEncoder(w).Encode(map[string]string{"error":"prompt must be 1..1000 characters"}); return }

		prompt := "Create a single non-explicit image suitable as source material for a fictional Japanese personal-computer BBS file library circa 1996. Compose specifically for a wide 16:10 landscape frame corresponding to a 640x400 PC-98 screen. Keep the main subject and important details safely inside that wide frame, with useful horizontal composition; do not compose as a square or portrait image. All depicted people must be adults age 20 or older. No nudity or explicit sexual content. Do not add text unless the subject requires it. Subject requested by the user: " + raw
		payload, _ := json.Marshal(map[string]any{
			"model":"gpt-image-1", "prompt":prompt,
			// GPT Image does not offer native 640x400. 1536x1024 is its landscape
			// generation size and is substantially closer to PC-98's 16:10 frame
			// than the old square source. The browser performs the final 640x400 fit.
			"size":"1536x1024", "quality":"low", "n":1,
		})
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, openAIImageGenerationURL, bytes.NewReader(payload))
		if err != nil { w.WriteHeader(http.StatusInternalServerError); _ = json.NewEncoder(w).Encode(map[string]string{"error":err.Error()}); return }
		req.Header.Set("Authorization", "Bearer "+apiKey); req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil { w.WriteHeader(http.StatusBadGateway); _ = json.NewEncoder(w).Encode(map[string]string{"error":err.Error()}); return }
		defer resp.Body.Close()
		var out imagePocOpenAIResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { w.WriteHeader(http.StatusBadGateway); _ = json.NewEncoder(w).Encode(map[string]string{"error":"invalid response from OpenAI image generation"}); return }
		if resp.StatusCode < 200 || resp.StatusCode >= 300 { message := "OpenAI image generation failed"; if out.Error != nil && out.Error.Message != "" { message=out.Error.Message }; w.WriteHeader(resp.StatusCode); _=json.NewEncoder(w).Encode(map[string]string{"error":message}); return }
		if len(out.Data)==0 { w.WriteHeader(http.StatusBadGateway); _=json.NewEncoder(w).Encode(map[string]string{"error":"OpenAI returned no image"}); return }
		image:=out.Data[0].URL; if out.Data[0].B64JSON!="" { image="data:image/png;base64,"+out.Data[0].B64JSON }
		if image=="" { w.WriteHeader(http.StatusBadGateway); _=json.NewEncoder(w).Encode(map[string]string{"error":"OpenAI returned no image"}); return }
		_ = json.NewEncoder(w).Encode(map[string]string{"image":image,"model":"gpt-image-1","source_size":"1536x1024"})
	}
}
