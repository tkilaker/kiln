package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	openAITTSURL       = "https://api.openai.com/v1/audio/speech"
	openAIMaxChunkSize = 4000 // OpenAI TTS max is 4096 chars, leave some margin
)

// OpenAIProvider implements the Provider interface for OpenAI's TTS API
type OpenAIProvider struct {
	apiKey string
	model  string
	voice  string
	client *http.Client
}

// NewOpenAIProvider creates a new OpenAI TTS provider
func NewOpenAIProvider(apiKey, model, voice string) *OpenAIProvider {
	if model == "" {
		model = "tts-1"
	}
	if voice == "" {
		voice = "alloy"
	}
	return &OpenAIProvider{
		apiKey: apiKey,
		model:  model,
		voice:  voice,
		client: &http.Client{},
	}
}

// openAITTSRequest is the request body for OpenAI's TTS API
type openAITTSRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Voice string `json:"voice"`
}

func (p *OpenAIProvider) GenerateChunkAudio(ctx context.Context, text, voiceID string) ([]byte, error) {
	if voiceID == "" {
		voiceID = p.voice
	}

	reqBody := openAITTSRequest{
		Model: p.model,
		Input: text,
		Voice: voiceID,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAITTSURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (p *OpenAIProvider) AvailableVoices() []Voice {
	return []Voice{
		{ID: "alloy", Name: "Alloy"},
		{ID: "echo", Name: "Echo"},
		{ID: "fable", Name: "Fable"},
		{ID: "onyx", Name: "Onyx"},
		{ID: "nova", Name: "Nova"},
		{ID: "shimmer", Name: "Shimmer"},
	}
}

func (p *OpenAIProvider) DefaultVoice() string {
	return p.voice
}

func (p *OpenAIProvider) MaxChunkSize() int {
	return openAIMaxChunkSize
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}
